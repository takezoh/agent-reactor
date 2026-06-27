package web

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
)

// ErrDaemonUnavailable is returned by SendCommand when the DaemonClient is
// not currently connected to the daemon.
var ErrDaemonUnavailable = errors.New("server/web: daemon unavailable")

// defaultMinDelay / defaultMaxDelay define the exponential backoff bounds for
// supervisor reconnect attempts.
const (
	defaultMinDelay = 250 * time.Millisecond
	defaultMaxDelay = 4 * time.Second
)

// dialFunc is the signature used by the supervisor to open a new connection.
type dialFunc func() (*proto.Client, error)

// DaemonClient is a proto.Client wrapper that maintains a persistent
// connection to the server daemon over a Unix socket. A supervisor goroutine
// reconnects on disconnect using full-jitter exponential backoff.
//
// Callers obtain event notifications via SubscribeEvents; the returned channel
// is per-call (a fresh, independently-buffered chan) and is closed on disconnect
// so callers should call SubscribeEvents again after observing a closure.
//
// Fan-out: every SubscribeEvents call registers a new subscriber. Each daemon
// event is broadcast to every active subscriber (non-blocking; slow subscribers
// have individual events dropped, never affecting other subscribers).
// This is the structural fix for the prior bug where concurrent AttachWS /
// AttachLifecycleWS handlers shared a single channel and stole each other's
// events.
type DaemonClient struct {
	dial     dialFunc
	sockPath string

	mu  sync.RWMutex
	cli *proto.Client // nil while disconnected

	// subsMu is RW: broadcastEvent takes the read lock so multiple fan-outs
	// can run concurrently while excluding closeAllSubs / SubscribeEvents
	// (which take the write lock). Crucially, the read lock keeps every
	// subscriber channel alive for the duration of one broadcast pass, so the
	// non-blocking `select case ch <- ev` send cannot race with `close(ch)`
	// — close() obtains the write lock and must wait for in-flight broadcasts
	// to finish before any channel is closed.
	subsMu sync.RWMutex
	subs   map[chan proto.ServerEvent]struct{}

	health      atomic.Bool
	lastErr     atomic.Pointer[error]
	lastAttempt atomic.Pointer[time.Time]

	stop chan struct{}
	once sync.Once

	// tunable for tests
	minDelay time.Duration
	maxDelay time.Duration
}

// perSubscriberBuf sizes each subscriber's outbox. Slow subscribers have
// individual events dropped (logged at warn) rather than blocking the
// daemon-side reader or other subscribers.
const perSubscriberBuf = 64

// NewDaemonClient eagerly dials sockPath then starts the supervisor goroutine
// that watches the connection and reconnects on loss.
func NewDaemonClient(sockPath string) *DaemonClient {
	d := &DaemonClient{
		sockPath: sockPath,
		dial:     func() (*proto.Client, error) { return proto.Dial(sockPath) },
		subs:     make(map[chan proto.ServerEvent]struct{}),
		stop:     make(chan struct{}),
		minDelay: defaultMinDelay,
		maxDelay: defaultMaxDelay,
	}
	d.tryDialOnce(1)
	go d.supervise()
	return d
}

// NewDaemonClientWithDialer is a test-only constructor that accepts a custom
// dialer and backoff delays, enabling unit tests to inject net.Pipe
// connections without a real Unix socket.
func NewDaemonClientWithDialer(dialFn func() (*proto.Client, error), minDelay, maxDelay time.Duration) *DaemonClient {
	d := &DaemonClient{
		sockPath: "<dialer>",
		dial:     dialFn,
		subs:     make(map[chan proto.ServerEvent]struct{}),
		stop:     make(chan struct{}),
		minDelay: minDelay,
		maxDelay: maxDelay,
	}
	d.tryDialOnce(1)
	go d.supervise()
	return d
}

// Close shuts down the supervisor goroutine and the underlying connection.
// Idempotent. Closes every active subscriber channel so subscribers observe
// EOF and tear down cleanly.
func (d *DaemonClient) Close() {
	d.once.Do(func() {
		close(d.stop)
		// Close the underlying proto.Client so the fanoutEvents goroutine and
		// any in-flight reads are unblocked and can exit cleanly.
		d.mu.Lock()
		cli := d.cli
		d.cli = nil
		d.mu.Unlock()
		if cli != nil {
			cli.Close()
		}
		d.closeAllSubs()
	})
}

// Health returns true when the daemon is currently reachable.
func (d *DaemonClient) Health() bool { return d.health.Load() }

// LastError returns the most recent dial error, or nil if there has been none.
func (d *DaemonClient) LastError() error {
	p := d.lastErr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// LastAttemptAt returns the time of the most recent dial attempt, or the zero
// value if no attempt has been made.
func (d *DaemonClient) LastAttemptAt() time.Time {
	p := d.lastAttempt.Load()
	if p == nil {
		return time.Time{}
	}
	return *p
}

// SendCommand sends a typed command to the daemon and waits for the response.
// Returns ErrDaemonUnavailable when not connected.
func (d *DaemonClient) SendCommand(ctx context.Context, cmd proto.Command) (proto.Response, error) {
	d.mu.RLock()
	cli := d.cli
	d.mu.RUnlock()
	if cli == nil {
		return nil, ErrDaemonUnavailable
	}
	return cli.Send(ctx, cmd)
}

// SubscribeEvents registers a fresh subscriber and returns its dedicated event
// channel. The subscriber is automatically unregistered when ctx is cancelled
// (so that AttachWS / AttachLifecycleWS exits do not leak channels into the
// fan-out map). The channel is also closed when the daemon disconnects or
// DaemonClient is Closed; callers should call SubscribeEvents again to obtain
// a new channel after observing closure. Events broadcast across all
// subscribers are non-blocking per recipient — slow subscribers have
// individual events dropped (logged) but do not block other subscribers or
// the daemon reader.
func (d *DaemonClient) SubscribeEvents(ctx context.Context) <-chan proto.ServerEvent {
	ch := make(chan proto.ServerEvent, perSubscriberBuf)

	d.subsMu.Lock()
	select {
	case <-d.stop:
		d.subsMu.Unlock()
		close(ch)
		return ch
	default:
	}
	d.subs[ch] = struct{}{}
	d.subsMu.Unlock()

	// Auto-unregister when ctx is cancelled (browser disconnect path). Uses
	// the same write lock as closeAllSubs / markDown, so the closeAllSubs
	// case (channel already closed) gracefully no-ops via the membership
	// check rather than panicking on double close.
	go func() {
		<-ctx.Done()
		d.subsMu.Lock()
		if _, ok := d.subs[ch]; ok {
			delete(d.subs, ch)
			close(ch)
		}
		d.subsMu.Unlock()
	}()
	return ch
}

// broadcastEvent fan-outs ev to every registered subscriber. Non-blocking per
// subscriber: a slow subscriber has the event dropped (logged at warn). The
// read lock keeps every channel alive until the fan-out finishes, eliminating
// the send-on-closed-channel race the previous snapshot-then-iterate design
// allowed.
func (d *DaemonClient) broadcastEvent(ev proto.ServerEvent) {
	d.subsMu.RLock()
	defer d.subsMu.RUnlock()
	for ch := range d.subs {
		select {
		case ch <- ev:
		default:
			slog.Warn("server/web: daemon event fan-out outbox full, dropping",
				"event", ev.EventName())
		}
	}
}

// closeAllSubs closes every active subscriber channel and forgets them.
// Subscribers observe an immediate channel close (the documented disconnect
// signal). Idempotent: safe under repeated invocation; obtaining the write
// lock ensures no broadcastEvent is in flight when we close. The close()
// calls happen WHILE the write lock is still held so that no SubscribeEvents
// caller can slip a new subscriber into d.subs in between the map swap and
// the close — otherwise that fresh subscriber would never see the
// disconnect signal and would wait forever on a silent channel.
func (d *DaemonClient) closeAllSubs() {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	for ch := range d.subs {
		close(ch)
	}
	d.subs = make(map[chan proto.ServerEvent]struct{})
}

// tryDialOnce performs a single dial attempt and updates health/lastErr/lastAttempt.
// It closes any prior *proto.Client before overwriting d.cli to avoid conn leaks.
// attempt is the 1-based attempt counter used for logging.
func (d *DaemonClient) tryDialOnce(attempt int) {
	now := time.Now()
	d.lastAttempt.Store(&now)

	cli, err := d.dial()
	if err != nil {
		d.lastErr.Store(&err)
		d.mu.Lock()
		d.cli = nil
		d.mu.Unlock()
		d.health.Store(false)
		return
	}

	d.mu.Lock()
	prev := d.cli // close any prior client that wasn't already cleaned up
	d.cli = cli
	d.mu.Unlock()
	if prev != nil {
		prev.Close()
	}
	d.health.Store(true)
	slog.Info("server/web: daemon connected", "sock", d.sockPath, "attempt", attempt)
}

// supervise runs the reconnect loop. It detects disconnection via the closure
// of the current client's Events channel and retries with backoff.
func (d *DaemonClient) supervise() {
	attempt := 1
	for {
		select {
		case <-d.stop:
			return
		default:
		}

		d.mu.RLock()
		cli := d.cli
		d.mu.RUnlock()

		if cli == nil {
			// Still disconnected; backoff then retry.
			attempt = d.reconnectLoop(attempt)
			continue
		}

		// Start fanout and wait for the client's Events chan to close
		// (signals daemon disconnect or intentional close).
		fanoutDone := make(chan struct{})
		go d.fanoutEvents(cli, fanoutDone)
		select {
		case <-d.stop:
			return
		case <-fanoutDone:
			// Disconnect detected.
		}

		// markDown closes every active subscriber channel so each WS handler
		// observes a typed close rather than blocking silently.
		d.markDown("events channel closed")
		attempt = d.reconnectLoop(attempt)
	}
}

// reconnectLoop retries dial until success or stop is requested.
// Returns the next attempt counter to use after a successful dial.
func (d *DaemonClient) reconnectLoop(attempt int) int {
	delay := d.minDelay
	for {
		select {
		case <-d.stop:
			return attempt
		default:
		}

		jitter := time.Duration(rand.Float64() * float64(delay))
		slog.Warn("server/web: backing off before next dial",
			"sock", d.sockPath,
			"attempt", attempt,
			"delay", jitter)

		select {
		case <-d.stop:
			return attempt
		case <-time.After(jitter):
		}

		attempt++
		d.tryDialOnce(attempt)

		if d.health.Load() {
			// Reconnected: subscribers' channels were closed in markDown, so
			// callers will obtain fresh per-call channels on their next
			// SubscribeEvents.
			return attempt
		}

		slog.Warn("server/web: daemon dial failed",
			"sock", d.sockPath,
			"attempt", attempt,
			"err", d.LastError(),
			"delay", jitter)

		// Increase delay (capped at maxDelay).
		delay *= 2
		if delay > d.maxDelay {
			delay = d.maxDelay
		}
	}
}

// fanoutEvents drains cli.Events() and broadcasts each event to every active
// SubscribeEvents subscriber. Exits when the source channel closes, signalling
// completion via done.
func (d *DaemonClient) fanoutEvents(cli *proto.Client, done chan<- struct{}) {
	defer close(done)
	src := cli.Events()
	for {
		ev, ok := <-src
		if !ok {
			return
		}
		d.broadcastEvent(ev)
	}
}

// markDown updates health state, closes every active subscriber channel so all
// SubscribeEvents callers observe a typed close immediately, and logs the
// disconnect event.
func (d *DaemonClient) markDown(reason string) {
	d.mu.Lock()
	d.cli = nil
	d.mu.Unlock()

	d.health.Store(false)
	d.closeAllSubs() // every subscriber observes typed close immediately
	slog.Warn("server/web: daemon disconnected", "reason", reason)
}
