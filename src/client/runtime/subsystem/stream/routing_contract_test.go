package stream

// Routing-isolation contract for the multiplexed stream backend.
//
// One Backend fronts a single codex app-server connection but multiplexes many
// frames (agents). Every server event for a thread T must be delivered ONLY to
// the frame that owns T. A leak — an agent's turn output surfacing in another
// agent's session — is the "cross-talk" failure this contract pins.
//
//	Routing Isolation Invariant:
//	  for every state.EvSubsystem emitted from thread T,  FrameID == owner(T)
//
// Since the demux fix (docs/adr/0001), ownership is established **synchronously**
// when the thread is created (bindThread issues thread/start and binds the
// returned id) or resumed — there is no cwd/active-frame guessing. So two frames
// sharing a cwd get distinct thread ids and cannot cross-talk by construction.
// These direct-drive cases pin that routing is by exact thread id, and that an
// unbound thread.started is dropped rather than adopted.

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/takezoh/agent-reactor/client/state"
)

// recordingRuntime captures every EvSubsystem the backend enqueues. It is the
// observation point for the isolation invariant: which FrameID each event
// carried. Safe for concurrent Enqueue (the wired harness drives from the read
// loop goroutine); the direct-drive harness is single-goroutine.
type recordingRuntime struct {
	mu     sync.Mutex
	events []state.EvSubsystem
}

func (r *recordingRuntime) Enqueue(e state.Event) {
	se, ok := e.(state.EvSubsystem)
	if !ok {
		return
	}
	r.mu.Lock()
	r.events = append(r.events, se)
	r.mu.Unlock()
}

// framesWithMarker returns the distinct frames whose emitted events carried the
// marker text (markers travel in LastAssistantMessage). Order-stable by first
// appearance.
func (r *recordingRuntime) framesWithMarker(marker string) []state.FrameID {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[state.FrameID]bool{}
	var out []state.FrameID
	for _, e := range r.events {
		if strings.Contains(e.Payload.LastAssistantMessage, marker) && !seen[e.FrameID] {
			seen[e.FrameID] = true
			out = append(out, e.FrameID)
		}
	}
	return out
}

func rawJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// inProc is the direct-drive harness: it manipulates the backend's frame
// registry the way bindThread leaves it (a frame bound to a distinct thread id),
// then feeds server events straight into the notification handlers.
type inProc struct {
	t  *testing.T
	b  *Backend
	rt *recordingRuntime
}

func newInProc(t *testing.T) *inProc {
	t.Helper()
	rt := &recordingRuntime{}
	b := New(rt, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, "/sock", 0)
	return &inProc{t: t, b: b, rt: rt}
}

// bind registers a frame attached to threadID — the deterministic state
// bindThread leaves after a synchronous thread/start (or resume).
func (h *inProc) bind(frame state.FrameID, threadID, startDir string) {
	h.b.mu.Lock()
	h.b.frames[frame] = &frameBinding{
		frameID:     frame,
		startDir:    startDir,
		threadID:    threadID,
		requestedID: threadID,
		observedID:  threadID,
		resumePhase: resumePhaseAttached,
	}
	h.b.threads[threadID] = frame
	h.b.mu.Unlock()
}

// started feeds a `thread/started` notification (nested shape, as the real
// app-server and codexclient.Server.EmitThreadStarted produce it).
func (h *inProc) started(threadID, cwd string) {
	h.b.handleThreadStarted(rawJSON(map[string]any{
		"thread": map[string]any{"id": threadID, "cwd": cwd},
	}))
}

func (h *inProc) message(threadID, delta string) {
	h.b.handleAgentMessageDelta(rawJSON(map[string]any{"threadId": threadID, "delta": delta}))
}

func (h *inProc) completed(threadID, text string) {
	h.b.handleTurnCompleted(rawJSON(map[string]any{"threadId": threadID, "text": text}))
}

func (h *inProc) release(frame state.FrameID) { h.b.ReleaseFrame(frame) }

func (h *inProc) wantMarkerFrames(marker string, want ...state.FrameID) {
	h.t.Helper()
	assertMarkerFrames(h.t, h.rt, marker, want...)
}

// assertMarkerFrames is the shared invariant check used by the direct-drive,
// wired, and e2e harnesses: the marker reached exactly `want` (and no others;
// pass no frames to assert it reached nobody).
func assertMarkerFrames(t *testing.T, rt *recordingRuntime, marker string, want ...state.FrameID) {
	t.Helper()
	got := rt.framesWithMarker(marker)
	if !sameFrameSet(got, want) {
		t.Errorf("routing isolation violated: marker %q delivered to %v, want %v", marker, got, want)
	}
}

// sameFrameSet compares as sets (self-contained: dedups both sides rather than
// relying on framesWithMarker's dedup).
func sameFrameSet(got, want []state.FrameID) bool {
	g := map[state.FrameID]bool{}
	for _, f := range got {
		g[f] = true
	}
	w := map[state.FrameID]bool{}
	for _, f := range want {
		w[f] = true
	}
	if len(g) != len(w) {
		return false
	}
	for f := range g {
		if !w[f] {
			return false
		}
	}
	return true
}

func TestStreamRoutingContract(t *testing.T) {
	t.Run("single_frame", func(t *testing.T) {
		// Markers must not be substrings of one another: framesWithMarker uses
		// Contains over the accumulated assistant text.
		h := newInProc(t)
		h.bind("f1", "t1", "/work")
		h.message("t1", "DELTA_ONE")
		h.completed("t1", "final FINAL_ONE")
		h.wantMarkerFrames("DELTA_ONE", "f1")
		h.wantMarkerFrames("FINAL_ONE", "f1")
	})

	t.Run("two_frames_same_cwd_distinct_threads", func(t *testing.T) {
		// The shared-container case: two frames share a cwd, but each is bound to
		// a distinct thread id at creation, so their output never crosses.
		h := newInProc(t)
		h.bind("A", "tA", "/work")
		h.bind("B", "tB", "/work")
		h.message("tA", "MARK_A")
		h.message("tB", "MARK_B")
		h.wantMarkerFrames("MARK_A", "A")
		h.wantMarkerFrames("MARK_B", "B")
	})

	t.Run("completion_reverse_order", func(t *testing.T) {
		h := newInProc(t)
		h.bind("A", "tA", "/a")
		h.bind("B", "tB", "/b")
		h.completed("tB", "MARK_Bc")
		h.completed("tA", "MARK_Ac")
		h.wantMarkerFrames("MARK_Bc", "B")
		h.wantMarkerFrames("MARK_Ac", "A")
	})

	t.Run("thread_started_confirms_bound", func(t *testing.T) {
		h := newInProc(t)
		h.bind("A", "tA", "/work")
		h.started("tA", "/work") // confirmation of an already-bound thread
		h.message("tA", "MARK_A")
		h.wantMarkerFrames("MARK_A", "A")
	})

	t.Run("thread_started_for_unknown_thread_drops", func(t *testing.T) {
		// No cwd/active-frame adoption: a thread.started for an unbound thread
		// reaches nobody (this is the removed cross-talk fallback).
		h := newInProc(t)
		h.bind("A", "tA", "/work")
		h.started("tX", "/work") // tX is not bound to any frame
		h.message("tX", "MARK_X")
		h.wantMarkerFrames("MARK_X") // nobody
	})

	t.Run("release_drops_stray_events", func(t *testing.T) {
		// After a frame is released, late events for its thread must not be
		// re-routed to any other frame.
		h := newInProc(t)
		h.bind("A", "tA", "/a")
		h.bind("B", "tB", "/b")
		h.message("tA", "MARK_A1")
		h.release("A")
		h.message("tA", "MARK_A2") // tA no longer mapped → must drop
		h.wantMarkerFrames("MARK_A1", "A")
		h.wantMarkerFrames("MARK_A2") // nobody
	})
}
