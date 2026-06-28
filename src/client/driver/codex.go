package driver

import (
	"strings"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

const (
	CodexDriverName = "codex"

	// CodexAppServerSockPrefix and CodexAppServerSockSuffix are the filename
	// prefix and suffix for per-session codex app-server unix sockets.
	// A session's socket is named codex-<sessionID>.sock.
	CodexAppServerSockPrefix = "codex-"
	CodexAppServerSockSuffix = ".sock"

	codexKeyThreadID          = "thread_id"
	codexKeyRequestedThreadID = "requested_thread_id"
	codexKeyObservedThreadID  = "observed_thread_id"
	codexKeyResumePhase       = "resume_phase"
)

type CodexState struct {
	CommonState

	ThreadID           string
	RequestedThreadID  string
	ObservedThreadID   string
	ResumePhase        string
	FailureReason      string
	CurrentTool        string
	PendingApproval    bool
	TranscriptInFlight bool
	WatchedFile        string
	StatusLine         string
	LastWindowTitle    string
	PlanSummary        string
	DiffSummary        string
	DiffPaths          []string
	RecentTurns        []SummaryTurn
	PendingTools       map[string]codexPendingTool
}

type CodexDriver struct {
	eventLogDir string
}

type codexPendingTool struct {
	Name      string
	Input     map[string]any
	StartedAt time.Time
}

// codexToolEvent carries the minimum fields needed for tool log emission.
// Used internally by handleSubsystemToolCompleted → emitToolLog.
type codexToolEvent struct {
	Kind        string // "PreToolUse" | "PostToolUse" | "PostToolUseFailure"
	ToolName    string
	ToolInput   map[string]any
	ToolUseID   string
	Error       string
	IsInterrupt bool
}

func NewCodexDriver(eventLogDir string) CodexDriver {
	return CodexDriver{eventLogDir: eventLogDir}
}

func (CodexDriver) Name() string                            { return CodexDriverName }
func (CodexDriver) DisplayName() string                     { return CodexDriverName }
func (CodexDriver) Status(s state.DriverState) state.Status { return s.(CodexState).Status }

func (CodexDriver) StartDir(s state.DriverState) string {
	cs, ok := s.(CodexState)
	if !ok {
		return ""
	}
	return cs.StartDir
}

func (CodexDriver) WithStartDir(s state.DriverState, dir string) state.DriverState {
	cs, ok := s.(CodexState)
	if !ok {
		return s
	}
	cs.StartDir = dir
	return cs
}

func (d CodexDriver) View(s state.DriverState) state.View {
	cs, ok := s.(CodexState)
	if !ok {
		cs = CodexState{}
	}
	return d.view(cs)
}

func (d CodexDriver) NewState(now time.Time) state.DriverState {
	return CodexState{
		CommonState: CommonState{
			Status:          state.StatusIdle,
			StatusChangedAt: now,
		},
	}
}

func (d CodexDriver) PrepareLaunch(s state.DriverState, mode state.LaunchMode, project, baseCommand string, options state.LaunchOptions, sandboxed bool) (state.LaunchPlan, error) {
	cs, ok := s.(CodexState)
	if !ok {
		cs = CodexState{}
	}
	startDir := project
	if cs.StartDir != "" {
		startDir = cs.StartDir
	}
	req, stripped := resolveWorktreeRequest(baseCommand, options, "--worktree")
	fields := strings.Fields(stripped)
	if len(fields) == 0 || fields[0] != CodexDriverName {
		return state.LaunchPlan{Command: strings.TrimSpace(baseCommand), StartDir: startDir, Options: options, Stdin: options.InitialInput}, nil
	}
	base := strings.TrimSpace(stripped)
	stream := state.StreamLaunchOptions{}
	if sandboxed {
		stream.SandboxPolicy = state.StreamSandboxPolicyExternal
		stream.ApprovalPolicy = state.StreamApprovalPolicyAutoApprove
	}
	if mode != state.LaunchModeColdStart || cs.ThreadID == "" || !isAlphanumHyphen(cs.ThreadID) || hasResumeToken(base) {
		return state.LaunchPlan{
			Command:   base,
			StartDir:  startDir,
			Options:   state.LaunchOptions{Worktree: state.WorktreeOption{Enabled: req.Enabled}},
			Subsystem: state.LaunchSubsystemStream,
			Stream:    stream,
			Stdin:     options.InitialInput,
		}, nil
	}
	stream.ResumeThreadID = cs.ThreadID
	return state.LaunchPlan{
		Command:   base,
		StartDir:  startDir,
		Options:   state.LaunchOptions{Worktree: state.WorktreeOption{Enabled: req.Enabled}},
		Subsystem: state.LaunchSubsystemStream,
		Stream:    stream,
		Stdin:     options.InitialInput,
	}, nil
}

func hasResumeToken(command string) bool {
	for _, p := range strings.Fields(command) {
		if p == "resume" {
			return true
		}
	}
	return false
}

func (d CodexDriver) Step(prev state.DriverState, ctx state.FrameContext, ev state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	cs, ok := prev.(CodexState)
	if !ok {
		cs = d.NewState(time.Time{}).(CodexState)
	}
	if !ctx.IsRoot {
		switch ev.(type) {
		case state.DEvSubsystem:
		default:
			return cs, nil, d.view(cs)
		}
	}

	switch e := ev.(type) {
	case state.DEvSubsystem:
		next, effs := d.handleSubsystem(cs, ctx, e)
		return next, effs, d.view(next)
	case state.DEvTick:
		effs := cs.HandleTick(e, false)
		return cs, effs, d.view(cs)
	case state.DEvFrameOsc:
		next := d.handleWindowTitle(cs, e.Title, e.Now)
		return next, nil, d.view(next)
	case state.DEvFileChanged:
		next, effs := d.handleTranscriptChanged(cs, e)
		return next, effs, d.view(next)
	case state.DEvJobResult:
		next := d.handleJobResult(cs, e)
		return next, nil, d.view(next)
	case state.DEvStatusLineClick:
		return cs, nil, d.view(cs)

	case state.DEvWorktreeResolved:
		cs.ApplyWorktreeResolved(e)
		return cs, nil, d.view(cs)
	case state.DEvCommandExited:
		cs.ApplyCommandExited(e)
		return cs, nil, d.view(cs)
	}
	return cs, nil, d.view(cs)
}

// RecoverableOnColdStart reports whether a stopped codex frame can be restored
// on cold start. The conversation lives in the codex thread (a host-mounted
// session store that survives container recreation), not in the dead frame, so a
// frame with a resumable thread is worth keeping and relaunching rather than
// dropping. This is the keep/drop decision only; the actual resume vs. fresh
// launch is decided by PrepareLaunch (which additionally declines to auto-resume
// when the command itself carries a `resume` token).
func (CodexDriver) RecoverableOnColdStart(s state.DriverState) bool {
	cs, ok := s.(CodexState)
	if !ok {
		return false
	}
	return cs.ThreadID != "" && isAlphanumHyphen(cs.ThreadID)
}

func (d CodexDriver) WarmStartRecover(s state.DriverState, now time.Time) (state.DriverState, []state.Effect) {
	cs, ok := s.(CodexState)
	if !ok {
		cs = d.NewState(now).(CodexState)
	}
	effs := watchCodexTranscript(&cs)
	effs = append(effs, d.startCodexTranscriptParse(&cs)...)
	return cs, effs
}
