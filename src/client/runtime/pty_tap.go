package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

// PtyFrameTap implements FrameTap on top of a PtyBackend's termvt.Manager. It is
// the replacement for the legacy pipe-pane tap on the pty backend path (plan A 5a/5b):
// each Start resolves the frame id to a live Session via mgr.Get and subscribes,
// then forwards termvt.EventOutput chunks as the raw byte stream the existing
// tap_manager (and its 1x1 vt.Terminal) consumes.
//
// EventControl is intentionally dropped here — tap_manager re-parses the same
// OSC sequences out of the raw bytes via its vt.Terminal, so feeding structured
// events would double-count. The structured fast path will be wired in plan
// A 5c, once the web view shares the same pure-core consumers as the TUI.
//
// EventExit and slow-subscriber disconnects are both surfaced by termvt as
// channel close; the forwarder treats both the same way (return on !ok) and the
// downstream out chan is closed via the deferred close in forwardEvents.
type PtyFrameTap struct {
	mgr *termvt.Manager

	mu   sync.Mutex
	subs map[string]ptyTapSub
}

// ptyTapSub tracks the live subscription for one frame so Stop can cancel the
// forwarder goroutine and Unsubscribe from termvt.
type ptyTapSub struct {
	subID  int
	sess   *termvt.Session
	cancel context.CancelFunc
}

// NewPtyFrameTap wires a tap onto the same termvt.Manager the backend uses.
// Sharing the Manager is what keeps the frame-id key space consistent: ids
// minted by SpawnFrame/RespawnFrame are exactly the keys mgr.Get resolves.
func NewPtyFrameTap(b *PtyBackend) *PtyFrameTap {
	return &PtyFrameTap{mgr: b.mgr, subs: map[string]ptyTapSub{}}
}

// Start opens a subscription on the Session under frameID and returns a channel
// of raw bytes. tap_manager already serialises Start/Stop per frame, but Start
// is defensive against a duplicate subscription on the same frame: the previous
// subscription is torn down first.
func (t *PtyFrameTap) Start(ctx context.Context, frameID string) (<-chan []byte, error) {
	sess, ok := t.mgr.Get(frameID)
	if !ok {
		return nil, fmt.Errorf("pty_tap: unknown frame %q: %w", frameID, ErrFrameMissing)
	}
	// A Session whose readLoop has already exited will accept Subscribe (it
	// only takes s.mu, with no exited guard); the subscriber receives the
	// snapshot Event Subscribe seeds and then blocks forever — no further
	// events, no EventExit, no channel close, because readLoop is gone.
	// Treat an exited Session as a missing frame so the runtime drops the
	// tap rather than leaking a goroutine on a dead Session.
	//
	// This narrows but does not close a TOCTOU window: the readLoop can
	// complete between this check and sess.Subscribe below. Closing it would
	// require an exited guard inside termvt.Session.Subscribe itself
	// (out-of-scope for plan A0). The residual leak is bounded by the
	// caller-supplied ctx — when tap_manager tears down the frame, the
	// forwarder unblocks via ctx.Done.
	if _, exited := sess.ExitCode(); exited {
		return nil, fmt.Errorf("pty_tap: frame %q already exited: %w", frameID, ErrFrameMissing)
	}

	// Tear down any prior subscription for this frame before opening a new one,
	// so a redundant Start cannot leak a forwarder goroutine.
	_ = t.Stop(frameID)

	subID, in := sess.Subscribe()
	tapCtx, cancel := context.WithCancel(ctx)

	t.mu.Lock()
	t.subs[frameID] = ptyTapSub{subID: subID, sess: sess, cancel: cancel}
	t.mu.Unlock()

	out := make(chan []byte, ptyTapOutBuffer)
	go t.forwardEvents(tapCtx, cancel, in, out, sess, subID, frameID)
	return out, nil
}

// Stop releases the subscription for frameID. Stop is idempotent: it is safe
// to call after the forwarder has already self-terminated (EventExit / slow
// disconnect) or never run (Start error).
func (t *PtyFrameTap) Stop(frameID string) error {
	t.mu.Lock()
	sub, ok := t.subs[frameID]
	if ok {
		delete(t.subs, frameID)
	}
	t.mu.Unlock()
	if !ok {
		return nil
	}
	sub.cancel()
	return nil
}

// forwardEvents drains the termvt subscription channel and forwards only the
// raw output to out. It exits on three conditions:
//   - the termvt channel is closed (EventExit, slow-subscriber disconnect, or
//     an explicit Unsubscribe issued via Stop/ctx cancel),
//   - the tap-scoped context is cancelled (Stop or runtime shutdown), at which
//     point it also Unsubscribes so termvt drops its buffer,
//   - the out channel cannot accept the next chunk because the tap ctx fires.
//
// The deferred close on out is what propagates "session ended" to readTap so it
// drops the frame's tap goroutine and the runtime continues without OSC events
// from that frame.
func (t *PtyFrameTap) forwardEvents(
	ctx context.Context,
	cancel context.CancelFunc,
	in <-chan termvt.Event,
	out chan<- []byte,
	sess *termvt.Session,
	subID int,
	frameID string,
) {
	// cancel must run on every exit path so context.WithCancel's tracking
	// goroutine releases — otherwise the cancel leaks until the parent ctx
	// fires (the `lostcancel` pattern go vet warns about). Stop-driven
	// teardown already cancelled the ctx, so this defer is a no-op there;
	// channel-close exits (EventExit / slow-disconnect) are the path that
	// actually needs it.
	defer cancel()
	defer close(out)
	defer t.forgetSub(frameID, subID, sess)
	for {
		select {
		case ev, ok := <-in:
			if !ok {
				return
			}
			if ev.Kind != termvt.EventOutput {
				// EventControl re-parsed by tap_manager's vt.Terminal (see pkg
				// doc); EventExit is paired with channel close on the next
				// iteration, so no per-event action is needed.
				continue
			}
			select {
			case out <- ev.Data:
			case <-ctx.Done():
				sess.Unsubscribe(subID)
				return
			}
		case <-ctx.Done():
			sess.Unsubscribe(subID)
			return
		}
	}
}

// forgetSub clears the subs entry for frameID only when it still matches the
// (subID, sess) pair, so a Start that already replaced this subscription is
// not undone by the prior forwarder's defer. The sess pointer check guards
// against subID collisions across termvt.Sessions: nextID is a per-Session
// counter, so a RespawnFrame'd frame reuses subID 0 for the new Session and
// the old forwarder would otherwise delete the new entry on its way out.
func (t *PtyFrameTap) forgetSub(frameID string, subID int, sess *termvt.Session) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cur, ok := t.subs[frameID]; ok && cur.subID == subID && cur.sess == sess {
		delete(t.subs, frameID)
	}
}

// ptyTapOutBuffer matches termvt's per-subscriber buffer so the tap is not
// the first backpressure point when readTap stalls briefly (GC pause,
// scheduler latency). When `out` saturates earlier than `in`, the forwarder
// blocks on `out <- ev.Data`, `in` then fills to subBuffer, and termvt
// disconnects the subscriber as "too slow" — a recoverable read pause turns
// into a permanently dead tap. Sizing `out` to the same depth keeps the
// disconnect threshold aligned with termvt's own.
const ptyTapOutBuffer = 256
