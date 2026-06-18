package stream

// Routing-isolation contract for the multiplexed stream backend.
//
// One Backend fronts a single codex app-server connection but multiplexes many
// frames (agents). Every server event for a thread T must be delivered ONLY to
// the frame that started/resumed T. A leak — an agent's turn output surfacing in
// another agent's session — is the "cross-talk" failure this contract pins.
//
//	Routing Isolation Invariant:
//	  for every state.EvSubsystem emitted from thread T,  FrameID == owner(T)
//
// The cases below drive the backend's event handlers directly (the same
// white-box altitude as event_test.go) so they are fully synchronous and
// deterministic. The GREEN cases run in CI as regression guards. The cross-talk
// pins are RED against the current demux (resolveFrameForStartedThread falls
// back to activeLookup when the start cwd is ambiguous) and are gated behind
// REACTOR_ROUTING_PINS so CI stays green until the fix lands; the fix flips them
// to GREEN and the gate is removed. See
// docs/adr/0001-multiplexed-backends-shared-routing-contract.md.

import (
	"encoding/json"
	"os"
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
// registry the way BindFrame would, then feeds server events straight into the
// notification handlers. activeLookup is controllable so a case can pin which
// frame the ambiguous fallback would (wrongly) pick.
type inProc struct {
	t      *testing.T
	b      *Backend
	rt     *recordingRuntime
	mu     sync.Mutex
	active state.FrameID
}

func newInProc(t *testing.T) *inProc {
	t.Helper()
	h := &inProc{t: t, rt: &recordingRuntime{}}
	h.b = New(h.rt, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, "/sock",
		func() state.FrameID {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.active
		}, 0)
	return h
}

// bindCold registers an unbound frame with a start dir — the state BindFrame
// leaves after issuing turn/start for a new thread (see bindThread cold path).
func (h *inProc) bindCold(frame state.FrameID, startDir string) {
	h.b.mu.Lock()
	h.b.frames[frame] = &frameBinding{frameID: frame, startDir: startDir}
	h.b.mu.Unlock()
}

// bindResume registers a frame already attached to threadID — the state
// BindFrame leaves after a synchronous thread/resume (see bindThread resume
// tail), where the thread→frame mapping is unambiguous.
func (h *inProc) bindResume(frame state.FrameID, startDir, threadID string) {
	h.b.mu.Lock()
	h.b.frames[frame] = &frameBinding{
		frameID:     frame,
		startDir:    startDir,
		threadID:    threadID,
		requestedID: threadID,
		observedID:  threadID,
		resumePhase: resumePhasePending,
	}
	h.b.threads[threadID] = frame
	h.b.mu.Unlock()
}

func (h *inProc) setActive(frame state.FrameID) {
	h.mu.Lock()
	h.active = frame
	h.mu.Unlock()
}

// started feeds a `thread/started` notification (nested shape, as the real
// app-server and codexclient.Server.EmitThreadStarted produce it).
func (h *inProc) started(threadID, cwd string) {
	h.b.handleThreadStarted(rawJSON(map[string]any{
		"thread": map[string]any{"id": threadID, "cwd": cwd},
	}))
}

func (h *inProc) message(threadID, delta string) {
	h.b.handleAgentMessageDelta(rawJSON(map[string]any{
		"threadId": threadID,
		"delta":    delta,
	}))
}

func (h *inProc) completed(threadID, text string) {
	h.b.handleTurnCompleted(rawJSON(map[string]any{
		"threadId": threadID,
		"text":     text,
	}))
}

func (h *inProc) release(frame state.FrameID) { h.b.ReleaseFrame(frame) }

// wantMarkerFrames asserts the marker reached exactly the expected frames (and
// no others). Passing no frames asserts the marker reached nobody.
func (h *inProc) wantMarkerFrames(marker string, want ...state.FrameID) {
	h.t.Helper()
	assertMarkerFrames(h.t, h.rt, marker, want...)
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

// requireRoutingPins gates the RED cross-talk pins so CI stays green until the
// demux fix lands. The fix removes these guards and the cases become permanent
// regression coverage.
func requireRoutingPins(t *testing.T) {
	t.Helper()
	if os.Getenv("REACTOR_ROUTING_PINS") == "" {
		t.Skip("routing-isolation pin: RED until the thread→frame demux fix lands; " +
			"set REACTOR_ROUTING_PINS=1 to run. " +
			"See docs/adr/0001-multiplexed-backends-shared-routing-contract.md")
	}
}

func TestStreamRoutingContract(t *testing.T) {
	// --- GREEN regression guards (run in CI) ---

	t.Run("single_frame_happy", func(t *testing.T) {
		// Markers must not be substrings of one another: framesWithMarker uses
		// Contains over the accumulated assistant text.
		h := newInProc(t)
		h.bindCold("f1", "/work")
		h.started("t1", "/work")
		h.message("t1", "DELTA_ONE")
		h.completed("t1", "final FINAL_ONE")
		h.wantMarkerFrames("DELTA_ONE", "f1")
		h.wantMarkerFrames("FINAL_ONE", "f1")
	})

	t.Run("two_frames_distinct_cwd", func(t *testing.T) {
		h := newInProc(t)
		h.bindCold("A", "/a")
		h.bindCold("B", "/b")
		h.started("tA", "/a")
		h.started("tB", "/b")
		h.message("tA", "MARK_A")
		h.message("tB", "MARK_B")
		h.wantMarkerFrames("MARK_A", "A")
		h.wantMarkerFrames("MARK_B", "B")
	})

	t.Run("interleaved_starts_distinct_cwd", func(t *testing.T) {
		h := newInProc(t)
		h.bindCold("A", "/a")
		h.bindCold("B", "/b")
		h.started("tA", "/a")
		h.message("tA", "MARK_A")
		h.started("tB", "/b")
		h.message("tB", "MARK_B")
		h.wantMarkerFrames("MARK_A", "A")
		h.wantMarkerFrames("MARK_B", "B")
	})

	t.Run("completion_reverse_order", func(t *testing.T) {
		h := newInProc(t)
		h.bindCold("A", "/a")
		h.bindCold("B", "/b")
		h.started("tA", "/a")
		h.started("tB", "/b")
		h.completed("tB", "MARK_Bc")
		h.completed("tA", "MARK_Ac")
		h.wantMarkerFrames("MARK_Bc", "B")
		h.wantMarkerFrames("MARK_Ac", "A")
	})

	t.Run("resume_then_coldstart_same_backend", func(t *testing.T) {
		h := newInProc(t)
		h.bindResume("A", "/a", "tA") // A already attached to tA
		h.bindCold("B", "/work")      // B awaiting its thread
		h.started("tB", "/work")
		h.message("tA", "MARK_A")
		h.message("tB", "MARK_B")
		h.wantMarkerFrames("MARK_A", "A")
		h.wantMarkerFrames("MARK_B", "B")
	})

	t.Run("release_drops_stray_events", func(t *testing.T) {
		// After a frame is released, late events for its thread must not be
		// re-routed to whatever frame is currently active.
		h := newInProc(t)
		h.bindCold("A", "/a")
		h.bindCold("B", "/b")
		h.started("tA", "/a")
		h.message("tA", "MARK_A1")
		h.setActive("B")
		h.release("A")
		h.message("tA", "MARK_A2") // tA no longer mapped → must drop, not leak to B
		h.wantMarkerFrames("MARK_A1", "A")
		h.wantMarkerFrames("MARK_A2") // delivered to nobody
	})

	// --- RED cross-talk pins (gated until the demux fix) ---

	t.Run("crosstalk_ambiguous_cwd", func(t *testing.T) {
		requireRoutingPins(t)
		// Two unbound frames share a cwd (the shared-container norm). The
		// app-server starts A's thread, but B happens to be the active frame.
		// resolveFrameForStartedThread can't disambiguate by cwd (2 candidates)
		// and falls back to activeLookup → binds tA to B → A's output leaks to B.
		h := newInProc(t)
		h.bindCold("A", "/work")
		h.bindCold("B", "/work")
		h.setActive("B")
		h.started("tA", "/work") // ground truth: tA is A's thread
		h.message("tA", "MARK_A")
		h.wantMarkerFrames("MARK_A", "A")
	})

	t.Run("crosstalk_zero_candidates_binds_active", func(t *testing.T) {
		requireRoutingPins(t)
		// A spurious/foreign thread.started whose cwd matches no waiting frame
		// must NOT be adopted by the active frame. The current fallback binds it
		// to activeLookup → foreign output surfaces in an unrelated agent.
		h := newInProc(t)
		h.bindCold("A", "/a")
		h.setActive("A")
		h.started("tX", "/somewhere-else") // owned by nobody here
		h.message("tX", "MARK_X")
		h.wantMarkerFrames("MARK_X") // must reach nobody
	})
}
