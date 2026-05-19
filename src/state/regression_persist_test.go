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
func (frameExitStub) Persist(s DriverState) map[string]string                  { return nil }
func (frameExitStub) Restore(bag map[string]string, now time.Time) DriverState { return frameExitState{} }
func (frameExitStub) View(s DriverState) View                                  { return View{} }
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

// Exit code 0 == intentional exit: the frame must be evicted from
// state and the dead tmux window must be torn down via
// EffKillSessionWindow.
func TestReduceFrameCommandExited_ZeroEvicts(t *testing.T) {
	s := New()
	id := SessionID("clean")
	s.Sessions[id] = newExitSession(id)

	next, effs := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: 0})
	if _, ok := next.Sessions[id]; ok {
		t.Error("exit 0 must evict the frame from state")
	}
	if _, ok := findEff[EffKillSessionWindow](effs); !ok {
		t.Error("exit 0 must request EffKillSessionWindow to tear down the dead window")
	}
}

// Non-zero exit code == abnormal exit: the frame stays in state with
// status=Stopped so the user can find the dead pane and the
// surrounding metadata on cold start.
func TestReduceFrameCommandExited_NonZeroMarksStopped(t *testing.T) {
	s := New()
	id := SessionID("crashed")
	s.Sessions[id] = newExitSession(id)

	next, effs := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: 137})

	sess, kept := next.Sessions[id]
	if !kept {
		t.Fatal("non-zero exit must keep the frame for inspection")
	}
	if len(sess.Frames) != 1 {
		t.Fatalf("frame count = %d, want 1", len(sess.Frames))
	}
	st := sess.Frames[0].Driver.(frameExitState).status
	if st != StatusStopped {
		t.Errorf("driver status = %v, want StatusStopped", st)
	}
	if _, ok := findEff[EffKillSessionWindow](effs); ok {
		t.Error("non-zero exit must NOT kill the window — the user needs the tail output")
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

	_, effs := Reduce(s, EvFrameCommandExited{FrameID: FrameID(id), ExitCode: 137})
	if len(effs) != 0 {
		t.Errorf("expected no effects on re-detection of stopped frame; got %d", len(effs))
	}
}
