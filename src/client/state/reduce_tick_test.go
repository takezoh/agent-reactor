package state

import (
	"testing"
	"time"
)

// tickTrackerDriver emits a unique EffStartJob on every DEvTick so tests can
// count how many times Step was called on this driver's frames.
type tickTrackerState struct{ DriverStateBase }

type tickTrackerDriver struct{}

func (tickTrackerDriver) Name() string                       { return "ticktracker" }
func (tickTrackerDriver) DisplayName() string                { return "ticktracker" }
func (tickTrackerDriver) Status(s DriverState) Status        { return StatusRunning }
func (tickTrackerDriver) NewState(now time.Time) DriverState { return tickTrackerState{} }
func (tickTrackerDriver) PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string, options LaunchOptions, _ bool) (LaunchPlan, error) {
	return LaunchPlan{Command: baseCommand, StartDir: project}, nil
}
func (tickTrackerDriver) Persist(s DriverState) map[string]string { return nil }
func (tickTrackerDriver) Restore(bag map[string]string, now time.Time) DriverState {
	return tickTrackerState{}
}
func (tickTrackerDriver) View(s DriverState) View { return View{} }
func (tickTrackerDriver) Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View) {
	if _, ok := ev.(DEvTick); ok {
		return prev, []Effect{EffBroadcastSessionsChanged{}}, View{}
	}
	return prev, nil, View{}
}

func init() {
	if _, exists := driverRegistry["ticktracker"]; !exists {
		Register(tickTrackerDriver{})
	}
}

// stepActiveSessions delivers DEvTick to all sessions regardless of status.
// Drivers decide internally whether to react (self-skip via no-op return).
// These tests verify that the runtime does NOT gate on status.
func TestTickDeliversToDAllSessions(t *testing.T) {
	now := time.Now()
	for _, status := range []Status{StatusIdle, StatusStopped, StatusRunning, StatusWaiting} {
		s := New()
		s.Sessions["s1"] = Session{
			ID:      "s1",
			Command: "stub",
			Driver:  stubDriverState{status: status},
		}
		next, _ := Reduce(s, EvTick{Now: now})
		// stubDriver.Step is a no-op, so state is unchanged — just confirm no panic.
		if next.Now != now {
			t.Errorf("status=%v: expected Now to be updated", status)
		}
	}
}

func TestTickProcessesRunningSessions(t *testing.T) {
	now := time.Now()
	s := New()
	s.Sessions["run1"] = Session{
		ID:      "run1",
		Command: "stub",
		Driver:  stubDriverState{status: StatusRunning},
	}
	s.ActiveSession = "run1"

	_, effs := Reduce(s, EvTick{Now: now})

	// Should have reconcile + health checks at minimum
	var reconcile int
	for _, e := range effs {
		if _, ok := e.(EffReconcileWindows); ok {
			reconcile++
		}
	}
	if reconcile != 1 {
		t.Errorf("EffReconcileWindows count = %d, want 1", reconcile)
	}
}

// === sibling independence (new model) ===

// TestSiblingIndependence verifies that evicting a child frame leaves
// all other frames (siblings and root) intact.
func TestSiblingIndependence(t *testing.T) {
	s := New()
	id := SessionID("abc")
	rootID := FrameID("frame-root")
	child1ID := FrameID("frame-child1")
	child2ID := FrameID("frame-child2")
	s.Sessions[id] = Session{
		ID:      id,
		Project: "/foo",
		Command: "stub",
		Driver:  stubDriverState{},
		Frames: []SessionFrame{
			{ID: rootID, Project: "/foo", Command: "stub", Driver: stubDriverState{}},
			{ID: child1ID, Project: "/foo", Command: "stub", Driver: stubDriverState{}},
			{ID: child2ID, Project: "/foo", Command: "stub", Driver: stubDriverState{}},
		},
	}
	s.ActiveSession = id

	next, _ := Reduce(s, EvPaneWindowVanished{FrameID: child1ID})

	sess, ok := next.Sessions[id]
	if !ok {
		t.Fatal("session should remain when root frame survives")
	}
	if len(sess.Frames) != 2 {
		t.Fatalf("frames = %d, want 2 (root + child2)", len(sess.Frames))
	}
	found := false
	for _, f := range sess.Frames {
		if f.ID == child2ID {
			found = true
		}
		if f.ID == child1ID {
			t.Errorf("evicted child1 should not appear in frames")
		}
	}
	if !found {
		t.Error("child2 should survive after child1 is evicted")
	}
	if sess.Frames[0].ID != rootID {
		t.Errorf("root frame should survive, got %q", sess.Frames[0].ID)
	}
}

// TestMRUFallbackOnFrameDeath verifies that when the active child frame dies,
// the previously active frame (via MRU) becomes the new active frame.
func TestMRUFallbackOnFrameDeath(t *testing.T) {
	s := New()
	id := SessionID("sess-mru")
	rootID := FrameID("frame-root")
	child1ID := FrameID("frame-child1")
	child2ID := FrameID("frame-child2")
	s.Sessions[id] = Session{
		ID:      id,
		Project: "/foo",
		Command: "stub",
		Driver:  stubDriverState{},
		Frames: []SessionFrame{
			{ID: rootID, Project: "/foo", Command: "stub", Driver: stubDriverState{}},
			{ID: child1ID, Project: "/foo", Command: "stub", Driver: stubDriverState{}},
			{ID: child2ID, Project: "/foo", Command: "stub", Driver: stubDriverState{}},
		},
		ActiveFrameID: child2ID,
		MRUFrameIDs:   []FrameID{child1ID, rootID},
	}
	s.ActiveSession = id

	next, _ := Reduce(s, EvPaneWindowVanished{FrameID: child2ID})

	sess, ok := next.Sessions[id]
	if !ok {
		t.Fatal("session should survive child2 death")
	}
	if sess.ActiveFrameID != child1ID {
		t.Errorf("ActiveFrameID = %q, want child1 via MRU fallback", sess.ActiveFrameID)
	}
	for _, f := range sess.Frames {
		if f.ID == child2ID {
			t.Error("dead child2 should not appear in frames")
		}
	}
}

// TestTickFansOutToRootFrameOnly verifies that reduceTick routes DEvTick
// only to the root frame (Frames[0]) and never to child frames. Uses
// tickTrackerDriver which emits EffBroadcastSessionsChanged on every tick
// so that call counts are observable via the effect list.
func TestTickFansOutToRootFrameOnly(t *testing.T) {
	now := time.Now()
	s := New()
	id := SessionID("s1")

	// Root frame uses stub (no-op on tick).
	// Child frame uses tickTracker — if called, it emits EffBroadcastSessionsChanged.
	// If fan-out reaches the child, we'll see an extra Broadcast from it.
	s.Sessions[id] = Session{
		ID:      id,
		Command: "stub",
		Driver:  stubDriverState{status: StatusRunning},
		Frames: []SessionFrame{
			{ID: "root-f", Project: "/foo", Command: "stub", Driver: stubDriverState{status: StatusRunning}},
			{ID: "child-f", Project: "/foo", Command: "ticktracker", Driver: tickTrackerState{}},
		},
	}

	_, effs := Reduce(s, EvTick{Now: now})

	var broadcastCount int
	for _, e := range effs {
		if _, ok := e.(EffBroadcastSessionsChanged); ok {
			broadcastCount++
		}
	}
	// The child frame's tickTrackerDriver emits one EffBroadcastSessionsChanged
	// per DEvTick call. If fan-out reached the child, broadcastCount >= 1.
	// The root (stubDriver) emits no Broadcast on tick.
	// A Broadcast from persist/changed path can appear but only after state
	// changes — stubDriver returns same state, so none from the root.
	if broadcastCount > 0 {
		t.Errorf("expected 0 EffBroadcastSessionsChanged from tick fan-out (child must not be stepped), got %d", broadcastCount)
	}
}

func TestTickNoBroadcastWhenNoChange(t *testing.T) {
	now := time.Now()
	s := New()
	// Running session but stubDriver.Step returns same state + no effects
	s.Sessions["s1"] = Session{
		ID:      "s1",
		Command: "stub",
		Driver:  stubDriverState{status: StatusRunning},
		Frames: []SessionFrame{{
			ID:      "s1",
			Command: "stub",
			Driver:  stubDriverState{status: StatusRunning},
		}},
	}

	_, effs := Reduce(s, EvTick{Now: now})

	for _, e := range effs {
		if _, ok := e.(EffBroadcastSessionsChanged); ok {
			t.Error("should not broadcast when no driver state changed")
		}
		if _, ok := e.(EffPersistSnapshot); ok {
			t.Error("should not persist when no driver state changed")
		}
	}
}
