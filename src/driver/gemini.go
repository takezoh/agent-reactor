package driver

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/state"
)

const (
	GeminiDriverName = "gemini"

	geminiKeyGeminiSessionID = "gemini_session_id"
)

type GeminiState struct {
	CommonState

	GeminiSessionID    string
	CurrentTool        string
	TranscriptInFlight bool
	WatchedFile        string
	StatusLine         string
	RecentTurns        []SummaryTurn
	PendingTools       map[string]geminiPendingTool
	LastWindowTitle    string
}

type geminiPendingTool struct {
	Name      string
	Input     map[string]any
	StartedAt time.Time
	SawPrompt bool
}

type GeminiDriver struct {
	eventLogDir string
}

func NewGeminiDriver(eventLogDir string) GeminiDriver {
	return GeminiDriver{eventLogDir: eventLogDir}
}

func (GeminiDriver) Name() string                            { return GeminiDriverName }
func (GeminiDriver) DisplayName() string                     { return GeminiDriverName }
func (GeminiDriver) Status(s state.DriverState) state.Status { return s.(GeminiState).Status }

func (GeminiDriver) StartDir(s state.DriverState) string {
	gs, ok := s.(GeminiState)
	if !ok {
		return ""
	}
	return gs.StartDir
}

func (GeminiDriver) WithStartDir(s state.DriverState, dir string) state.DriverState {
	gs, ok := s.(GeminiState)
	if !ok {
		return s
	}
	gs.StartDir = dir
	return gs
}

func (d GeminiDriver) View(s state.DriverState) state.View {
	gs, ok := s.(GeminiState)
	if !ok {
		gs = GeminiState{}
	}
	return d.view(gs)
}

func (d GeminiDriver) NewState(now time.Time) state.DriverState {
	return GeminiState{
		CommonState: CommonState{
			Status:          state.StatusIdle,
			StatusChangedAt: now,
		},
	}
}

func (d GeminiDriver) PrepareLaunch(s state.DriverState, mode state.LaunchMode, project, baseCommand string, options state.LaunchOptions, sandboxed bool) (state.LaunchPlan, error) {
	gs, ok := s.(GeminiState)
	if !ok {
		gs = GeminiState{}
	}
	startDir := project
	if gs.StartDir != "" {
		startDir = gs.StartDir
	}
	req, command := resolveWorktreeRequest(baseCommand, options, "--worktree", "--workspace")
	command = strings.TrimSpace(command)
	if sandboxed && !hasFlagToken(command, "--yolo") {
		command = appendFlag(command, "--yolo", true)
	}
	if mode != state.LaunchModeColdStart || gs.GeminiSessionID == "" || !isAlphanumHyphen(gs.GeminiSessionID) {
		return state.LaunchPlan{
			Command:  command,
			StartDir: startDir,
			Stdin:    options.InitialInput,
			Options:  state.LaunchOptions{Worktree: state.WorktreeOption{Enabled: req.Enabled}},
		}, nil
	}
	if strings.Contains(command, "--resume") || strings.Contains(command, " -r") {
		return state.LaunchPlan{Command: command, StartDir: startDir, Stdin: options.InitialInput}, nil
	}
	return state.LaunchPlan{
		Command:  command + " --resume " + gs.GeminiSessionID,
		StartDir: startDir,
		Stdin:    options.InitialInput,
	}, nil
}

func (d GeminiDriver) Step(prev state.DriverState, ctx state.FrameContext, ev state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	gs, ok := prev.(GeminiState)
	if !ok {
		gs = d.NewState(time.Time{}).(GeminiState)
	}
	if !ctx.IsRoot {
		if _, ok := ev.(state.DEvHook); !ok {
			return gs, nil, d.view(gs)
		}
	}

	switch e := ev.(type) {
	case state.DEvHook:
		next, effs := d.handleHook(gs, ctx, e)
		return next, effs, d.view(next)
	case state.DEvTick:
		effs := gs.HandleTick(e, false)
		return gs, effs, d.view(gs)
	case state.DEvPaneOsc:
		next := d.handleWindowTitle(gs, e.Title, e.Now)
		return next, nil, d.view(next)
	case state.DEvFileChanged:
		next, effs := d.handleTranscriptChanged(gs, e)
		return next, effs, d.view(next)
	case state.DEvJobResult:
		next := d.handleJobResult(gs, e)
		return next, nil, d.view(next)
	case state.DEvStatusLineClick:
		return gs, nil, d.view(gs)

	case state.DEvWorktreeResolved:
		gs.ApplyWorktreeResolved(e)
		return gs, nil, d.view(gs)
	case state.DEvCommandExited:
		gs.ApplyCommandExited(e)
		return gs, nil, d.view(gs)
	}
	return gs, nil, d.view(gs)
}

func (d GeminiDriver) WarmStartRecover(s state.DriverState, now time.Time) (state.DriverState, []state.Effect) {
	gs, ok := s.(GeminiState)
	if !ok {
		gs = d.NewState(now).(GeminiState)
	}
	effs := watchGeminiTranscript(&gs)
	effs = append(effs, d.startGeminiTranscriptParse(&gs)...)
	return gs, effs
}

func parseGeminiHookPayload(payload json.RawMessage) geminiHookPayload {
	return parsePayload[geminiHookPayload](payload)
}
