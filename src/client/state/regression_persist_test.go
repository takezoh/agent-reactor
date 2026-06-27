package state

import (
	"testing"
	"time"
)

// regression_persist_test.go previously asserted that specific
// reducers emit EffPersistSnapshot{}. That contract has been retired:
// persistence is now a runtime-level invariant driven by the
// state.Sessions delta in Runtime.dispatch, not an opt-in effect each
// reducer must remember to emit. The corresponding behavioural tests
// now live in runtime/regression_persist_test.go.
//
// What remains here is the reducer-level branching for
// EvFrameCommandExited — exit code 0 evicts the frame, non-zero keeps
// it as stopped. This is pure state behaviour, expressible without
// runtime plumbing.

// frameExitStub is a Driver whose status can be set directly via the
// state bag. It also recognises DEvCommandExited and forwards the
// transition to StatusStopped so the reducer's idempotency check can
// be exercised.
type frameExitStub struct{}

type frameExitState struct {
	DriverStateBase
	status Status
}

func (frameExitStub) Name() string                       { return "exitstub" }
func (frameExitStub) DisplayName() string                { return "exitstub" }
func (frameExitStub) Status(s DriverState) Status        { return s.(frameExitState).status }
func (frameExitStub) NewState(now time.Time) DriverState { return frameExitState{} }
func (frameExitStub) PrepareLaunch(s DriverState, _ LaunchMode, project, baseCommand string, _ LaunchOptions, _ bool) (LaunchPlan, error) {
	return LaunchPlan{Command: baseCommand, StartDir: project}, nil
}
func (frameExitStub) Persist(s DriverState) map[string]string { return nil }
func (frameExitStub) Restore(bag map[string]string, now time.Time) DriverState {
	return frameExitState{}
}
func (frameExitStub) View(s DriverState) View { return View{} }
func (frameExitStub) Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View) {
	s := prev.(frameExitState)
	if _, ok := ev.(DEvCommandExited); ok {
		s.status = StatusStopped
	}
	return s, nil, View{}
}

func init() {
	if _, exists := driverRegistry["exitstub"]; !exists {
		Register(frameExitStub{})
	}
}

func newExitSession(id SessionID) Session {
	return Session{
		ID:      id,
		Project: "/p",
		Command: "exitstub",
		Driver:  frameExitState{},
		Frames: []SessionFrame{{
			ID:      FrameID(id),
			Project: "/p",
			Command: "exitstub",
			Driver:  frameExitState{},
		}},
	}
}

// Intentional exit codes (clean exit + standard termination signals)
// must evict the frame from state and tear down the dead backend window
// via EffKillSessionWindow. Many TUI agents return a non-zero code on
// /quit or Ctrl-C, so eviction must not be limited to ExitCode == 0
// or those user-driven terminations linger as Stopped entries that
// the next cold start would restore.
//
//   - 0:   clean exit
//   - 129: SIGHUP  (controlling terminal closed)
//   - 130: SIGINT  (Ctrl-C)
//   - 137: SIGKILL (`kill -9` / OOM)
//   - 143: SIGTERM (graceful kill)
func TestReduceFrameCommandExited_IntentionalExitCodesEvict(t *testing.T) {
	intentional := []int{0, 129, 130, 137, 143}
	for _, code := range intentional {
		t.Run(exitCodeName(code), func(t *testing.T) {
			s := New()
			id := SessionID("clean")
			s.Sessions[id] = newExitSession(id)

			next, effs := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: code})
			if _, ok := next.Sessions[id]; ok {
				t.Errorf("exit %d must evict the frame from state", code)
			}
			if _, ok := findEff[EffKillSessionWindow](effs); !ok {
				t.Errorf("exit %d must request EffKillSessionWindow to tear down the dead window", code)
			}
		})
	}
}

// Crash-style exit codes (a true abnormal exit — not in the
// intentional-termination set) must keep the frame in state with
// status=Stopped so the user can find the dead pane and the
// surrounding metadata for inspection.
//
//   - 1:   generic error
//   - 2:   misuse of shell builtins / argparse error
//   - 134: SIGABRT (assertion failure / panic)
//   - 139: SIGSEGV (segfault)
func TestReduceFrameCommandExited_CrashExitCodesMarkStopped(t *testing.T) {
	crash := []int{1, 2, 134, 139}
	for _, code := range crash {
		t.Run(exitCodeName(code), func(t *testing.T) {
			s := New()
			id := SessionID("crashed")
			s.Sessions[id] = newExitSession(id)

			next, effs := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: code})

			sess, kept := next.Sessions[id]
			if !kept {
				t.Fatalf("crash exit %d must keep the frame for inspection", code)
			}
			if len(sess.Frames) != 1 {
				t.Fatalf("frame count = %d, want 1", len(sess.Frames))
			}
			st := sess.Frames[0].Driver.(frameExitState).status
			if st != StatusStopped {
				t.Errorf("driver status = %v, want StatusStopped", st)
			}
			if _, ok := findEff[EffKillSessionWindow](effs); ok {
				t.Errorf("crash exit %d must NOT kill the window — the user needs the tail output", code)
			}
		})
	}
}

// Reconciliation runs every few ticks, so a dead pane fires the
// event repeatedly. The reducer must be idempotent: once a frame is
// already Stopped, subsequent EvFrameCommandExited events produce no
// effects.
func TestReduceFrameCommandExited_IdempotentAfterStopped(t *testing.T) {
	s := New()
	id := SessionID("crashed")
	sess := newExitSession(id)
	sess.Frames[0].Driver = frameExitState{status: StatusStopped}
	s.Sessions[id] = sess

	_, effs := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: 139})
	if len(effs) != 0 {
		t.Errorf("expected no effects on re-detection of stopped frame; got %d", len(effs))
	}
}

// A driver can reach StatusStopped via its own hook stream — claude
// fires SessionEnd → status=stopped before the pty actually closes.
// When the pane subsequently exits cleanly (code 0/129/130/137/143),
// the reducer must still evict the frame. Otherwise the session
// sticks in the list forever (the arc web server has no fast
// EvPaneDied path and relies entirely on reconcileWindows →
// EvFrameCommandExited for eviction).
func TestReduceFrameCommandExited_IntentionalExitEvictsEvenWhenDriverStopped(t *testing.T) {
	intentional := []int{0, 129, 130, 137, 143}
	for _, code := range intentional {
		t.Run(exitCodeName(code), func(t *testing.T) {
			s := New()
			id := SessionID("hook-stopped")
			sess := newExitSession(id)
			// Simulate a driver that already transitioned to Stopped via
			// its own hook (e.g. claude SessionEnd) before the pty died.
			sess.Frames[0].Driver = frameExitState{status: StatusStopped}
			s.Sessions[id] = sess

			next, effs := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: code})
			if _, ok := next.Sessions[id]; ok {
				t.Errorf("intentional exit %d on hook-stopped frame must still evict from state", code)
			}
			if _, ok := findEff[EffKillSessionWindow](effs); !ok {
				t.Errorf("intentional exit %d on hook-stopped frame must still request EffKillSessionWindow", code)
			}
		})
	}
}

func exitCodeName(code int) string {
	switch code {
	case 0:
		return "exit0_clean"
	case 1:
		return "exit1_generic"
	case 2:
		return "exit2_misuse"
	case 129:
		return "exit129_SIGHUP"
	case 130:
		return "exit130_SIGINT"
	case 134:
		return "exit134_SIGABRT"
	case 137:
		return "exit137_SIGKILL"
	case 139:
		return "exit139_SIGSEGV"
	case 143:
		return "exit143_SIGTERM"
	default:
		return "exit_other"
	}
}
