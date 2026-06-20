package runtime

import (
	"sync"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/termvt"
)

// SurfaceBackend is the subset of PtyBackend that TerminalRelay depends on.
// It is extracted as an interface so tests can inject a fake implementation
// without starting a real pty.
type SurfaceBackend interface {
	SubscribeSurface(paneID string) (int, <-chan termvt.Event, error)
	UnsubscribeSurface(paneID string, id int) error
	WriteSurface(paneID string, data []byte) error
	ResizeSurface(paneID string, cols, rows int) error
}

// surfaceKey is the map key for a per-(ConnID, SessionID) subscription.
type surfaceKey struct {
	connID    state.ConnID
	sessionID state.SessionID
}

// surfaceSub holds the live state of one fan-out goroutine.
type surfaceSub struct {
	paneID string
	subID  int           // termvt subscriber id returned by SubscribeSurface
	cancel chan struct{} // closed to stop the fan-out goroutine early
	seq    uint64        // next Sequence value to emit (subscribe-scoped, resets on re-subscribe)
}

// TerminalRelay manages per-(ConnID, SessionID) subscriptions to termvt
// sessions and fans EventOutput chunks out as internalBroadcastSurface events
// on the runtime event loop. It is a reducer-bypass goroutine in the same
// spirit as FileRelay.
//
// Public API (4 methods):
//   - Subscribe / Unsubscribe manage the fan-out goroutine lifecycle.
//   - Write / Resize forward raw input and resize requests to the backend.
//   - Close tears everything down.
type TerminalRelay struct {
	backend SurfaceBackend
	// send posts an internal event onto the runtime event loop. TerminalRelay
	// holds only this bound function (not *Runtime) so its fan-out goroutines
	// cannot touch loop-owned state directly.
	send    func(internalEvent)
	startTS time.Time // base for TimeSec computation

	mu   sync.Mutex
	subs map[surfaceKey]*surfaceSub
}

// NewTerminalRelay creates a TerminalRelay that forwards surface events via send.
// send is typically rt.enqueueInternal, bound at construction time.
func NewTerminalRelay(b SurfaceBackend, send func(internalEvent)) *TerminalRelay {
	return &TerminalRelay{
		backend: b,
		send:    send,
		startTS: time.Now(),
		subs:    make(map[surfaceKey]*surfaceSub),
	}
}

// Subscribe starts a fan-out goroutine for (connID, sessionID) on paneID.
// If a subscription already exists for that key it is a no-op (idempotent).
func (tr *TerminalRelay) Subscribe(connID state.ConnID, sessionID state.SessionID, paneID string) error {
	key := surfaceKey{connID: connID, sessionID: sessionID}

	tr.mu.Lock()
	if _, exists := tr.subs[key]; exists {
		tr.mu.Unlock()
		return nil
	}
	tr.mu.Unlock()

	id, ch, err := tr.backend.SubscribeSurface(paneID)
	if err != nil {
		return err
	}

	sub := &surfaceSub{
		paneID: paneID,
		subID:  id,
		cancel: make(chan struct{}),
	}

	tr.mu.Lock()
	// Double-check after acquiring lock (another goroutine could have raced us).
	if _, exists := tr.subs[key]; exists {
		tr.mu.Unlock()
		_ = tr.backend.UnsubscribeSurface(paneID, id)
		return nil
	}
	tr.subs[key] = sub
	tr.mu.Unlock()

	go tr.fanOut(key, sub, ch)
	return nil
}

// Unsubscribe stops the fan-out goroutine for (connID, sessionID) and
// releases the termvt subscriber. Idempotent — safe to call multiple times.
func (tr *TerminalRelay) Unsubscribe(connID state.ConnID, sessionID state.SessionID) {
	key := surfaceKey{connID: connID, sessionID: sessionID}

	tr.mu.Lock()
	sub, ok := tr.subs[key]
	if !ok {
		tr.mu.Unlock()
		return
	}
	delete(tr.subs, key)
	tr.mu.Unlock()

	close(sub.cancel)
	_ = tr.backend.UnsubscribeSurface(sub.paneID, sub.subID)
}

// Write forwards raw bytes to the pane's pty. No carriage return is appended;
// the caller (xterm.js via the web gateway) is responsible for proper encoding.
func (tr *TerminalRelay) Write(paneID string, data []byte) error {
	return tr.backend.WriteSurface(paneID, data)
}

// Resize forwards a terminal resize to the pane's pty and VT emulator.
func (tr *TerminalRelay) Resize(paneID string, cols, rows int) error {
	return tr.backend.ResizeSurface(paneID, cols, rows)
}

// Close unsubscribes all active subscriptions and shuts down TerminalRelay.
func (tr *TerminalRelay) Close() {
	tr.mu.Lock()
	keys := make([]surfaceKey, 0, len(tr.subs))
	for k := range tr.subs {
		keys = append(keys, k)
	}
	tr.mu.Unlock()

	for _, key := range keys {
		tr.Unsubscribe(key.connID, key.sessionID)
	}
}

// fanOut runs in a dedicated goroutine per subscription. It receives termvt
// events from ch, copies EventOutput payloads into internalBroadcastSurface,
// and enqueues them on the runtime event loop. When the channel is closed
// (slow-close by termvt on process exit) it emits one internalSurfaceClosed
// and exits. When cancel is closed (Unsubscribe / Close) it exits immediately.
func (tr *TerminalRelay) fanOut(key surfaceKey, sub *surfaceSub, ch <-chan termvt.Event) {
	for {
		select {
		case <-sub.cancel:
			return
		case ev, ok := <-ch:
			if !ok {
				// termvt slow-closed the channel. Remove the entry from the
				// local subs map BEFORE notifying the reducer so that even if
				// the non-blocking send drops the internalSurfaceClosed event
				// (when the runtime's internal queue is saturated), we do not
				// leak tr.subs[key]. The reducer's SurfaceSubs reconciliation
				// may then be slightly delayed but is not lost (a subsequent
				// EvConnClosed for the daemon ConnID will clean state anyway).
				tr.mu.Lock()
				if cur, ok := tr.subs[key]; ok && cur == sub {
					delete(tr.subs, key)
				}
				tr.mu.Unlock()
				tr.send(internalSurfaceClosed{
					ConnID:    key.connID,
					SessionID: key.sessionID,
				})
				return
			}
			if ev.Kind != termvt.EventOutput {
				continue
			}
			data := make([]byte, len(ev.Data))
			copy(data, ev.Data)

			// sub.seq is owned exclusively by this fanOut goroutine — no
			// other goroutine reads or writes it for the same key
			// (Subscribe / Unsubscribe manage the map under tr.mu, but each
			// fanOut owns its sub pointer for its lifetime). Taking tr.mu
			// here serialised every output event against unrelated
			// Subscribe / Unsubscribe traffic for no correctness benefit.
			seq := sub.seq
			sub.seq++

			tr.send(internalBroadcastSurface{
				ConnID:    key.connID,
				SessionID: key.sessionID,
				Data:      data,
				TimeSec:   time.Since(tr.startTS).Seconds(),
				Sequence:  seq,
			})
		}
	}
}

// === Internal event types ===

// internalBroadcastSurface is enqueued by TerminalRelay when it receives an
// EventOutput chunk. The event loop routes it to the single ConnID that
// subscribed so that EvtSurfaceOutput is streamed over the wire.
type internalBroadcastSurface struct {
	ConnID    state.ConnID
	SessionID state.SessionID
	Data      []byte
	TimeSec   float64
	Sequence  uint64
}

func (internalBroadcastSurface) isInternalEvent() {}

// internalSurfaceClosed is enqueued by TerminalRelay when termvt closes the
// subscriber channel (slow-close on process exit). The event loop uses it to
// remove the entry from state.SurfaceSubs so the client knows the stream ended.
type internalSurfaceClosed struct {
	ConnID    state.ConnID
	SessionID state.SessionID
}

func (internalSurfaceClosed) isInternalEvent() {}
