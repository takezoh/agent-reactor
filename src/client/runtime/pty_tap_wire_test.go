package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

// eventSink buffers state.Events from a tap goroutine for assertions. The slice
// is protected because tap_manager's readTap calls enqueue from its own
// goroutine and the race detector will catch unsynchronised append.
type eventSink struct {
	mu     sync.Mutex
	events []state.Event
}

func (s *eventSink) push(ev state.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *eventSink) snapshot() []state.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]state.Event(nil), s.events...)
}

// waitForEvent polls the sink until pred matches an event or budget elapses.
// Returns the matching event for follow-up assertions.
func waitForEvent(t *testing.T, sink *eventSink, pred func(state.Event) bool, budget time.Duration) state.Event {
	t.Helper()
	deadline := time.Now().Add(budget)
	for {
		for _, ev := range sink.snapshot() {
			if pred(ev) {
				return ev
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("waitForEvent: timeout after %v, events=%+v", budget, sink.snapshot())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPtyTapWiring_OSC0TitleReachesEvFrameOsc(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	// '\033]0;Braille\a' is the OSC 0 (window-title) escape Claude emits while
	// "thinking" — the tap_manager's 1x1 vt.Terminal must parse it out of the
	// raw byte stream the PtyFrameTap forwards. The leading sleep keeps printf
	// from racing the Subscribe call: server-side em.Render() of a screen that
	// has already absorbed the OSC does NOT replay it, so the subscriber must
	// register before printf fires. 500ms is enough headroom for `go test
	// -race` on contended CI runners.
	frameID := spawnFrame(t, backend, `sleep 0.5; printf '\033]0;Braille\a'; sleep 1`)
	tap := NewPtyFrameTap(backend)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := newTapManager(ctx, tap)
	t.Cleanup(mgr.stopAll)

	sink := &eventSink{}
	mgr.start(state.FrameID("frame-osc0"), frameID, sink.push)

	ev := waitForEvent(t, sink, func(ev state.Event) bool {
		osc, ok := ev.(state.EvFrameOsc)
		return ok && osc.Cmd == 0 && osc.Title == "Braille"
	}, 2*time.Second)

	osc := ev.(state.EvFrameOsc)
	if osc.FrameID != state.FrameID("frame-osc0") {
		t.Errorf("FrameID = %q, want %q", osc.FrameID, "frame-osc0")
	}
}

func TestPtyTapWiring_OSC133ReachesEvFramePrompt(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	// OSC 133;C marks the start of a command, OSC 133;D;<code> the end. The
	// Shell driver consumes these phases to flip Status between running and
	// waiting; without PtyFrameTap, no EvFramePrompt would ever arrive. The
	// leading sleep matches the OSC 0 test rationale: Subscribe must register
	// before printf fires, or the server-side emulator absorbs the OSC into
	// its state and snapshot Render() does NOT replay it.
	frameID := spawnFrame(t, backend, `sleep 0.5; printf '\033]133;C\a'; printf '\033]133;D;0\a'; sleep 1`)
	tap := NewPtyFrameTap(backend)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := newTapManager(ctx, tap)
	t.Cleanup(mgr.stopAll)

	sink := &eventSink{}
	mgr.start(state.FrameID("frame-osc133"), frameID, sink.push)

	// Wait for the Command phase first…
	waitForEvent(t, sink, func(ev state.Event) bool {
		p, ok := ev.(state.EvFramePrompt)
		return ok && p.Phase == state.PromptPhaseCommand
	}, 2*time.Second)

	// …then the Complete phase with the captured exit code.
	completeEv := waitForEvent(t, sink, func(ev state.Event) bool {
		p, ok := ev.(state.EvFramePrompt)
		return ok && p.Phase == state.PromptPhaseComplete
	}, 2*time.Second)

	complete := completeEv.(state.EvFramePrompt)
	if complete.ExitCode == nil || *complete.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", complete.ExitCode)
	}
}
