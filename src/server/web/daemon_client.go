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
// connection to the arc daemon over a Unix socket. A supervisor goroutine
// reconnects on disconnect using full-jitter exponential backoff.
//
// Callers obtain event notifications via SubscribeEvents; the returned channel
// is closed on disconnect and replaced on reconnect, so callers should call
// SubscribeEvents again after observing a closure.
type DaemonClient struct {
	dial     dialFunc
	sockPath string

	mu     sync.RWMutex
	cli    *proto.Client         // nil while disconnected
	events chan proto.ServerEvent // closed and replaced on each reconnect

	health      atomic.Bool
	lastErr     atomic.Pointer[error]
	lastAttempt atomic.Pointer[time.Time]

	stop chan struct{}
	once sync.Once

	// tunable for tests
	minDelay time.Duration
	maxDelay time.Duration
}

// NewDaemonClient eagerly dials sockPath then starts the supervisor goroutine
// that watches the connection and reconnects on loss.
func NewDaemonClient(sockPath string) *DaemonClient {
	d := &DaemonClient{
		sockPath: sockPath,
		dial:     func() (*proto.Client, error) { return proto.Dial(sockPath) },
		events:   make(chan proto.ServerEvent, 64),
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
		events:   make(chan proto.ServerEvent, 64),
		stop:     make(chan struct{}),
		minDelay: minDelay,
		maxDelay: maxDelay,
	}
	d.tryDialOnce(1)
	go d.supervise()
	return d
}

// Close shuts down the supervisor goroutine and the underlying connection.
// Idempotent.
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

// SubscribeEvents returns the current events channel. The channel is closed
// when the daemon disconnects; callers should call SubscribeEvents again to
// obtain the replacement channel after reconnection.
func (d *DaemonClient) SubscribeEvents() <-chan proto.ServerEvent {
	d.mu.RLock()
	ch := d.events
	d.mu.RUnlock()
	return ch
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

		// markDown immediately closes the old events chan so subscribers
		// observe a typed close rather than blocking on a silent channel.
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
			// Reconnected: open a fresh events chan (old one was closed in markDown).
			d.mu.Lock()
			d.events = make(chan proto.ServerEvent, 64)
			d.mu.Unlock()
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

// fanoutEvents drains cli.Events() into d.events until the source channel is
// closed, then signals completion via done.
func (d *DaemonClient) fanoutEvents(cli *proto.Client, done chan<- struct{}) {
	defer close(done)
	src := cli.Events()
	for {
		ev, ok := <-src
		if !ok {
			return
		}
		d.mu.RLock()
		dst := d.events
		d.mu.RUnlock()
		select {
		case dst <- ev:
		default:
			slog.Warn("server/web: daemon event fan-out outbox full, dropping",
				"event", ev.EventName())
		}
	}
}

// markDown updates health state, closes the current events channel so that
// all SubscribeEvents callers observe a typed close immediately, and logs the
// disconnect event.
func (d *DaemonClient) markDown(reason string) {
	d.mu.Lock()
	d.cli = nil
	evCh := d.events
	d.mu.Unlock()

	d.health.Store(false)
	close(evCh) // subscribers observe typed close immediately
	slog.Warn("server/web: daemon disconnected", "reason", reason)
}
