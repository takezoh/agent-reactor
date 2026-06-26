package termvt

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

// fakeEmulator drives session_actor_test without a real vt grid. Its Write
// captures bytes; Read serves whatever Replies returns; Render returns a
// canned snapshot; OSC handlers and Callbacks are stored so a test can
// invoke them directly (e.g. firing OSC 9 during a controlled chunk).
//
// All hooks default to harmless behaviour, so tests only set the ones they
// care about.
type fakeEmulator struct {
	mu sync.Mutex

	WriteHook func(p []byte) // called inside Write while no lock is held
	RenderOut string

	// ScrollbackOut is the canned scrollback payload returned by
	// SerializeScrollback. Tests fill this to exercise the seed shape;
	// leaving it empty makes the actor emit only the screen frame.
	ScrollbackOut []byte

	// CursorX / CursorY are returned by CursorPosition. Default (0, 0)
	// matches a fresh emulator; tests that care about cursor pinning set
	// these explicitly and assert the seed's trailing CUP escape.
	CursorX, CursorY int

	written []byte
	closed  atomic.Bool

	// Replies bytes Read should emit. Drained as Read is called. EOF when
	// empty AND Close has been called.
	replies      chan []byte
	closeReplies sync.Once

	callbacks vt.Callbacks
	osc       map[int]vt.OscHandler
}

func newFakeEmulator() *fakeEmulator {
	return &fakeEmulator{
		replies: make(chan []byte, 8),
		osc:     map[int]vt.OscHandler{},
	}
}

func (e *fakeEmulator) Write(p []byte) (int, error) {
	if e.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	hook := e.WriteHook
	if hook != nil {
		hook(p)
	}
	e.mu.Lock()
	e.written = append(e.written, p...)
	e.mu.Unlock()
	return len(p), nil
}

func (e *fakeEmulator) Read(p []byte) (int, error) {
	b, ok := <-e.replies
	if !ok {
		return 0, io.EOF
	}
	return copy(p, b), nil
}

func (e *fakeEmulator) Render() string                            { return e.RenderOut }
func (e *fakeEmulator) Resize(_, _ int)                           {}
func (e *fakeEmulator) SetCallbacks(cb vt.Callbacks)              { e.callbacks = cb }
func (e *fakeEmulator) RegisterOscHandler(c int, h vt.OscHandler) { e.osc[c] = h }
func (e *fakeEmulator) SetScrollbackSize(_ int)                   {}
func (e *fakeEmulator) SerializeScrollback() []byte               { return e.ScrollbackOut }
func (e *fakeEmulator) CursorPosition() (int, int)                { return e.CursorX, e.CursorY }
func (e *fakeEmulator) CloseInputPipe() error {
	if e.closed.Swap(true) {
		return nil
	}
	e.closeReplies.Do(func() { close(e.replies) })
	return nil
}

// fakePTY is a pty stand-in: Read draws chunks the test feeds via `in`,
// Write discards (no test currently asserts pty writes), Close unblocks a
// pending Read with io.EOF.
type fakePTY struct {
	in chan []byte

	closeOnce sync.Once
	closed    chan struct{}
}

func newFakePTY() *fakePTY {
	return &fakePTY{
		in:     make(chan []byte, 8),
		closed: make(chan struct{}),
	}
}

func (p *fakePTY) Read(buf []byte) (int, error) {
	select {
	case b, ok := <-p.in:
		if !ok {
			return 0, io.EOF
		}
		return copy(buf, b), nil
	case <-p.closed:
		return 0, io.EOF
	}
}

func (p *fakePTY) Write(b []byte) (int, error) {
	select {
	case <-p.closed:
		return 0, io.ErrClosedPipe
	default:
	}
	return len(b), nil
}

func (p *fakePTY) Close() error {
	p.closeOnce.Do(func() { close(p.closed) })
	return nil
}

func (p *fakePTY) SetSize(_, _ int) error { return nil }

// helper: build a session against fakes, ready for use in a test.
func newFakeSession(em Emulator, pty PTY) *Session {
	return NewSessionWithDeps(em, pty, nil, 80, 24)
}

// TestActor_SubscribeReceivesSnapshotThenChunk pins the
// snapshot-before-live-chunk ordering inside the actor: Subscribe runs
// between chunks (cmdCh is processed serially with chunkCh), so the very
// first event must be the reattach snapshot and the next must be the live
// output from a chunk fed AFTER Subscribe returned.
func TestActor_SubscribeReceivesSnapshotThenChunk(t *testing.T) {
	em := newFakeEmulator()
	em.RenderOut = "SNAPSHOT"
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()
	pty.in <- []byte("CHUNK")

	// Seed: Render() bytes + trailing CUP pinning the cursor to (0,0).
	first := waitNext(t, ch, time.Second)
	if first.Kind != EventOutput || string(first.Data) != "SNAPSHOT\x1b[1;1H" {
		t.Fatalf("first event = %+v, want EventOutput SNAPSHOT\\x1b[1;1H", first)
	}
	second := waitNext(t, ch, time.Second)
	if second.Kind != EventOutput || string(second.Data) != "CHUNK" {
		t.Fatalf("second event = %+v, want EventOutput CHUNK", second)
	}
}

// TestActor_SubscribeSeedWithScrollback pins the two-frame seed shape: when
// the emulator's SerializeScrollback returns bytes, Subscribe emits a first
// EventOutput carrying those bytes with a trailing newline separator, then a
// second EventOutput carrying the screen render. xterm.js writes the two
// frames back-to-back; the newline keeps the screen render from
// concatenating onto the last scrollback row.
//
// The empty-scrollback case (frame elided) is covered by
// TestActor_SubscribeReceivesSnapshotThenChunk above — its fake leaves
// ScrollbackOut nil and the test asserts the very first frame is the
// snapshot.
func TestActor_SubscribeSeedWithScrollback(t *testing.T) {
	em := newFakeEmulator()
	em.RenderOut = "SCREEN"
	em.ScrollbackOut = []byte("old1\nold2")
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()

	first := waitNext(t, ch, time.Second)
	if first.Kind != EventOutput || string(first.Data) != "old1\nold2\n" {
		t.Fatalf("first event = %+v, want EventOutput old1\\nold2\\n", first)
	}
	second := waitNext(t, ch, time.Second)
	if second.Kind != EventOutput || string(second.Data) != "SCREEN\x1b[1;1H" {
		t.Fatalf("second event = %+v, want EventOutput SCREEN\\x1b[1;1H", second)
	}
}

// TestActor_SubscribeSeedPinsCursorWithCUP asserts that the second seed
// frame ends with a CUP escape that places xterm.js's cursor at the same
// (x, y) the emulator holds. Without this, Render() bytes leave the
// browser-side cursor at the bottom-right of the rendered grid and any
// subsequent shell echo gets painted at the wrong screen cell (the
// "session-switch input position is broken" regression — Web UI bug
// fixed by this commit). CUP is 1-based; emulator coords are 0-based.
func TestActor_SubscribeSeedPinsCursorWithCUP(t *testing.T) {
	em := newFakeEmulator()
	em.RenderOut = "SCREEN"
	em.CursorX = 12
	em.CursorY = 4
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	_, ch := s.Subscribe()

	ev := waitNext(t, ch, time.Second)
	want := "SCREEN\x1b[5;13H"
	if ev.Kind != EventOutput || string(ev.Data) != want {
		t.Fatalf("seed event = %+v, want EventOutput %q", ev, want)
	}
}

// TestActor_ExitCodeNeverGoesThroughMainLoop hammers ExitCode while mainLoop
// is parked in a deliberately slow em.Write (the fake's WriteHook sleeps).
// Every ExitCode must return within milliseconds — it lives on atomics and
// must not be routed through cmdCh. This is the structural invariant that
// keeps the runtime's dispatch goroutine responsive under any backend
// latency.
func TestActor_ExitCodeNeverGoesThroughMainLoop(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	chunkArrived := make(chan struct{})
	em.WriteHook = func(_ []byte) {
		select {
		case chunkArrived <- struct{}{}:
		default:
		}
		time.Sleep(300 * time.Millisecond) // park mainLoop here
	}
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	pty.in <- []byte("slow")
	<-chunkArrived // mainLoop is now inside em.Write

	// 200 concurrent ExitCode calls, each must finish under 10ms.
	const N = 200
	var maxNs atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			_, _ = s.ExitCode()
			d := time.Since(start).Nanoseconds()
			for {
				cur := maxNs.Load()
				if d <= cur || maxNs.CompareAndSwap(cur, d) {
					break
				}
			}
		}()
	}
	wg.Wait()
	if maxNs.Load() > int64(10*time.Millisecond) {
		t.Fatalf("ExitCode max latency = %dns (> 10ms) — looks like it routed through mainLoop",
			maxNs.Load())
	}
}

// TestActor_SubscribeIDsAreUniqueAndNonZero pins the sentinel contract:
// mainLoop must allocate ids strictly greater than zero so the post-shutdown
// sentinel 0 is unambiguous. A regression here would make callers like
// TerminalRelay (which trusts the id) silently break re-subscribe when a
// real id collides with the shutdown value.
func TestActor_SubscribeIDsAreUniqueAndNonZero(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	s := newFakeSession(em, pty)
	defer func() { _ = s.Close() }()

	seen := map[int]bool{}
	for i := 0; i < 5; i++ {
		id, _ := s.Subscribe()
		if id == 0 {
			t.Fatalf("live Subscribe #%d returned id 0 — collides with shutdown sentinel", i)
		}
		if seen[id] {
			t.Fatalf("Subscribe #%d returned duplicate id %d", i, id)
		}
		seen[id] = true
	}
}

// TestActor_SubscribeAfterShutdownReturnsClosedChannel pins the actor's
// post-exit contract: Subscribe (and any other RPC) must not deadlock if
// mainLoop has already exited. The pre-actor implementation leaked a
// goroutine waiting on events that would never come; the actor returns a
// closed channel so the caller's select sees EOF immediately.
func TestActor_SubscribeAfterShutdownReturnsClosedChannel(t *testing.T) {
	em := newFakeEmulator()
	pty := newFakePTY()
	s := newFakeSession(em, pty)

	_ = s.Close()
	// Wait for mainLoop to actually exit (Close just nudges shutdown). Loop
	// up to a generous deadline; ExitCode flips when handleExit runs.
	deadline := time.After(2 * time.Second)
	for {
		if _, exited := s.ExitCode(); exited {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Session did not finish shutdown within 2s")
		case <-time.After(5 * time.Millisecond):
		}
	}

	id, ch := s.Subscribe()
	if id != 0 {
		t.Errorf("post-shutdown Subscribe id = %d, want 0 sentinel", id)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("post-shutdown Subscribe channel delivered an event")
		}
	case <-time.After(time.Second):
		t.Fatal("post-shutdown Subscribe channel did not close")
	}
}

// waitNext returns the next event on ch or fails the test.
func waitNext(t *testing.T, ch <-chan Event, d time.Duration) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed unexpectedly")
		}
		return ev
	case <-time.After(d):
		t.Fatal("timeout waiting for event")
		return Event{}
	}
}
