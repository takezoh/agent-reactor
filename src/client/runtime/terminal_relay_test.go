package runtime

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/termvt"
)

// fakeSurfaceBackend is a test double for SurfaceBackend. Each call to
// SubscribeSurface allocates a new subscriber id and a fresh buffered channel.
// Tests can push events into the channel via Send, or close it via Close to
// simulate termvt slow-close.
type fakeSurfaceBackend struct {
	mu      sync.Mutex
	nextID  atomic.Int32
	subs    map[int]chan termvt.Event // id → channel
	written []writeCall
	resized []resizeCall
}

type writeCall struct {
	paneID string
	data   []byte
}

type resizeCall struct {
	paneID string
	cols   int
	rows   int
}

func newFakeSurfaceBackend() *fakeSurfaceBackend {
	return &fakeSurfaceBackend{subs: make(map[int]chan termvt.Event)}
}

func (f *fakeSurfaceBackend) SubscribeSurface(paneID string) (int, <-chan termvt.Event, error) {
	id := int(f.nextID.Add(1))
	ch := make(chan termvt.Event, 32)
	f.mu.Lock()
	f.subs[id] = ch
	f.mu.Unlock()
	return id, ch, nil
}

func (f *fakeSurfaceBackend) UnsubscribeSurface(paneID string, id int) error {
	f.mu.Lock()
	delete(f.subs, id)
	f.mu.Unlock()
	return nil
}

func (f *fakeSurfaceBackend) WriteSurface(paneID string, data []byte) error {
	f.mu.Lock()
	f.written = append(f.written, writeCall{paneID: paneID, data: data})
	f.mu.Unlock()
	return nil
}

func (f *fakeSurfaceBackend) ResizeSurface(paneID string, cols, rows int) error {
	f.mu.Lock()
	f.resized = append(f.resized, resizeCall{paneID: paneID, cols: cols, rows: rows})
	f.mu.Unlock()
	return nil
}

// Send pushes ev into the channel for the given subscriber id.
func (f *fakeSurfaceBackend) Send(id int, ev termvt.Event) {
	f.mu.Lock()
	ch := f.subs[id]
	f.mu.Unlock()
	if ch != nil {
		ch <- ev
	}
}

// CloseID closes the channel for the given subscriber id, simulating termvt
// slow-close on process exit.
func (f *fakeSurfaceBackend) CloseID(id int) {
	f.mu.Lock()
	ch, ok := f.subs[id]
	if ok {
		delete(f.subs, id)
	}
	f.mu.Unlock()
	if ok {
		close(ch)
	}
}

// --- helpers ---

// collectEvents drains the send channel until timeout or n events are received.
func collectEvents(t *testing.T, ch <-chan internalEvent, n int, timeout time.Duration) []internalEvent {
	t.Helper()
	var out []internalEvent
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
	return out
}

// newTestTerminalRelay wires up a TerminalRelay whose send function posts to
// the returned channel, making test assertions straightforward.
func newTestTerminalRelay(t *testing.T, b SurfaceBackend) (*TerminalRelay, <-chan internalEvent) {
	t.Helper()
	ch := make(chan internalEvent, 64)
	send := func(ev internalEvent) { ch <- ev }
	return NewTerminalRelay(b, send), ch
}

const (
	conn1 = state.ConnID(1)
	conn2 = state.ConnID(2)
	sess1 = state.SessionID("sess-1")
	sess2 = state.SessionID("sess-2")
)

// TestTerminalRelay_SnapshotSequenceZero: first EventOutput gets Sequence == 0
// and Data is preserved correctly.
func TestTerminalRelay_SnapshotSequenceZero(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	id := int(b.nextID.Load())
	payload := []byte("hello snapshot")
	b.Send(id, termvt.Event{Kind: termvt.EventOutput, Data: payload})

	got := collectEvents(t, events, 1, time.Second)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	bs, ok := got[0].(internalBroadcastSurface)
	if !ok {
		t.Fatalf("expected internalBroadcastSurface, got %T", got[0])
	}
	if bs.Sequence != 0 {
		t.Errorf("Sequence = %d, want 0", bs.Sequence)
	}
	if string(bs.Data) != string(payload) {
		t.Errorf("Data = %q, want %q", bs.Data, payload)
	}
	if bs.ConnID != conn1 {
		t.Errorf("ConnID = %v, want %v", bs.ConnID, conn1)
	}
	if bs.SessionID != sess1 {
		t.Errorf("SessionID = %v, want %v", bs.SessionID, sess1)
	}
}

// TestTerminalRelay_SequenceMonotonic: Sequence increments 0,1,2,3 across
// four consecutive EventOutput events on the same subscription.
func TestTerminalRelay_SequenceMonotonic(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	id := int(b.nextID.Load())

	for i := 0; i < 4; i++ {
		b.Send(id, termvt.Event{Kind: termvt.EventOutput, Data: []byte{byte(i)}})
	}

	got := collectEvents(t, events, 4, time.Second)
	if len(got) != 4 {
		t.Fatalf("expected 4 events, got %d", len(got))
	}
	for i, ev := range got {
		bs, ok := ev.(internalBroadcastSurface)
		if !ok {
			t.Fatalf("event[%d]: expected internalBroadcastSurface, got %T", i, ev)
		}
		if bs.Sequence != uint64(i) {
			t.Errorf("event[%d]: Sequence = %d, want %d", i, bs.Sequence, i)
		}
	}
}

// TestTerminalRelay_SubscribeRestartsSequence: Unsubscribe then re-Subscribe
// on the same (ConnID, SessionID) resets Sequence to 0 (ADR 0010).
func TestTerminalRelay_SubscribeRestartsSequence(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	id1 := int(b.nextID.Load())
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("a")})
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("b")})

	// Wait for both events to be delivered before unsubscribing.
	got := collectEvents(t, events, 2, time.Second)
	if len(got) != 2 {
		t.Fatalf("expected 2 events before Unsubscribe, got %d", len(got))
	}

	tr.Unsubscribe(conn1, sess1)

	// Re-subscribe — Sequence must restart from 0.
	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("re-Subscribe: %v", err)
	}
	id2 := int(b.nextID.Load())
	b.Send(id2, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c")})

	got2 := collectEvents(t, events, 1, time.Second)
	if len(got2) != 1 {
		t.Fatalf("expected 1 event after re-Subscribe, got %d", len(got2))
	}
	bs, ok := got2[0].(internalBroadcastSurface)
	if !ok {
		t.Fatalf("expected internalBroadcastSurface, got %T", got2[0])
	}
	if bs.Sequence != 0 {
		t.Errorf("Sequence after re-Subscribe = %d, want 0", bs.Sequence)
	}
}

// TestTerminalRelay_SlowCloseEmitsClosedEvent: when the backend closes the
// channel (process exit), exactly one internalSurfaceClosed is emitted.
func TestTerminalRelay_SlowCloseEmitsClosedEvent(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	id := int(b.nextID.Load())

	b.CloseID(id)

	got := collectEvents(t, events, 1, time.Second)
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	sc, ok := got[0].(internalSurfaceClosed)
	if !ok {
		t.Fatalf("expected internalSurfaceClosed, got %T", got[0])
	}
	if sc.ConnID != conn1 || sc.SessionID != sess1 {
		t.Errorf("closed event = {%v, %v}, want {%v, %v}", sc.ConnID, sc.SessionID, conn1, sess1)
	}

	// No additional events should arrive.
	extra := collectEvents(t, events, 1, 50*time.Millisecond)
	if len(extra) != 0 {
		t.Errorf("unexpected extra events: %v", extra)
	}
}

// TestTerminalRelay_Write: Write forwards data to backend.WriteSurface.
func TestTerminalRelay_Write(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, _ := newTestTerminalRelay(t, b)
	defer tr.Close()

	data := []byte("input bytes")
	if err := tr.Write("%1", data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.written) != 1 {
		t.Fatalf("expected 1 WriteSurface call, got %d", len(b.written))
	}
	if b.written[0].paneID != "%1" {
		t.Errorf("paneID = %q, want %%1", b.written[0].paneID)
	}
	if string(b.written[0].data) != string(data) {
		t.Errorf("data = %q, want %q", b.written[0].data, data)
	}
}

// TestTerminalRelay_Resize: Resize forwards dimensions to backend.ResizeSurface.
func TestTerminalRelay_Resize(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, _ := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Resize("%1", 80, 24); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.resized) != 1 {
		t.Fatalf("expected 1 ResizeSurface call, got %d", len(b.resized))
	}
	rc := b.resized[0]
	if rc.paneID != "%1" || rc.cols != 80 || rc.rows != 24 {
		t.Errorf("resize = {%q, %d, %d}, want {%%1, 80, 24}", rc.paneID, rc.cols, rc.rows)
	}
}

// TestTerminalRelay_UnsubscribeIdempotent: calling Unsubscribe twice must not
// panic or produce errors.
func TestTerminalRelay_UnsubscribeIdempotent(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, _ := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	tr.Unsubscribe(conn1, sess1)
	tr.Unsubscribe(conn1, sess1) // should not panic
}

// TestTerminalRelay_TwoConnsIndependent: two ConnIDs subscribing to the same
// paneID get independent termvt subscriber ids and independent Sequence counters.
func TestTerminalRelay_TwoConnsIndependent(t *testing.T) {
	b := newFakeSurfaceBackend()
	tr, events := newTestTerminalRelay(t, b)
	defer tr.Close()

	if err := tr.Subscribe(conn1, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe conn1: %v", err)
	}
	id1 := int(b.nextID.Load())

	if err := tr.Subscribe(conn2, sess1, "%1"); err != nil {
		t.Fatalf("Subscribe conn2: %v", err)
	}
	id2 := int(b.nextID.Load())

	if id1 == id2 {
		t.Fatalf("expected different subscriber ids, both got %d", id1)
	}

	// Send two events to conn1's sub and one to conn2's sub.
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c1-a")})
	b.Send(id1, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c1-b")})
	b.Send(id2, termvt.Event{Kind: termvt.EventOutput, Data: []byte("c2-a")})

	got := collectEvents(t, events, 3, time.Second)
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}

	// Bucket by ConnID.
	seqByConn := map[state.ConnID][]uint64{}
	for _, ev := range got {
		bs, ok := ev.(internalBroadcastSurface)
		if !ok {
			t.Fatalf("expected internalBroadcastSurface, got %T", ev)
		}
		seqByConn[bs.ConnID] = append(seqByConn[bs.ConnID], bs.Sequence)
	}

	if len(seqByConn[conn1]) != 2 {
		t.Errorf("conn1 events = %d, want 2", len(seqByConn[conn1]))
	}
	if len(seqByConn[conn2]) != 1 {
		t.Errorf("conn2 events = %d, want 1", len(seqByConn[conn2]))
	}
	// conn2 must start at Sequence 0, not carry over conn1's counter.
	if seqByConn[conn2][0] != 0 {
		t.Errorf("conn2 first Sequence = %d, want 0", seqByConn[conn2][0])
	}
}
