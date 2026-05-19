package driver

import (
	"strings"
	"time"

	"github.com/takezoh/agent-roost/state"
)

// Generic driver: polling-driven status producer for arbitrary terminal panes
// (vim, tig, build output, unknown commands, fallback). Receives OSC events
// via DEvPaneOsc when the pane emits semantic sequences; otherwise transitions
// Running → Waiting after IdleThreshold elapses.
//
// Shell-specific logic (OSC 133, promptRe heuristic) lives in ShellDriver.
//
// All state lives in GenericState, all I/O is delegated to the worker
// pool via jobs. Step is a pure function.

// GenericState is the per-session state for the generic driver. Plain
// data — no goroutines, no I/O.
type GenericState struct {
	CommonState

	// Driver name (e.g. "bash", "codex", "gemini", or "" for fallback).
	// Stored on the state so the same generic driver impl can serve
	// multiple registered names.
	Name string
}

// GenericDriver is the stateless plugin value. Multiple registered
// names share this single value via the Name field on GenericState.
type GenericDriver struct {
	name        string // e.g. "bash"; empty for fallback
	displayName string
	threshold   time.Duration
}

// NewGenericDriver constructs a generic driver registered under the
// given name. Use "" for the fallback driver. The IdleThreshold is
// captured at construction so each driver instance carries its own
// configured value.
func NewGenericDriver(name, displayName string, threshold time.Duration) GenericDriver {
	return GenericDriver{
		name:        name,
		displayName: displayName,
		threshold:   threshold,
	}
}

func (d GenericDriver) Name() string                          { return d.name }
func (d GenericDriver) DisplayName() string                   { return d.displayName }
func (GenericDriver) Status(s state.DriverState) state.Status { return s.(GenericState).Status }

func (GenericDriver) StartDir(s state.DriverState) string {
	gs, ok := s.(GenericState)
	if !ok {
		return ""
	}
	return gs.StartDir
}

func (GenericDriver) WithStartDir(s state.DriverState, dir string) state.DriverState {
	gs, ok := s.(GenericState)
	if !ok {
		return s
	}
	gs.StartDir = dir
	return gs
}

// View returns the cached View for the given GenericState. Pure
// getter — same as the View Step returns, but callable from the
// runtime without going through Step.
func (d GenericDriver) View(s state.DriverState) state.View {
	gs, ok := s.(GenericState)
	if !ok {
		gs = GenericState{}
	}
	return d.view(gs)
}

func (d GenericDriver) NewState(now time.Time) state.DriverState {
	return GenericState{
		Name: d.name,
		CommonState: CommonState{
			// GenericDriver is 2-state (Running/Waiting) — never Idle/Stopped.
			// Start in Waiting so the first capture establishes the baseline
			// before any stability-threshold logic can run.
			Status:          state.StatusWaiting,
			StatusChangedAt: now,
		},
	}
}

func (d GenericDriver) PrepareLaunch(s state.DriverState, _ state.LaunchMode, project, baseCommand string, options state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	gs, ok := s.(GenericState)
	if !ok {
		gs = GenericState{}
	}
	startDir := project
	req, command := resolveWorktreeRequest(baseCommand, options, "--worktree")
	if gs.StartDir != "" {
		startDir = gs.StartDir
		req.Enabled = true
	}
	return state.LaunchPlan{
		Command:  strings.TrimSpace(command),
		StartDir: startDir,
		Options:  state.LaunchOptions{Worktree: state.WorktreeOption{Enabled: req.Enabled}},
		Stdin:    options.InitialInput,
	}, nil
}

func (d GenericDriver) Persist(s state.DriverState) map[string]string {
	gs, ok := s.(GenericState)
	if !ok {
		return nil
	}
	out := make(map[string]string, 10)
	gs.PersistCommon(out)
	return out
}

func (d GenericDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	gs := GenericState{
		Name: d.name,
		CommonState: CommonState{
			Status:          state.StatusWaiting,
			StatusChangedAt: now,
		},
	}
	if len(bag) == 0 {
		return gs
	}
	gs.RestoreCommon(bag)
	return gs
}

// Step is the pure reducer for the generic driver.
func (d GenericDriver) Step(prev state.DriverState, ctx state.FrameContext, ev state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	gs, ok := prev.(GenericState)
	if !ok {
		gs = d.NewState(time.Time{}).(GenericState)
	}
	if !ctx.IsRoot {
		if _, ok := ev.(state.DEvHook); !ok {
			return gs, nil, d.view(gs)
		}
	}

	switch e := ev.(type) {
	case state.DEvTick:
		// Tick only when visible on the main pane OR actively running
		// (hash still changing). Parked + waiting sessions skip to save CPU;
		// the next tick after the user brings them back to active resumes.
		if !e.Active && gs.Status != state.StatusRunning {
			return gs, nil, d.view(gs)
		}
		effs := gs.HandleTick(e, false)
		return gs, effs, d.view(gs)

	case state.DEvJobResult:
		if summary, inFlight, ok := applySummaryJobResult(gs.Summary, gs.SummaryInFlight, e); ok {
			gs.Summary = summary
			gs.SummaryInFlight = inFlight
			return gs, nil, d.view(gs)
		}

		if r, ok := e.Result.(BranchDetectResult); ok {
			gs.BranchInFlight = false
			if e.Err != nil || r.Branch == "" {
				return gs, nil, d.view(gs) // preserve existing tag; retry on next tick
			}
			gs.BranchTag = r.Branch
			gs.BranchBG = r.Background
			gs.BranchFG = r.Foreground
			gs.BranchAt = e.Now
			gs.BranchIsWorktree = r.IsWorktree
			gs.BranchParentBranch = r.ParentBranch
			return gs, nil, d.view(gs)
		}

		return gs, nil, d.view(gs)

	case state.DEvHook:
		// generic drivers don't consume hooks
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

func (d GenericDriver) PrepareCreate(s state.DriverState, _ state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	gs, ok := s.(GenericState)
	if !ok {
		gs = GenericState{}
	}
	launch, err := CommonPrepareCreate(&gs.CommonState, project, command, options, "--worktree")
	return gs, launch, err
}
