package runtime

import (
	"bufio"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
	rsubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/pathmap"
)

// TestHandleSpawnComplete_storesHandlesNonContainer verifies the event loop
// stores the spawn goroutine's results into the loop-owned maps for a
// non-container frame, and registers no container token.
func TestHandleSpawnComplete_storesHandlesNonContainer(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/p",
		Frames: []state.SessionFrame{{ID: "f1", Project: "/p", Command: "shell"}},
	}
	sub := &fakeSubsystem{id: "sub-1", kind: state.LaunchSubsystemCLI}

	r.handleSpawnComplete(internalSpawnComplete{
		effect:      state.EffSpawnFrame{SessionID: "s1", FrameID: "f1", Project: "/p"},
		subsystemID: "sub-1",
		sub:         sub,
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

	r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/p",
		Frames: []state.SessionFrame{{ID: "f1", Project: "/p", Command: "shell"}},
	}
	sub := &fakeSubsystem{id: "sub-1", kind: state.LaunchSubsystemCLI}
	ms := pathmap.Mounts{{Host: "/h/work", Container: "/work"}}

	r.handleSpawnComplete(internalSpawnComplete{
		effect:           state.EffSpawnFrame{SessionID: "s1", FrameID: "f1", Project: "/p"},
		subsystemID:      "sub-1",
		sub:              sub,
		token:            "tok-1",
		mounts:           ms,
		containerSockDir: dir,
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

// TestSpawnFrameWindow_emitsInternalSpawnComplete verifies the free spawn
// function performs I/O and reports back via internalSpawnComplete without
// touching any *Runtime state (it holds only a spawnDeps).
func TestSpawnFrameWindow_emitsInternalSpawnComplete(t *testing.T) {
	sub := &fakeSubsystem{id: "sub-x", kind: state.LaunchSubsystemCLI}
	internalCh := make(chan internalEvent, 1)
	eventCh := make(chan state.Event, 1)

	deps := spawnDeps{
		backend:  newFakeBackend(),
		launcher: DirectLauncher{},
		factories: map[state.LaunchSubsystem]rsubsystem.Factory{
			state.LaunchSubsystemCLI: &fakeFactory{sub: sub},
		},
		sendInternal: func(ev internalEvent) { internalCh <- ev },
		sendEvent:    func(ev state.Event) { eventCh <- ev },
	}

	spawnFrameWindow(deps, state.EffSpawnFrame{
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
		// Verify SpawnFrame was called with the frame id directly.
		backend := deps.backend.(*fakeBackend)
		backend.mu.Lock()
		ids := append([]string(nil), backend.spawnFrameIDs...)
		backend.mu.Unlock()
		if len(ids) != 1 || ids[0] != "f1" {
			t.Errorf("SpawnFrame called with frameIDs = %v, want [\"f1\"]", ids)
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

// TestSpawnFrameWindow_emitsSpawnFailedOnError verifies a backend SpawnFrame
// failure is reported via EvSpawnFailed and no internalSpawnComplete.
func TestSpawnFrameWindow_emitsSpawnFailedOnError(t *testing.T) {
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
		sendInternal: func(ev internalEvent) { internalCh <- ev },
		sendEvent:    func(ev state.Event) { eventCh <- ev },
	}

	spawnFrameWindow(deps, state.EffSpawnFrame{
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

// TestSpawnFrameWindow_cleanupOnSpawnError verifies that when the sandbox was
// acquired (WrapLaunch returned a Cleanup) but backend SpawnFrame then fails, the
// spawn goroutine releases the sandbox — otherwise the container ref leaks
// because no EvFrameSpawned / kill path ever reaches this frame.
func TestSpawnFrameWindow_cleanupOnSpawnError(t *testing.T) {
	var cleaned atomic.Bool
	backend := newFakeBackend()
	backend.spawnErr = errors.New("backend boom")

	deps := spawnDeps{
		backend:  backend,
		launcher: &testLauncher{cleanup: func() error { cleaned.Store(true); return nil }},
		factories: map[state.LaunchSubsystem]rsubsystem.Factory{
			state.LaunchSubsystemCLI: &fakeFactory{sub: &fakeSubsystem{id: "s", kind: state.LaunchSubsystemCLI}},
		},
		sendInternal: func(internalEvent) {},
		sendEvent:    func(state.Event) {},
	}

	spawnFrameWindow(deps, state.EffSpawnFrame{
		SessionID: "s1", FrameID: "f1", Project: "/p", Command: "minimal-test",
	})

	if !cleaned.Load() {
		t.Error("wrapped.Cleanup was not invoked after SpawnFrame failure (sandbox leak)")
	}
}

// TestHandleSpawnComplete_discardsWhenFrameKilledMidSpawn verifies the 027
// fix: if the spawn target session/frame is no longer in reducer state when
// the completion arrives (EffKillFrame processed first), the loop
// must NOT write the loop-owned maps and must release the resources the
// goroutine acquired (cleanup closure, ReleaseFrame, frame kill).
func TestHandleSpawnComplete_discardsWhenFrameKilledMidSpawn(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{Backend: backend})
	// state.Sessions is empty — frame "f1" does not exist.

	sub := &fakeSubsystem{id: "sub-1", kind: state.LaunchSubsystemCLI}
	var cleaned atomic.Bool

	r.handleSpawnComplete(internalSpawnComplete{
		effect:      state.EffSpawnFrame{SessionID: "s1", FrameID: "f1", Project: "/p"},
		subsystemID: "sub-1",
		sub:         sub,
		cleanup:     func() error { cleaned.Store(true); return nil },
	})

	// Loop-owned maps must remain untouched.
	if _, ok := r.subsystems["sub-1"]; ok {
		t.Error("subsystems[sub-1] was written for a killed frame (resurrection leak)")
	}
	if _, ok := r.frameSubsystems["f1"]; ok {
		t.Error("frameSubsystems[f1] was written for a killed frame (resurrection leak)")
	}
	if _, ok := r.frameSubsystemIDs["f1"]; ok {
		t.Error("frameSubsystemIDs[f1] was written for a killed frame (resurrection leak)")
	}

	// The orphan frame must be killed synchronously on the loop.
	backend.mu.Lock()
	killCalls := backend.killCalls
	killedFrames := append([]string(nil), backend.killedFrames...)
	backend.mu.Unlock()
	if killCalls != 1 || len(killedFrames) != 1 || killedFrames[0] != "f1" {
		t.Errorf("expected one KillFrame(\"f1\"), got killCalls=%d killedFrames=%v", killCalls, killedFrames)
	}

	// Cleanup + ReleaseFrame run off-loop. Wait briefly for the goroutine.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cleaned.Load() && atomic.LoadInt32(&sub.releaseN) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cleaned.Load() {
		t.Error("cleanup closure was not invoked (container/sandbox leak)")
	}
	if got := atomic.LoadInt32(&sub.releaseN); got != 1 {
		t.Errorf("sub.ReleaseFrame call count = %d, want 1", got)
	}
}

// TestHandleSpawnComplete_discardsContainerFrame asserts the same discard
// path also skips container registration (token+mounts) when the frame is
// gone. This is the path described in 027 that previously caused container
// endpoint + warm-file leaks.
func TestHandleSpawnComplete_discardsContainerFrame(t *testing.T) {
	dir := t.TempDir()
	backend := newFakeBackend()
	r := New(Config{Backend: backend})
	t.Cleanup(r.shutdownContainerEndpoints)

	sub := &fakeSubsystem{id: "sub-1", kind: state.LaunchSubsystemCLI}
	ms := pathmap.Mounts{{Host: "/h/work", Container: "/work"}}
	var cleaned atomic.Bool

	r.handleSpawnComplete(internalSpawnComplete{
		effect:           state.EffSpawnFrame{SessionID: "ghost", FrameID: "ghost", Project: "/p"},
		subsystemID:      "sub-1",
		sub:              sub,
		cleanup:          func() error { cleaned.Store(true); return nil },
		token:            "tok-1",
		mounts:           ms,
		containerSockDir: dir,
	})

	if _, ok := r.frameReg.Lookup("tok-1"); ok {
		t.Error("frameReg.Lookup(tok-1) succeeded for a killed frame (container token leak)")
	}
	if _, ok := r.frameReg.GetMounts("ghost"); ok {
		t.Error("frameReg.GetMounts(ghost) returned mounts for a killed frame (mount leak)")
	}
	if _, ok := r.containerEndpoints["/p"]; ok {
		t.Error("container endpoint was started for a killed frame (endpoint leak)")
	}

	// Frame kill is loop-synchronous.
	backend.mu.Lock()
	killCalls := backend.killCalls
	killedFrames := append([]string(nil), backend.killedFrames...)
	backend.mu.Unlock()
	if killCalls != 1 || len(killedFrames) != 1 || killedFrames[0] != "ghost" {
		t.Errorf("expected one KillFrame(\"ghost\"), got killCalls=%d killedFrames=%v", killCalls, killedFrames)
	}

	// Cleanup + ReleaseFrame run off-loop. The container-frame discard path is
	// the more leak-prone branch (endpoint + token + warm-file all in scope),
	// so it must release the same resources the non-container sibling does.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cleaned.Load() && atomic.LoadInt32(&sub.releaseN) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cleaned.Load() {
		t.Error("cleanup closure was not invoked for discarded container frame")
	}
	if got := atomic.LoadInt32(&sub.releaseN); got != 1 {
		t.Errorf("sub.ReleaseFrame call count = %d, want 1", got)
	}
}

// TestHandleSpawnComplete_discardRepliesToOriginalCaller verifies that when
// the loop discards a spawn (kill-mid-spawn), the original CreateSession /
// AddFrame caller — still parked on its reply channel — receives an error
// response rather than waiting for an HTTP timeout. Without this dispatch,
// the kill replied to its own caller but the spawn caller hung indefinitely.
func TestHandleSpawnComplete_discardRepliesToOriginalCaller(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	r := New(Config{Backend: newFakeBackend()})
	cc := newIPCConn(1, server)
	r.conns[1] = cc
	go r.connWriter(cc)
	t.Cleanup(cc.shut)

	got := make(chan []byte, 1)
	go func() {
		reader := bufio.NewReader(client)
		line, _ := reader.ReadBytes('\n')
		got <- line
	}()

	sub := &fakeSubsystem{id: "sub-1", kind: state.LaunchSubsystemCLI}
	r.handleSpawnComplete(internalSpawnComplete{
		effect: state.EffSpawnFrame{
			SessionID: "ghost", FrameID: "ghost", Project: "/p",
			ReplyConn: state.ConnID(1), ReplyReqID: "spawn-req-1",
		},
		subsystemID: "sub-1",
		sub:         sub,
	})

	select {
	case line := <-got:
		env, err := proto.DecodeEnvelope(line)
		if err != nil {
			t.Fatalf("DecodeEnvelope: %v", err)
		}
		if env.Type != proto.TypeResponse {
			t.Fatalf("type = %q, want %q", env.Type, proto.TypeResponse)
		}
		if env.ReqID != "spawn-req-1" {
			t.Errorf("req_id = %q, want spawn-req-1 (discard must reply to original caller)", env.ReqID)
		}
		if env.Status != proto.StatusError {
			t.Errorf("status = %q, want %q", env.Status, proto.StatusError)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for discard error reply — original caller would hang")
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
		r.sendSpawnComplete(internalSpawnComplete{effect: state.EffSpawnFrame{FrameID: "f1"}})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sendSpawnComplete did not return after r.done closed (goroutine leak)")
	}
}
