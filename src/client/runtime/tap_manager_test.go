package runtime

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/client/driver/vt"
	"github.com/takezoh/agent-roost/client/state"
)

// fakePaneTap records Start/Stop calls for assertions.
type fakePaneTap struct {
	mu      sync.Mutex
	started []string
	stopped []string
}

func (f *fakePaneTap) Start(_ context.Context, pane string) (<-chan []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, pane)
	ch := make(chan []byte)
	return ch, nil
}

func (f *fakePaneTap) Stop(pane string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, pane)
	return nil
}

func (f *fakePaneTap) startedSorted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.started...)
	sort.Strings(out)
	return out
}

func TestReadTapEmitsOscEvents(t *testing.T) {
	frameID := state.FrameID("f1")
	ch := make(chan []byte, 4)
	ch <- []byte("\x1b]9;hello world\x07")
	close(ch)

	var events []state.Event
	enqueue := func(e state.Event) { events = append(events, e) }

	readTap(context.Background(), frameID, "%1", ch, enqueue)

	var gotOsc bool
	for _, ev := range events {
		if o, ok := ev.(state.EvPaneOsc); ok {
			gotOsc = true
			if o.Cmd != 9 {
				t.Errorf("Cmd = %d, want 9", o.Cmd)
			}
			if o.Title != "hello world" {
				t.Errorf("Title = %q, want %q", o.Title, "hello world")
			}
		}
	}
	if !gotOsc {
		t.Error("expected EvPaneOsc event")
	}
}

func TestReadTapEmitsRepeatedPromptEvents(t *testing.T) {
	frameID := state.FrameID("f1")
	ch := make(chan []byte, 4)
	ch <- []byte("\x1b]133;C\x07")
	ch <- []byte("\x1b]133;D;0\x07")
	ch <- []byte("\x1b]133;C\x07")
	ch <- []byte("\x1b]133;D;42\x07")
	close(ch)

	var events []state.Event
	enqueue := func(e state.Event) { events = append(events, e) }

	readTap(context.Background(), frameID, "%1", ch, enqueue)

	var prompts []state.EvPanePrompt
	for _, ev := range events {
		if p, ok := ev.(state.EvPanePrompt); ok {
			prompts = append(prompts, p)
		}
	}
	if len(prompts) != 4 {
		t.Fatalf("prompt events = %d, want 4", len(prompts))
	}
	if prompts[0].Phase != state.PromptPhaseCommand {
		t.Errorf("prompts[0].Phase = %v, want Command", prompts[0].Phase)
	}
	if prompts[1].Phase != state.PromptPhaseComplete {
		t.Errorf("prompts[1].Phase = %v, want Complete", prompts[1].Phase)
	}
	if prompts[1].ExitCode == nil || *prompts[1].ExitCode != 0 {
		t.Errorf("prompts[1].ExitCode = %v, want 0", prompts[1].ExitCode)
	}
	if prompts[2].Phase != state.PromptPhaseCommand {
		t.Errorf("prompts[2].Phase = %v, want Command", prompts[2].Phase)
	}
	if prompts[3].Phase != state.PromptPhaseComplete {
		t.Errorf("prompts[3].Phase = %v, want Complete", prompts[3].Phase)
	}
	if prompts[3].ExitCode == nil || *prompts[3].ExitCode != 42 {
		t.Errorf("prompts[3].ExitCode = %v, want 42", prompts[3].ExitCode)
	}
}

func TestParseOscNotification_OSC9(t *testing.T) {
	title, body := parseOscNotification(vt.OscNotification{Cmd: 9, Payload: "  hello  "})
	if title != "hello" || body != "" {
		t.Errorf("got title=%q body=%q", title, body)
	}
}

func TestParseOscNotification_OSC777(t *testing.T) {
	title, body := parseOscNotification(vt.OscNotification{Cmd: 777, Payload: "notify;My Title;My Body"})
	if title != "My Title" || body != "My Body" {
		t.Errorf("got title=%q body=%q", title, body)
	}
}

func TestParseOscNotification_OSC99(t *testing.T) {
	title, body := parseOscNotification(vt.OscNotification{Cmd: 99, Payload: "i=1:d=Alert:p=Something happened"})
	if title != "Alert" || body != "Something happened" {
		t.Errorf("got title=%q body=%q", title, body)
	}
}

func TestReadTapCancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan []byte)

	done := make(chan struct{})
	go func() {
		readTap(ctx, "f1", "%1", ch, func(state.Event) {})
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("readTap did not exit after context cancel")
	}
}

// Regression: the pane-tap VT emulator runs at 1x1 dimensions and the
// upstream charmbracelet/x/vt library panics with "index out of range"
// when certain ESC sequences (e.g. CSI M / DECRC / ESC M) drive
// Screen.InsertLineArea past the buffer bounds. The panic was raised in
// the per-frame readTap goroutine and, with no recovery, terminated the
// whole daemon — closing every IPC socket and leaving every TUI pane
// dead. feedSafe must absorb the panic, log it, and let the reader move
// on to the next chunk so a single rogue codex frame can't take down
// the daemon.
func TestFeedSafe_RecoversFromVTPanic(t *testing.T) {
	// Real reproduction: a 1x1 vt.Terminal fed with ESC M (reverseIndex)
	// triggers ScrollDown → InsertLine → InsertLineArea index-out-of-range.
	term := vt.New(1, 1)
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("feedSafe must absorb the vt panic; bubbled: %v", rec)
		}
	}()
	feedSafe(state.FrameID("f1"), "%1", term, []byte("\x1bM\x1bM\x1bM"))
}

// After a recovered panic on a previous chunk, subsequent chunks must
// still be processed — otherwise the frame's OSC events go silent and
// the daemon's view of the world diverges.
func TestFeedSafe_ContinuesAfterPanic(t *testing.T) {
	frameID := state.FrameID("f1")
	var events []state.Event
	enqueue := func(e state.Event) { events = append(events, e) }
	term := newPaneTapTerminal(frameID, enqueue)
	// Chunk 1: ESC sequences that crash the 1x1 emulator.
	feedSafe(frameID, "%1", term, []byte("\x1bM\x1bM\x1bM"))
	// Chunk 2: a well-formed OSC notification must still come through.
	feedSafe(frameID, "%1", term, []byte("\x1b]9;ping\x07"))

	var gotOSC bool
	for _, ev := range events {
		if o, ok := ev.(state.EvPaneOsc); ok && o.Cmd == 9 && o.Title == "ping" {
			gotOSC = true
		}
	}
	if !gotOSC {
		t.Fatalf("expected OSC 9 event after recovery, got events=%+v", events)
	}
}

// A panic in one frame's goroutine must not propagate beyond feedSafe
// into the caller (readTap). This guards the for-loop that drains the
// channel — without recovery, the goroutine dies and the daemon dies.
func TestReadTap_SurvivesVTPanic(t *testing.T) {
	frameID := state.FrameID("f1")
	ch := make(chan []byte, 4)
	// 1x1 emulator panics on these.
	ch <- []byte("\x1bM\x1bM\x1bM")
	// Followed by a legitimate OSC payload that must arrive.
	ch <- []byte("\x1b]9;after-panic\x07")
	close(ch)

	var events []state.Event
	enqueue := func(e state.Event) { events = append(events, e) }
	done := make(chan struct{})
	go func() {
		readTap(context.Background(), frameID, "%1", ch, enqueue)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("readTap did not return after channel close (likely killed by panic instead of recovering)")
	}

	var gotOSC bool
	for _, ev := range events {
		if o, ok := ev.(state.EvPaneOsc); ok && o.Title == "after-panic" {
			gotOSC = true
		}
	}
	if !gotOSC {
		t.Fatalf("expected post-panic OSC event, got %+v", events)
	}
}

func TestStartRestoredTaps_StartsOnlyRootFrames(t *testing.T) {
	tap := &fakePaneTap{}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tap:          tap,
	})
	// frame_a is root of session s1, frame_b is root of session s2.
	// frame_c is a non-root child frame and must NOT get a tap.
	r.state.Sessions[state.SessionID("s1")] = state.Session{
		ID: "s1",
		Frames: []state.SessionFrame{
			{ID: state.FrameID("frame_a")},
			{ID: state.FrameID("frame_c")},
		},
	}
	r.state.Sessions[state.SessionID("s2")] = state.Session{
		ID:     "s2",
		Frames: []state.SessionFrame{{ID: state.FrameID("frame_b")}},
	}
	r.sessionPanes["frame_a"] = "%1"
	r.sessionPanes["frame_b"] = "%2"
	r.sessionPanes["frame_c"] = "%3"
	r.sessionPanes["_main"] = "%0"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.taps = newTapManager(ctx, tap)

	r.startRestoredTaps()

	got := tap.startedSorted()
	want := []string{"%1", "%2"}
	if len(got) != len(want) {
		t.Fatalf("started panes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("started[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStartRestoredTaps_NoTapsWhenNilManager(t *testing.T) {
	tap := &fakePaneTap{}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tap:          tap,
	})
	r.sessionPanes["frame_a"] = "%1"
	// r.taps left nil (Run not started)

	r.startRestoredTaps() // must not panic

	if len(tap.started) != 0 {
		t.Errorf("tap unexpectedly started while taps manager was nil: %v", tap.started)
	}
}

func TestStartTapsForRestoredFrames_DispatchesViaEventLoop(t *testing.T) {
	tap := &fakePaneTap{}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Millisecond,
		Tap:          tap,
		Tmux:         noopTmux{},
	})
	r.state.Sessions[state.SessionID("s1")] = state.Session{
		ID:     "s1",
		Frames: []state.SessionFrame{{ID: state.FrameID("frame_a")}},
	}
	r.sessionPanes["frame_a"] = "%1"
	r.sessionPanes["_main"] = "%0"

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = r.Run(ctx) }()

	r.StartTapsForRestoredFrames()

	deadline := time.Now().Add(1 * time.Second)
	for len(tap.startedSorted()) != 1 { //nolint:staticcheck
		if time.Now().After(deadline) {
			t.Fatalf("tap never started; got %v", tap.startedSorted())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := tap.startedSorted(); got[0] != "%1" {
		t.Errorf("started = %v, want [%%1]", got)
	}
}
