package termvt

import (
	"strings"
	"testing"
	"time"
)

// waitTimeout bounds how long the event-waiting helpers block before failing.
const waitTimeout = 3 * time.Second

// waitFor reads events until pred matches or waitTimeout elapses.
func waitFor(t *testing.T, ch <-chan Event, pred func(Event) bool) {
	t.Helper()
	deadline := time.After(waitTimeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before match")
			}
			if pred(ev) {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for event")
		}
	}
}

func outputContains(ev Event, sub string) bool {
	return ev.Kind == EventOutput && strings.Contains(string(ev.Data), sub)
}

func controlMatch(ev Event, kind string, code int, dataSub string) bool {
	return ev.Kind == EventControl && ev.Ctl.Kind == kind &&
		ev.Ctl.Code == code && strings.Contains(ev.Ctl.Data, dataSub)
}

func TestSessionEchoesInput(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	s.WriteInput([]byte("ping-123\n"))
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "ping-123") })
}

func TestSessionCapturesOSC9(t *testing.T) {
	// printf emits an OSC 9 desktop-notification sequence; the session must
	// surface it as a Control event rather than raw bytes.
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", `printf '\033]9;hello-notif\a'; sleep 0.3`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return controlMatch(ev, "osc", 9, "hello-notif") })
}

func TestSessionCapturesOSC133Prompt(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", `printf '\033]133;A\a'; sleep 0.3`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return controlMatch(ev, "prompt", 133, "A") })
}

func TestSessionCapturesTitle(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", `printf '\033]0;my-title\a'; sleep 0.3`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return controlMatch(ev, "title", 0, "my-title") })
}

func TestSessionReattachSnapshotFirst(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "1"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	select {
	case ev := <-ch:
		if ev.Kind != EventOutput {
			t.Fatalf("first event is not an output snapshot: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot event")
	}
}

func TestSessionResize(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "1"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if cols, rows := s.Size(); cols != 100 || rows != 30 {
		t.Fatalf("resize not applied: got %dx%d", cols, rows)
	}
}

func TestSessionEmitsExitOnClose(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "5"}})
	if err != nil {
		t.Fatal(err)
	}
	_, ch := s.Subscribe()
	_ = s.Close()
	waitFor(t, ch, func(ev Event) bool { return ev.Kind == EventExit })
}

func TestSessionDefaultSize(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"sleep", "1"}}) // no Cols/Rows → defaults
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if cols, rows := s.Size(); cols != 80 || rows != 24 {
		t.Fatalf("default size = %dx%d, want 80x24", cols, rows)
	}
}

func TestNewSessionEmptyArgv(t *testing.T) {
	if _, err := NewSession(Spec{}); err == nil {
		t.Fatal("expected error for empty argv")
	}
}

// TestNormalizeSizeClamp pins the dimension guard: non-positive sizes floor to
// the defaults and oversized ones are capped at maxDim — so a client cannot
// overflow the uint16 pty winsize (65536 → 0) or drive the VT grid toward OOM.
func TestNormalizeSizeClamp(t *testing.T) {
	cases := []struct{ inC, inR, wantC, wantR int }{
		{0, 0, 80, 24},
		{-5, -1, 80, 24},
		{100, 30, 100, 30},
		{100000, 100000, maxDim, maxDim}, // OOM-sized grid → clamped
		{65536, 1, maxDim, 1},            // would wrap to 0 cols without the cap
	}
	for _, c := range cases {
		if gotC, gotR := normalizeSize(c.inC, c.inR); gotC != c.wantC || gotR != c.wantR {
			t.Errorf("normalizeSize(%d,%d) = %dx%d, want %dx%d",
				c.inC, c.inR, gotC, gotR, c.wantC, c.wantR)
		}
	}
}

// TestSessionDisconnectsSlowSubscriber verifies that a subscriber which does not
// drain its channel is disconnected (channel closed) once its buffer overflows,
// rather than having events silently dropped. A 20MB output stream yields far
// more than the buffer's worth of events; we deliberately do not read during the
// flood, then drain and require the channel to be closed.
func TestSessionDisconnectsSlowSubscriber(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c", "head -c 20000000 /dev/zero"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	time.Sleep(500 * time.Millisecond) // let the flood overflow the buffer

	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // disconnected as expected
			}
		case <-deadline:
			t.Fatal("slow subscriber was not disconnected on overflow")
		}
	}
}
