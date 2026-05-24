package stream

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/client/state"
)

func TestFactoryMakeIDIsSessionKeyed(t *testing.T) {
	f := &Factory{cfg: FactoryConfig{}}
	id := f.makeID("abc123")
	if want := state.SubsystemID("stream:session:abc123"); id != want {
		t.Errorf("id = %q, want %q", id, want)
	}
}

func TestFactoryMakeIDDifferentSessionsDifferentIDs(t *testing.T) {
	f := &Factory{cfg: FactoryConfig{}}
	idA := f.makeID("session-a")
	idB := f.makeID("session-b")
	if idA == idB {
		t.Fatalf("different sessions must not collide: %q", idA)
	}
}

func TestFactory_EnsureSharesBackendWithinSession(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	sessID := state.SessionID("sess-shared")
	sharedID := state.SubsystemID("stream:session:sess-shared")
	sentinel := &Backend{subsystemID: sharedID}
	f.backends[sharedID] = sentinel

	plan := state.LaunchPlan{Command: "codex"}
	subA, idA, errA := f.Ensure(context.Background(), sessID, "/workspace/a", plan)
	if errA != nil {
		t.Fatalf("Ensure A: %v", errA)
	}
	subB, idB, errB := f.Ensure(context.Background(), sessID, "/workspace/b", plan)
	if errB != nil {
		t.Fatalf("Ensure B: %v", errB)
	}
	if idA != sharedID || idB != sharedID {
		t.Errorf("same session: IDs must match; A=%q B=%q want=%q", idA, idB, sharedID)
	}
	if subA != sentinel || subB != sentinel {
		t.Errorf("Ensure returned different Backend instances for the same session")
	}
	if got := len(f.backends); got != 1 {
		t.Errorf("backend count = %d, want 1 (one app-server per session)", got)
	}
}

func TestFactory_EnsureDifferentSessionsDifferentBackends(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	idA := state.SubsystemID("stream:session:sess-a")
	idB := state.SubsystemID("stream:session:sess-b")
	backendA := &Backend{subsystemID: idA}
	backendB := &Backend{subsystemID: idB}
	f.backends[idA] = backendA
	f.backends[idB] = backendB

	plan := state.LaunchPlan{Command: "codex"}
	subA, gotIDA, errA := f.Ensure(context.Background(), "sess-a", "/workspace/a", plan)
	if errA != nil {
		t.Fatalf("Ensure A: %v", errA)
	}
	subB, gotIDB, errB := f.Ensure(context.Background(), "sess-b", "/workspace/b", plan)
	if errB != nil {
		t.Fatalf("Ensure B: %v", errB)
	}
	if gotIDA == gotIDB {
		t.Fatalf("different sessions must get different IDs: %q", gotIDA)
	}
	if subA == subB {
		t.Errorf("different sessions must get different Backend instances")
	}
	if subA != backendA {
		t.Errorf("session-a: got wrong Backend")
	}
	if subB != backendB {
		t.Errorf("session-b: got wrong Backend")
	}
}

func TestFactory_RemoveStopsAndDeletesBackend(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	stopped := false
	b := &Backend{
		subsystemID: "stream:session:sess-rm",
		cancel:      func() {},
		done:        make(chan struct{}),
	}
	close(b.done) // simulate already stopped
	_ = stopped
	f.backends["stream:session:sess-rm"] = b

	f.Remove(context.Background(), "stream:session:sess-rm")

	if _, ok := f.backends["stream:session:sess-rm"]; ok {
		t.Errorf("backend not removed after Remove")
	}
}

func TestBackend_BindThreadRegistersMultipleFrameBindings(t *testing.T) {
	b := New(nil, "stream:session:sess1", "sess1", "/workspace/agent-roost",
		"codex", nil, "", false, false,
		"/tmp/codex.sock", "/opt/roost/run/codex.sock", LoopbackPort,
		func() state.FrameID { return "" }, 0,
	)

	frameA := state.FrameID("frame-a")
	frameB := state.FrameID("frame-b")

	b.frames[frameA] = &frameBinding{frameID: frameA, threadID: "thread-a"}
	b.threads["thread-a"] = frameA
	b.frames[frameB] = &frameBinding{frameID: frameB, threadID: "thread-b"}
	b.threads["thread-b"] = frameB

	if got := len(b.frames); got != 2 {
		t.Fatalf("frame bindings = %d, want 2 (one app-server, two frames)", got)
	}
	if b.frameForThread("thread-a") != frameA {
		t.Errorf("thread-a → frame mapping lost")
	}
	if b.frameForThread("thread-b") != frameB {
		t.Errorf("thread-b → frame mapping lost")
	}

	b.ReleaseFrame(frameA)
	if _, exists := b.frames[frameA]; exists {
		t.Errorf("released frameA still in frames map")
	}
	if _, exists := b.threads["thread-a"]; exists {
		t.Errorf("released frameA's thread-a still in threads map")
	}
	if _, exists := b.frames[frameB]; !exists {
		t.Errorf("frameB was unexpectedly removed when releasing frameA")
	}
	if b.frameForThread("thread-b") != frameB {
		t.Errorf("frameB → thread-b mapping lost after releasing A")
	}
}
