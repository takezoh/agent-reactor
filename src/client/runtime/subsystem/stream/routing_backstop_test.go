package stream

// Fake fidelity backstop for the routing-isolation invariant.
//
// The real-app-server e2e (routing_e2e_test.go, build tag `e2e`) validates
// that the fake matches production wire behaviour, but it needs model access
// and takes minutes. This file re-uses the same scenario+assertion pattern
// against fake.AppServer so the invariant is exercised on every default `go
// test` run without any external dependencies.

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/runtime/subsystem/stream/fake"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
)

// backstopEmitter is the interface a backstop test provides to
// runIsolationScenario for two operations: minting a fresh thread id (as if a
// spawned CLI issued its own thread/start) and broadcasting a marker
// notification on a given thread. Both operations must go through the same
// wire path (WebSocket-over-UDS to the fake or real app-server) so
// Backend.handleThreadStarted sees the resulting broadcast.
type backstopEmitter interface {
	// mintThread returns a fresh thread id and broadcasts thread/started
	// for it. The Backend's adopt path should pick it up into the frame
	// currently holding the initState slot.
	mintThread(cwd string) string
	// emitMarker broadcasts an agent-message delta carrying marker to
	// threadID, exercising the routing path from server → Backend →
	// recordingRuntime.
	emitMarker(threadID, marker string)
}

// runIsolationScenario drives the ADR-0081 invariant end-to-end against
// either the fake or a real app-server. Two frames sharing a cwd each get
// a distinct thread id via the CLI-owned lifecycle (mintThread), and marker
// events must route back only to the owning frame — no cross-talk.
func runIsolationScenario(t *testing.T, attach func(t *testing.T) (*Backend, *recordingRuntime, backstopEmitter)) {
	t.Helper()
	b, rt, emit := attach(t)

	type frame struct {
		id     state.FrameID
		marker string
	}
	frames := []frame{{"A", "FAKE_MARKER_ALPHA"}, {"B", "FAKE_MARKER_BRAVO"}}
	threadIDs := make([]string, len(frames))

	// Serialize the fresh-start → adopt cycle: BindFrame acquires the
	// initState slot, then a fresh thread/started broadcast fills it. This
	// mirrors what happens with a real CLI, minus the pty process.
	for i, f := range frames {
		if _, err := b.BindFrame(context.Background(), subsystem.BindRequest{
			FrameID: f.id,
			Plan:    state.LaunchPlan{StartDir: "/shared"},
		}); err != nil {
			t.Fatalf("BindFrame(%s): %v", f.id, err)
		}
		// Emit thread/started on a fresh id; the Backend's read loop will
		// call handleThreadStarted → adopt into the pending frame.
		threadIDs[i] = emit.mintThread("/shared")
		if err := waitForBinding(t, b, f.id, 2*time.Second); err != nil {
			t.Fatalf("adopt for %s did not complete: %v", f.id, err)
		}
	}
	if threadIDs[0] == threadIDs[1] {
		t.Fatalf("same-cwd frames must get distinct thread ids, both = %q", threadIDs[0])
	}

	for i, f := range frames {
		emit.emitMarker(threadIDs[i], f.marker)
	}

	// Give the async pipeline time to flush both markers.
	deadline := time.Now().Add(3 * time.Second)
	for len(rt.framesWithMarker(frames[0].marker)) == 0 || len(rt.framesWithMarker(frames[1].marker)) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for markers; A=%v B=%v",
				rt.framesWithMarker(frames[0].marker), rt.framesWithMarker(frames[1].marker))
		}
		time.Sleep(2 * time.Millisecond)
	}
	for _, f := range frames {
		assertMarkerFrames(t, rt, f.marker, f.id)
	}
}

// waitForBinding blocks until b.frames[frameID].threadID is non-empty (adopt
// completed) or timeout elapses. Small helper used by the scenario runner
// after mintThread broadcasts.
func waitForBinding(t *testing.T, b *Backend, frameID state.FrameID, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		binding := b.frames[frameID]
		adopted := binding != nil && binding.threadID != ""
		b.mu.Unlock()
		if adopted {
			return nil
		}
		time.Sleep(2 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

// TestStreamRoutingFakeBackstop is the always-on version of the routing
// isolation e2e — same scenario, same assertions, but against fake.AppServer.
// If this ever fails, the fake and the real server have diverged and the
// production Backend can no longer rely on the fake as a proxy for wire
// behaviour.
func TestStreamRoutingFakeBackstop(t *testing.T) {
	runIsolationScenario(t, func(t *testing.T) (*Backend, *recordingRuntime, backstopEmitter) {
		srv := fake.New(fake.Config{Sock: filepath.Join(t.TempDir(), "backstop.sock")})
		if err := srv.Start(); err != nil {
			t.Fatalf("fake.Start: %v", err)
		}
		t.Cleanup(srv.Stop)

		rt := &recordingRuntime{}
		b := New(rt, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, srv.SockPath(), time.Second)
		tr, err := codexclient.DialUDS(srv.SockPath(), 3*time.Second)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		b.conn = codexclient.NewConn(tr, time.Second)
		// Give the backend its own reap loop as production would.
		b.ctx, b.cancel = context.WithCancel(context.Background())
		t.Cleanup(b.cancel)
		go b.conn.Run(b.ctx, b) //nolint:errcheck
		go b.reapExpiredSlots()
		if err := codexclient.Initialize(b.conn); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		return b, rt, &fakeEmitter{srv: srv}
	})
}

// fakeEmitter adapts fake.AppServer into the scenario-agnostic
// backstopEmitter interface.
type fakeEmitter struct {
	srv *fake.AppServer
}

func (e *fakeEmitter) mintThread(cwd string) string {
	// Trigger a thread/started broadcast for a fresh id. There is no public
	// helper on fake.AppServer for this, so we synthesize an id and use
	// Broadcast directly — that is what the real server does when a CLI
	// completes thread/start.
	id := "fake-thread-mint-" + fakeMintCounter()
	if err := e.srv.Broadcast(codexschema.MethodThreadStarted, map[string]any{
		"thread": map[string]any{"id": id, "sessionId": id, "cwd": cwd},
	}); err != nil {
		panic(err)
	}
	return id
}

func (e *fakeEmitter) emitMarker(threadID, marker string) {
	if err := e.srv.Broadcast(codexschema.MethodItemAgentMessageDelta, map[string]any{
		"threadId": threadID,
		"delta":    marker,
	}); err != nil {
		panic(err)
	}
}

// fakeMintCounter yields monotonic ids for the local scenario runs; ids need
// only be unique within one test, so a package-scoped counter is enough.
// Kept out of fake/ so it stays a test-only helper.
var mintSeq int

func fakeMintCounter() string {
	mintSeq++
	// json import kept for future use (payload assertions).
	_ = json.RawMessage(nil)
	return string(rune('a' - 1 + mintSeq)) // 'a', 'b', 'c' … enough for isolation tests
}
