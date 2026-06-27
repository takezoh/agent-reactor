package runtime

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	rsubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/pathmap"
)

// TestHandleSpawnComplete_storesHandlesNonContainer verifies the event loop
// stores the spawn goroutine's results into the loop-owned maps for a
// non-container frame, and registers no container token.
func TestHandleSpawnComplete_storesHandlesNonContainer(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	sub := &fakeSubsystem{id: "sub-1", kind: state.LaunchSubsystemCLI}

	r.handleSpawnComplete(internalSpawnComplete{
		effect:      state.EffSpawnPaneWindow{SessionID: "s1", FrameID: "f1", Project: "/p"},
		subsystemID: "sub-1",
		sub:         sub,
		paneID:      "%1",
	})

	if r.subsystems["sub-1"] != sub {
		t.Errorf("subsystems[sub-1] not stored")
	}
	if r.frameSubsystems["f1"] != sub {
		t.Errorf("frameSubsystems[f1] not stored")
	}
	if r.frameSubsystemIDs["f1"] != "sub-1" {
		t.Errorf("frameSubsystemIDs[f1] = %q, want sub-1", r.frameSubsystemIDs["f1"])
	}
	if _, ok := r.frameReg.GetMounts("f1"); ok {
		t.Errorf("non-container frame must not register mounts")
	}
}

// TestHandleSpawnComplete_registersContainerFrame verifies that a container
// spawn (token set) atomically registers the token and mounts via the registry
// and starts the endpoint.
func TestHandleSpawnComplete_registersContainerFrame(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{Backend: newFakeBackend()})
	t.Cleanup(r.shutdownContainerEndpoints)

	sub := &fakeSubsystem{id: "sub-1", kind: state.LaunchSubsystemCLI}
	ms := pathmap.Mounts{{Host: "/h/work", Container: "/work"}}

	r.handleSpawnComplete(internalSpawnComplete{
		effect:           state.EffSpawnPaneWindow{SessionID: "s1", FrameID: "f1", Project: "/p"},
		subsystemID:      "sub-1",
		sub:              sub,
		token:            "tok-1",
		mounts:           ms,
		containerSockDir: dir,
		paneID:           "%1",
	})

	id, ok := r.frameReg.Lookup("tok-1")
	if !ok || id != "f1" {
		t.Fatalf("frameReg.Lookup(tok-1) = (%q, %v), want (f1, true)", id, ok)
	}
	got, ok := r.frameReg.GetMounts("f1")
	if !ok || len(got) != 1 {
		t.Fatalf("frameReg.GetMounts(f1) = (%v, %v), want one mount", got, ok)
	}
	if _, ok := r.containerEndpoints["/p"]; !ok {
		t.Errorf("container endpoint for project /p not started")
	}
}

// fakeFactory and fakeSubsystem live in subsystem_dispatch_test.go.

// TestSpawnPaneWindow_emitsInternalSpawnComplete verifies the free spawn
// function performs I/O and reports back via internalSpawnComplete without
// touching any *Runtime state (it holds only a spawnDeps).
func TestSpawnPaneWindow_emitsInternalSpawnComplete(t *testing.T) {
	sub := &fakeSubsystem{id: "sub-x", kind: state.LaunchSubsystemCLI}
	internalCh := make(chan internalEvent, 1)
	eventCh := make(chan state.Event, 1)

	deps := spawnDeps{
		backend:  newFakeBackend(),
		launcher: DirectLauncher{},
		factories: map[state.LaunchSubsystem]rsubsystem.Factory{
			state.LaunchSubsystemCLI: &fakeFactory{sub: sub},
		},
		sessionName:  "roost",
		mainPaneSize: func() paneSize { return paneSize{} },
		sendInternal: func(ev internalEvent) { internalCh <- ev },
		sendEvent:    func(ev state.Event) { eventCh <- ev },
	}

	spawnPaneWindow(deps, state.EffSpawnPaneWindow{
		SessionID: "s1", FrameID: "f1", Project: "/p", Command: "minimal-test",
	})

	select {
	case ev := <-internalCh:
		sc, ok := ev.(internalSpawnComplete)
		if !ok {
			t.Fatalf("expected internalSpawnComplete, got %T", ev)
		}
		if sc.subsystemID != "sub-x" || sc.sub != sub {
			t.Errorf("internalSpawnComplete subsystem = (%q, %v), want (sub-x, sub)", sc.subsystemID, sc.sub)
		}
		if sc.token != "" {
			t.Errorf("non-container spawn must carry empty token, got %q", sc.token)
		}
		if sc.paneID != "%1" {
			t.Errorf("paneID = %q, want %%1", sc.paneID)
		}
	default:
		t.Fatal("no internalSpawnComplete emitted")
	}

	select {
	case ev := <-eventCh:
		t.Fatalf("unexpected event on success: %T", ev)
	default:
	}
}

// TestSpawnPaneWindow_emitsSpawnFailedOnError verifies a backend SpawnWindow
// failure is reported via EvSpawnFailed and no internalSpawnComplete.
func TestSpawnPaneWindow_emitsSpawnFailedOnError(t *testing.T) {
	sub := &fakeSubsystem{id: "sub-x", kind: state.LaunchSubsystemCLI}
	backend := newFakeBackend()
	backend.spawnErr = errors.New("backend boom")

	internalCh := make(chan internalEvent, 1)
	eventCh := make(chan state.Event, 1)

	deps := spawnDeps{
		backend:  backend,
		launcher: DirectLauncher{},
		factories: map[state.LaunchSubsystem]rsubsystem.Factory{
			state.LaunchSubsystemCLI: &fakeFactory{sub: sub},
		},
		sessionName:  "roost",
		mainPaneSize: func() paneSize { return paneSize{} },
		sendInternal: func(ev internalEvent) { internalCh <- ev },
		sendEvent:    func(ev state.Event) { eventCh <- ev },
	}

	spawnPaneWindow(deps, state.EffSpawnPaneWindow{
		SessionID: "s1", FrameID: "f1", Project: "/p", Command: "minimal-test",
	})

	select {
	case ev := <-eventCh:
		if _, ok := ev.(state.EvSpawnFailed); !ok {
			t.Fatalf("expected EvSpawnFailed, got %T", ev)
		}
	default:
		t.Fatal("no EvSpawnFailed emitted")
	}

	select {
	case ev := <-internalCh:
		t.Fatalf("unexpected internal event on failure: %T", ev)
	default:
	}
}

// TestSpawnPaneWindow_cleanupOnSpawnError verifies that when the sandbox was
// acquired (WrapLaunch returned a Cleanup) but backend SpawnWindow then fails, the
// spawn goroutine releases the sandbox — otherwise the container ref leaks
// because no EvPaneSpawned / kill path ever reaches this frame.
func TestSpawnPaneWindow_cleanupOnSpawnError(t *testing.T) {
	var cleaned atomic.Bool
	backend := newFakeBackend()
	backend.spawnErr = errors.New("backend boom")

	deps := spawnDeps{
		backend:  backend,
		launcher: &testLauncher{cleanup: func() error { cleaned.Store(true); return nil }},
		factories: map[state.LaunchSubsystem]rsubsystem.Factory{
			state.LaunchSubsystemCLI: &fakeFactory{sub: &fakeSubsystem{id: "s", kind: state.LaunchSubsystemCLI}},
		},
		sessionName:  "roost",
		mainPaneSize: func() paneSize { return paneSize{} },
		sendInternal: func(internalEvent) {},
		sendEvent:    func(state.Event) {},
	}

	spawnPaneWindow(deps, state.EffSpawnPaneWindow{
		SessionID: "s1", FrameID: "f1", Project: "/p", Command: "minimal-test",
	})

	if !cleaned.Load() {
		t.Error("wrapped.Cleanup was not invoked after SpawnWindow failure (sandbox leak)")
	}
}

// TestSendSpawnComplete_unblocksOnShutdown verifies the reliable spawn-complete
// sender does not leak a goroutine: when the internal channel is full and the
// daemon has shut down (r.done closed), the send returns instead of blocking
// forever.
func TestSendSpawnComplete_unblocksOnShutdown(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	// Fill the internal channel to capacity so the next send would block.
	for {
		select {
		case r.internalCh <- internalStartRestoredTaps{}:
			continue
		default:
		}
		break
	}

	close(r.done)

	done := make(chan struct{})
	go func() {
		r.sendSpawnComplete(internalSpawnComplete{effect: state.EffSpawnPaneWindow{FrameID: "f1"}})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sendSpawnComplete did not return after r.done closed (goroutine leak)")
	}
}
