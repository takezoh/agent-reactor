package state

import (
	"fmt"
	"testing"
)

// containsReleaseFor returns true when EffReleaseFrameSandbox{frameID} is
// present in effs. Helper for the tests below.
func containsReleaseFor(effs []Effect, frameID FrameID) bool {
	for _, e := range effs {
		if r, ok := e.(EffReleaseFrameSandbox); ok && r.FrameID == frameID {
			return true
		}
	}
	return false
}

// containsKillSessionWindow returns true when EffKillSessionWindow is
// emitted for frameID. Used together with containsReleaseFor to assert
// the two effects' independence — pane kill follows window liveness,
// sandbox release follows frame eviction regardless.
func containsKillSessionWindow(effs []Effect, frameID FrameID) bool {
	for _, e := range effs {
		if k, ok := e.(EffKillSessionWindow); ok && k.FrameID == frameID {
			return true
		}
	}
	return false
}

// TestPaneWindowVanished_emitsReleaseFrameSandbox_butNotKill asserts the
// fix for the “container never goes away” bug. When the pane process
// exits (pty EOF) the reducer routes via evictFrame(killWindow=false) so
// no EffKillSessionWindow is emitted (the backend window is already
// gone), but EffReleaseFrameSandbox MUST still fire so the per-frame
// cleanup runs Manager.ReleaseFrame → 0 なら DestroyInstance. Before the
// fix the two responsibilities were welded into EffKillSessionWindow and
// pane-vanished left the container alive forever.
func TestPaneWindowVanished_emitsReleaseFrameSandbox_butNotKill(t *testing.T) {
	s := New()
	id := SessionID("sess-vanish")
	rootID := FrameID("frame-vanish")
	s.Sessions[id] = Session{
		ID:      id,
		Project: "/p",
		Command: "stub",
		Driver:  stubDriverState{},
		Frames: []SessionFrame{
			{ID: rootID, Project: "/p", Command: "stub", Driver: stubDriverState{}},
		},
	}
	s.ActiveSession = id

	_, effs := Reduce(s, EvPaneWindowVanished{FrameID: rootID})

	if !containsReleaseFor(effs, rootID) {
		t.Errorf("expected EffReleaseFrameSandbox{%q} in effects, got %v", rootID, effectTypes(effs))
	}
	if containsKillSessionWindow(effs, rootID) {
		t.Errorf("EffKillSessionWindow must not fire when window already vanished, effects=%v", effectTypes(effs))
	}
}

// TestFrameCommandExited_intentional_emitsKillAndRelease asserts the
// clean exit path still kills the pane window AND releases the sandbox.
// Both effects are required: clean exit means the backend window is
// alive and needs an explicit kill, but the sandbox refcount must drop
// just the same.
func TestFrameCommandExited_intentional_emitsKillAndRelease(t *testing.T) {
	s := New()
	id := SessionID("sess-clean")
	rootID := FrameID("frame-clean")
	s.Sessions[id] = Session{
		ID:      id,
		Project: "/p",
		Command: "stub",
		Driver:  stubDriverState{},
		Frames: []SessionFrame{
			{ID: rootID, Project: "/p", Command: "stub", Driver: stubDriverState{}},
		},
	}
	s.ActiveSession = id

	_, effs := Reduce(s, EvFrameCommandExited{FrameID: rootID, ExitCode: 0})

	if !containsReleaseFor(effs, rootID) {
		t.Errorf("clean exit must emit EffReleaseFrameSandbox{%q}, got %v", rootID, effectTypes(effs))
	}
	if !containsKillSessionWindow(effs, rootID) {
		t.Errorf("clean exit must emit EffKillSessionWindow{%q}, got %v", rootID, effectTypes(effs))
	}
}

// TestFrameCommandExited_abnormal_releasesSandboxButKeepsFrame asserts
// the new behaviour: a crashing agent leaves the frame Stopped in the
// list (so the user can inspect tail output) but the sandbox is still
// released. Before, the container was held forever — that's why a
// build → `make update-server` cycle never reflected new hook
// registrations: postCreate only runs on a freshly provisioned
// container.
func TestFrameCommandExited_abnormal_releasesSandboxButKeepsFrame(t *testing.T) {
	s := New()
	id := SessionID("sess-crash")
	rootID := FrameID("frame-crash")
	s.Sessions[id] = Session{
		ID:      id,
		Project: "/p",
		Command: "stub",
		Driver:  stubDriverState{},
		Frames: []SessionFrame{
			{ID: rootID, Project: "/p", Command: "stub", Driver: stubDriverState{}},
		},
	}
	s.ActiveSession = id

	next, effs := Reduce(s, EvFrameCommandExited{FrameID: rootID, ExitCode: 7})

	if _, ok := next.Sessions[id]; !ok {
		t.Fatalf("abnormal exit must keep the session in state for user inspection")
	}
	if !containsReleaseFor(effs, rootID) {
		t.Errorf("abnormal exit must emit EffReleaseFrameSandbox{%q}, got %v", rootID, effectTypes(effs))
	}
}

// TestStopSession_releasesEveryFrameSandbox guards the regression that
// motivated frameTeardownEffects: reduceStopSession used to emit only
// EffKillSessionWindow + EffUnregisterPane + EffUnwatchFile per frame
// and silently leaked the sandbox. The fix routes through the shared
// teardown helper so a frame removed by stop-session releases its
// container refcount the same way pane-vanish and clean-exit do.
func TestStopSession_releasesEveryFrameSandbox(t *testing.T) {
	s := New()
	id := SessionID("sess-stop")
	rootID := FrameID("frame-stop-root")
	childID := FrameID("frame-stop-child")
	s.Sessions[id] = Session{
		ID:      id,
		Project: "/p",
		Command: "stub",
		Driver:  stubDriverState{},
		Frames: []SessionFrame{
			{ID: rootID, Project: "/p", Command: "stub", Driver: stubDriverState{}},
			{ID: childID, Project: "/p", Command: "stub", Driver: stubDriverState{}},
		},
	}
	s.ActiveSession = id

	_, effs := reduceStopSession(s, 1, "req-1", StopSessionParams{SessionID: string(id)})

	for _, fid := range []FrameID{rootID, childID} {
		if !containsReleaseFor(effs, fid) {
			t.Errorf("stop-session must emit EffReleaseFrameSandbox{%q}; effects=%v", fid, effectTypes(effs))
		}
		if !containsKillSessionWindow(effs, fid) {
			t.Errorf("stop-session must emit EffKillSessionWindow{%q}; effects=%v", fid, effectTypes(effs))
		}
	}
}

// TestEvictRootFrame_releasesEverySiblingSandbox asserts root death
// drags every sibling frame's sandbox release with it. Each frame may
// share the same container (per-project mode) or a single shared
// container (shared mode); either way the runtime's makeCleanup =
// Manager.ReleaseFrame ensures the container only goes away when its
// refcount hits zero, so emitting per-frame is the correct granularity.
func TestEvictRootFrame_releasesEverySiblingSandbox(t *testing.T) {
	s := New()
	id := SessionID("sess-root-kill")
	rootID := FrameID("frame-root")
	childID := FrameID("frame-child")
	s.Sessions[id] = Session{
		ID:      id,
		Project: "/p",
		Command: "stub",
		Driver:  stubDriverState{},
		Frames: []SessionFrame{
			{ID: rootID, Project: "/p", Command: "stub", Driver: stubDriverState{}},
			{ID: childID, Project: "/p", Command: "stub", Driver: stubDriverState{}},
		},
	}
	s.ActiveSession = id

	_, effs := Reduce(s, EvFrameCommandExited{FrameID: rootID, ExitCode: 0})

	if !containsReleaseFor(effs, rootID) {
		t.Errorf("root release missing from effects %v", effectTypes(effs))
	}
	if !containsReleaseFor(effs, childID) {
		t.Errorf("child release missing from effects %v", effectTypes(effs))
	}
}

// effectTypes returns the type names of effs for diagnostics. %T renders
// `state.EffKillSessionWindow`-style names which is enough for test
// failure messages without enumerating every effect type by hand.
func effectTypes(effs []Effect) []string {
	out := make([]string, 0, len(effs))
	for _, e := range effs {
		out = append(out, fmt.Sprintf("%T", e))
	}
	return out
}
