package driver

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/client/state"
)

// ShellDriverName is the registry key for the shell driver.
const ShellDriverName = "shell"

// ShellState is the per-session state for the shell driver. Plain data — no
// goroutines, no I/O.
type ShellState struct {
	CommonState

	// SawPromptEvent is set on the first OSC 133 event. Once true, promptRe
	// fallback is disabled and only OSC 133 events drive status transitions.
	SawPromptEvent bool

	// LastExitCode is the exit code from the most recent OSC 133;D event.
	// nil means no command has completed yet in this session.
	LastExitCode *int
}

// ShellDriver is the stateless plugin value for "shell"-keyed sessions.
type ShellDriver struct {
	name        string
	displayName string
	threshold   time.Duration
}

// NewShellDriver constructs a shell driver registered under the given name.
func NewShellDriver(name, displayName string, threshold time.Duration) ShellDriver {
	return ShellDriver{
		name:        name,
		displayName: displayName,
		threshold:   threshold,
	}
}

func (d ShellDriver) Name() string                          { return d.name }
func (d ShellDriver) DisplayName() string                   { return d.displayName }
func (ShellDriver) Status(s state.DriverState) state.Status { return s.(ShellState).Status }

func (ShellDriver) StartDir(s state.DriverState) string {
	ss, ok := s.(ShellState)
	if !ok {
		return ""
	}
	return ss.StartDir
}

func (ShellDriver) WithStartDir(s state.DriverState, dir string) state.DriverState {
	ss, ok := s.(ShellState)
	if !ok {
		return s
	}
	ss.StartDir = dir
	return ss
}

func (d ShellDriver) View(s state.DriverState) state.View {
	ss, ok := s.(ShellState)
	if !ok {
		ss = ShellState{}
	}
	return d.view(ss)
}

func (d ShellDriver) NewState(now time.Time) state.DriverState {
	return ShellState{
		CommonState: CommonState{
			Status:          state.StatusWaiting,
			StatusChangedAt: now,
		},
	}
}

func (d ShellDriver) PrepareLaunch(s state.DriverState, _ state.LaunchMode, project, baseCommand string, options state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	ss, ok := s.(ShellState)
	if !ok {
		ss = ShellState{}
	}
	startDir := project
	req, command := resolveWorktreeRequest(baseCommand, options, "--worktree")
	if ss.StartDir != "" {
		startDir = ss.StartDir
	}
	return state.LaunchPlan{
		Command:  strings.TrimSpace(command),
		StartDir: startDir,
		Options:  state.LaunchOptions{Worktree: state.WorktreeOption{Enabled: req.Enabled}},
		Stdin:    options.InitialInput,
	}, nil
}

func (d ShellDriver) Persist(s state.DriverState) map[string]string {
	ss, ok := s.(ShellState)
	if !ok {
		return nil
	}
	out := make(map[string]string, 12)
	ss.PersistCommon(out)
	if ss.SawPromptEvent {
		out[keyShellSawPromptEvent] = "1"
	}
	if ss.LastExitCode != nil {
		out[keyShellLastExitCode] = fmt.Sprintf("%d", *ss.LastExitCode)
	}
	return out
}

func (d ShellDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	ss := ShellState{
		CommonState: CommonState{
			Status:          state.StatusWaiting,
			StatusChangedAt: now,
		},
	}
	if len(bag) == 0 {
		return ss
	}
	ss.RestoreCommon(bag)
	ss.SawPromptEvent = bag[keyShellSawPromptEvent] == "1"
	if v := bag[keyShellLastExitCode]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ss.LastExitCode = &n
		}
	}
	return ss
}

const (
	keyShellSawPromptEvent = "shell_saw_prompt_event"
	keyShellLastExitCode   = "shell_last_exit_code"
)

func (d ShellDriver) Step(prev state.DriverState, ctx state.FrameContext, ev state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	ss, ok := prev.(ShellState)
	if !ok {
		ss = d.NewState(time.Time{}).(ShellState)
	}
	if !ctx.IsRoot {
		if _, ok := ev.(state.DEvHook); !ok {
			return ss, nil, d.view(ss)
		}
	}

	switch e := ev.(type) {
	case state.DEvTick:
		if !e.Active && ss.Status != state.StatusRunning {
			return ss, nil, d.view(ss)
		}
		effs := ss.HandleTick(e, false)
		return ss, effs, d.view(ss)

	case state.DEvJobResult:
		ss = d.applyJobResult(ss, e)
		return ss, nil, d.view(ss)

	case state.DEvPanePrompt:
		ss = applyShellPromptEvent(ss, e)
		return ss, nil, d.view(ss)

	case state.DEvWorktreeResolved:
		ss.ApplyWorktreeResolved(e)
		return ss, nil, d.view(ss)
	case state.DEvCommandExited:
		ss.ApplyCommandExited(e)
		return ss, nil, d.view(ss)

	case state.DEvHook:
		return ss, nil, d.view(ss)
	}

	return ss, nil, d.view(ss)
}

func applyShellPromptEvent(ss ShellState, e state.DEvPanePrompt) ShellState {
	ss.SawPromptEvent = true
	prev := ss.Status
	switch e.Phase {
	case state.PromptPhaseInput:
		ss = setShellStatus(ss, state.StatusWaiting, e.Now)
	case state.PromptPhaseCommand:
		ss = setShellStatus(ss, state.StatusRunning, e.Now)
	case state.PromptPhaseComplete:
		ss.LastExitCode = e.ExitCode
		ss = setShellStatus(ss, state.StatusWaiting, e.Now)
	}
	slog.Info("shell: prompt event",
		"phase", e.Phase, "prevStatus", prev, "nextStatus", ss.Status,
		"exitCode", e.ExitCode)
	return ss
}

func setShellStatus(ss ShellState, next state.Status, now time.Time) ShellState {
	if ss.Status == next {
		return ss
	}
	ss.Status = next
	ss.StatusChangedAt = now
	return ss
}

func (d ShellDriver) applyJobResult(ss ShellState, e state.DEvJobResult) ShellState {
	if summary, inFlight, ok := applySummaryJobResult(ss.Summary, ss.SummaryInFlight, e); ok {
		ss.Summary = summary
		ss.SummaryInFlight = inFlight
		return ss
	}
	if r, ok := e.Result.(BranchDetectResult); ok {
		ss.ApplyBranchResult(r, e.Err, e.Now)
	}
	return ss
}

func (d ShellDriver) PrepareCreate(s state.DriverState, _ state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	ss, ok := s.(ShellState)
	if !ok {
		ss = ShellState{}
	}
	launch, err := CommonPrepareCreate(&ss.CommonState, project, command, options, "--worktree")
	return ss, launch, err
}
