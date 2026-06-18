package termvt

import (
	"testing"
	"time"
)

// Fan-out isolation contract for the tmux-free multiplexer. Where the stream
// subsystem's safety-critical property is routing isolation (an event reaches
// only the frame that owns its thread — see docs/adr/0001), termvt's analogue is
// FAN-OUT ISOLATION:
//
//	Every event produced by a Session reaches exactly the live subscribers of
//	THAT session — every one of them, in order — and no subscriber of any other
//	session; a subscriber that cannot keep up is severed without starving or
//	corrupting the others.
//
// Cross-talk here would be one session's bytes surfacing in another session's
// terminal (Manager isolation) or a slow client wedging a healthy one
// (back-pressure isolation). termvt has no in-process fake — its only backend is
// a real pty — so these contract cases drive a real pty directly; there is no
// separate opt-in fidelity tier (cf. docs/adr/0003).

// assertNoOutput fails if sub emits an output frame containing sub-string within
// the window — the negative half of an isolation assertion. An unexpected close
// of the foreign subscriber during the window is itself a failure: treating a
// close as "no further output" would mask a regression in which one session
// severs another's subscriber (the very cross-talk this guards against).
func assertNoOutput(t *testing.T, ch <-chan Event, sub string, window time.Duration) {
	t.Helper()
	deadline := time.After(window)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("foreign subscriber channel closed during isolation window")
			}
			if outputContains(ev, sub) {
				t.Fatalf("isolation violated: saw %q on a foreign subscriber", sub)
			}
		case <-deadline:
			return
		}
	}
}

// TestFanoutDeliversToEverySubscriber pins completeness: every live subscriber
// of a session receives that session's output.
func TestFanoutDeliversToEverySubscriber(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"cat"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, a := s.Subscribe()
	_, b := s.Subscribe()
	s.WriteInput([]byte("MARK-XYZ\n"))

	waitFor(t, a, func(ev Event) bool { return outputContains(ev, "MARK-XYZ") })
	waitFor(t, b, func(ev Event) bool { return outputContains(ev, "MARK-XYZ") })
}

// TestManagerSessionsDoNotCrossTalk is the core isolation case: a subscriber of
// session A sees A's bytes and never B's, and vice versa — the direct analogue
// of routing isolation across the multiplexer's sessions.
func TestManagerSessionsDoNotCrossTalk(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()

	sa, err := m.Create("A", Spec{Argv: []string{"cat"}})
	if err != nil {
		t.Fatal(err)
	}
	sb, err := m.Create("B", Spec{Argv: []string{"cat"}})
	if err != nil {
		t.Fatal(err)
	}

	_, ca := sa.Subscribe()
	_, cb := sb.Subscribe()
	sa.WriteInput([]byte("AAA-MARK\n"))
	sb.WriteInput([]byte("BBB-MARK\n"))

	// Each subscriber receives its own session's marker …
	waitFor(t, ca, func(ev Event) bool { return outputContains(ev, "AAA-MARK") })
	waitFor(t, cb, func(ev Event) bool { return outputContains(ev, "BBB-MARK") })
	// … and never the other session's, even given time to leak.
	assertNoOutput(t, ca, "BBB-MARK", 300*time.Millisecond)
	assertNoOutput(t, cb, "AAA-MARK", 300*time.Millisecond)
}

// TestSlowSubscriberDoesNotStarveFast pins back-pressure isolation: a subscriber
// that never drains is severed, while a concurrent fast subscriber still
// receives the whole stream through to EventExit. A leak here would be the slow
// client's full buffer blocking fanout and wedging the healthy client.
func TestSlowSubscriberDoesNotStarveFast(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", "head -c 5000000 /dev/zero"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, slow := s.Subscribe() // deliberately never drained
	_, fast := s.Subscribe()

	// The fast subscriber drains continuously and must reach EventExit (proof it
	// was neither blocked nor severed by the slow one's back-pressure).
	gotExit := make(chan bool, 1)
	go func() {
		for ev := range fast {
			if ev.Kind == EventExit {
				gotExit <- true
				return
			}
		}
		gotExit <- false // channel closed without EventExit → fast was severed
	}()

	select {
	case ok := <-gotExit:
		if !ok {
			t.Fatal("fast subscriber was severed/starved by the slow one")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("fast subscriber never reached EventExit")
	}

	// The slow subscriber, having overflowed, must have been severed (closed).
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-slow:
			if !ok {
				return // severed as required
			}
		case <-deadline:
			t.Fatal("slow subscriber was not severed on overflow")
		}
	}
}

// TestControlPrecedesOutputInChunk pins ordering: a semantic sequence captured
// from a chunk is delivered as a Control event BEFORE the raw output of that same
// chunk, so the client applies state (e.g. a title) before rendering the bytes.
func TestControlPrecedesOutputInChunk(t *testing.T) {
	// One write emits an OSC 9 notification followed by visible text; the OSC is
	// consumed into a Control event, "TAIL-TEXT" remains in the output.
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", `printf '\033]9;NOTE\aTAIL-TEXT'; sleep 0.3`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	sawControl := false
	deadline := time.After(waitTimeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before TAIL-TEXT")
			}
			if controlMatch(ev, "osc", 9, "NOTE") {
				sawControl = true
			}
			if outputContains(ev, "TAIL-TEXT") {
				if !sawControl {
					t.Fatal("output delivered before its chunk's Control event")
				}
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for TAIL-TEXT")
		}
	}
}
