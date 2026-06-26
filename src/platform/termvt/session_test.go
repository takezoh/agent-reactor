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
	if err := s.WriteInput([]byte("ping-123\n")); err != nil {
		t.Fatalf("WriteInput: %v", err)
	}
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

// TestClampDim pins the per-dimension clamp helper directly, exercising the def
// parameter (which normalizeSize hard-codes to 80/24) and the exact maxDim edge.
func TestClampDim(t *testing.T) {
	cases := []struct {
		name string
		d    int
		def  int
		want int
	}{
		{"zero floors to def", 0, 80, 80},
		{"negative floors to def", -10, 24, 24},
		{"in range passes through", 100, 80, 100},
		{"exactly maxDim passes through", maxDim, 80, maxDim},
		{"above maxDim caps", maxDim + 1, 80, maxDim},
		{"far above maxDim caps", 100000, 24, maxDim},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampDim(c.d, c.def); got != c.want {
				t.Errorf("clampDim(%d, %d) = %d, want %d", c.d, c.def, got, c.want)
			}
		})
	}
}

// TestSessionExitCodeNeverBlocksDuringCSIReportMode reproduces a deadlock:
// the shell emits CSI Report Mode (DECRQM, "\033[?1$p"), the VT emulator's
// handleRequestMode writes the reply to its internal io.Pipe synchronously,
// and nothing drains the read end — em.Write blocks forever, holding s.mu,
// and every ExitCode call (which the runtime dispatch loop fires every tick
// via PaneAlive) hangs in turn. Bug surfaces as the whole daemon's IPC
// freezing under any tty client that ever queries terminal modes.
//
// Skipped pre-fix; the responseLoop drain in step 3 makes it pass. Step 4's
// atomic ExitCode makes ExitCode robust even if mainLoop is busy.
func TestSessionExitCodeNeverBlocksDuringCSIReportMode(t *testing.T) {
	s, err := NewSession(Spec{Argv: []string{"bash", "-c",
		`printf '\033[?1$p'; sleep 0.2; exit 0`}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			_, _ = s.ExitCode()
			time.Sleep(10 * time.Millisecond)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ExitCode blocked — readLoop deadlock on CSI Report Mode")
	}
}

// TestSessionScrollbackSurvivesLateSubscribe pins the late-join contract:
// a fresh subscriber that attaches after the visible grid has scrolled past
// the first lines must receive those lines in the seed's scrollback frame.
// Without server-side scrollback the late subscriber would only see the
// trailing 24 rows (visible grid) and the early "line 1" would be lost —
// exactly the regression this feature exists to prevent.
func TestSessionScrollbackSurvivesLateSubscribe(t *testing.T) {
	s, err := NewSession(Spec{
		Argv: []string{"bash", "-c",
			`for i in $(seq 1 200); do printf "scrollback-row-%d\n" $i; done; sleep 2`},
		Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	// First subscriber drives the timing: drain until we see the last row,
	// which proves the emulator has processed all 200 lines and the early
	// ones have necessarily scrolled past the 24-row visible grid into
	// scrollback.
	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "scrollback-row-200") })

	// Now a late subscriber attaches. Its first frame must be scrollback
	// (carrying the long-gone "scrollback-row-1") and its second frame must
	// be the current visible grid.
	_, late := s.Subscribe()
	first := waitNext(t, late, waitTimeout)
	if first.Kind != EventOutput {
		t.Fatalf("late subscriber first frame kind = %v, want EventOutput", first.Kind)
	}
	if !strings.Contains(string(first.Data), "scrollback-row-1\n") {
		t.Fatalf("late subscriber scrollback frame missing row 1; got first 200 bytes: %q",
			truncate(string(first.Data), 200))
	}
	// Sanity: the screen frame follows; we don't assert its content (the
	// trailing rows depend on bash's exact stdout cadence) but it must arrive.
	if _, ok := <-late; !ok {
		t.Fatal("late subscriber did not receive a screen frame after scrollback")
	}
}

// TestSessionScrollbackLinesCapHonored verifies that Spec.ScrollbackLines
// bounds the buffer: with cap=5 only the trailing 5 scrolled-off rows reach
// a late subscriber's scrollback frame, even when hundreds of rows have
// been printed.
func TestSessionScrollbackLinesCapHonored(t *testing.T) {
	s, err := NewSession(Spec{
		Argv: []string{"bash", "-c",
			`for i in $(seq 1 200); do printf "row-%d\n" $i; done; sleep 2`},
		Cols: 80, Rows: 24,
		ScrollbackLines: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "row-200") })

	_, late := s.Subscribe()
	first := waitNext(t, late, waitTimeout)
	if first.Kind != EventOutput {
		t.Fatalf("late subscriber first frame kind = %v, want EventOutput", first.Kind)
	}
	// Newline accounting for cap=5: Lines.Render() emits 4 separators
	// between 5 rows, subscribeCmd appends 1 trailing newline → exactly 5
	// total. Asserting equality (not ≤) pins the cap precisely: cap=10
	// would produce 10 newlines, cap=∞ would produce 176, both rejected.
	payload := string(first.Data)
	if got := strings.Count(payload, "\n"); got != 5 {
		t.Fatalf("scrollback has %d newlines, want 5 (4 between + 1 trailing for cap=5 rows); payload: %q",
			got, truncate(payload, 400))
	}
	// The cap-drops-old invariant: row-1 is the very first emitted line and
	// must have fallen off the scrollback long ago. If it's present the cap
	// is not being applied.
	if strings.Contains(payload, "row-1\n") {
		t.Fatalf("scrollback cap=5 leaked row-1 (oldest emitted line, cap should have dropped it);"+
			" payload: %q", truncate(payload, 400))
	}
}

// TestSessionScrollbackOmittedInAltScreen pins the alt-screen contract:
// when the program is on the alternate screen (DECSET 1049), nothing has
// spilled to scrollback yet — primary screen scrollback is empty — and the
// seed must elide the scrollback frame entirely. The single seed frame the
// late subscriber receives carries the current alt-screen render.
func TestSessionScrollbackOmittedInAltScreen(t *testing.T) {
	s, err := NewSession(Spec{
		Argv: []string{"bash", "-c",
			`printf '\033[?1049h'; printf 'alt-marker\n'; sleep 2`},
		Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	waitFor(t, ch, func(ev Event) bool { return outputContains(ev, "alt-marker") })

	_, late := s.Subscribe()
	first := waitNext(t, late, waitTimeout)
	if first.Kind != EventOutput {
		t.Fatalf("late subscriber first frame kind = %v, want EventOutput", first.Kind)
	}
	// In alt-screen mode the scrollback buffer is empty, so the very first
	// seed frame must already be the screen render (it contains alt-marker).
	if !strings.Contains(string(first.Data), "alt-marker") {
		t.Fatalf("first frame should be screen render with alt-marker; got: %q",
			truncate(string(first.Data), 400))
	}
}

// truncate returns up to n bytes of s, with an ellipsis when longer. Used by
// scrollback-test failure messages to keep error logs bounded when a multi-KB
// frame is involved.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
