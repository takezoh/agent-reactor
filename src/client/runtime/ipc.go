package runtime

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"github.com/takezoh/agent-reactor/client/proto"
	rsubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/pathmap"
)

// ipcConn is one accepted client connection. The reader goroutine
// decodes envelopes off the socket and forwards typed Commands to
// the runtime as state.Event values; the writer goroutine drains
// outbox and writes wire bytes to the socket.
type ipcConn struct {
	id      state.ConnID
	conn    net.Conn
	outbox  chan []byte
	done    chan struct{}
	once    sync.Once
	writeMu sync.Mutex
}

const ipcOutboxSize = 64

func newIPCConn(id state.ConnID, conn net.Conn) *ipcConn {
	return &ipcConn{
		id:     id,
		conn:   conn,
		outbox: make(chan []byte, ipcOutboxSize),
		done:   make(chan struct{}),
	}
}

// shut closes the connection and signals the writer to exit.
// Idempotent.
func (cc *ipcConn) shut() {
	cc.once.Do(func() {
		close(cc.done)
		_ = cc.conn.Close()
	})
}

// === Listener / accept loop ===

// StartIPC opens the Unix socket and starts the accept loop. Should
// be called from main after Run is already running (so the accept
// loop can call Enqueue).
func (r *Runtime) StartIPC(sockPath string) error {
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("runtime: listen %s: %w", sockPath, err)
	}
	// Restrict socket to owner only — the client controls the pane backend
	// lifecycle, so unauthenticated local access = arbitrary command
	// execution.
	if err := os.Chmod(sockPath, 0o600); err != nil {
		_ = ln.Close()
		return fmt.Errorf("runtime: chmod %s: %w", sockPath, err)
	}
	r.listener = ln
	slog.Info("runtime: ipc listening", "sock", sockPath)
	go r.acceptLoop()
	return nil
}

func (r *Runtime) acceptLoop() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.done:
				return
			default:
				if errors.Is(err, net.ErrClosed) {
					return
				}
				slog.Error("runtime: accept failed", "err", err)
				continue
			}
		}
		if err := checkPeerCred(conn); err != nil {
			slog.Warn("runtime: rejecting connection", "err", err)
			_ = conn.Close()
			continue
		}
		_ = r.enqueueInternal(connOpen{conn: conn})
	}
}

// internalEvent is the closed sum of runtime-internal lifecycle
// events that bypass state.Reduce. Used for connection accept /
// teardown, where the runtime owns mutable state (the conns map)
// the reducer can't see.
type internalEvent interface {
	isInternalEvent()
}

// connOpen is enqueued by the accept loop after Accept returns.
type connOpen struct {
	conn net.Conn
}

func (connOpen) isInternalEvent() {}

// connClose is enqueued by connReader after the socket EOFs.
type connClose struct {
	id state.ConnID
}

func (connClose) isInternalEvent() {}

// internalSetRelay is enqueued by SetRelay to wire a FileRelay into the loop.
type internalSetRelay struct {
	relay *FileRelay
}

func (internalSetRelay) isInternalEvent() {}

// internalBroadcastWire is enqueued by FileRelay so broadcastWire runs
// on the event loop, never on the sweep goroutine (which would race
// the loop's conns / Subscribers maps).
type internalBroadcastWire struct {
	wire      []byte
	eventName string
}

func (internalBroadcastWire) isInternalEvent() {}

// internalStartRestoredTaps is enqueued by StartTapsForRestoredFrames to
// attach pane taps to panes that were restored from the snapshot (warm
// or cold start) — bypasses the reducer because Reduce is only invoked
// by user-driven events, and restored panes never go through
// EvFrameSpawned.
type internalStartRestoredTaps struct{}

func (internalStartRestoredTaps) isInternalEvent() {}

// internalSpawnComplete is enqueued by the spawn goroutine after a window has
// been launched. The goroutine performs all slow I/O and carries the resulting
// per-frame handles back as data; the event loop is the sole writer that stores
// them into loop-owned maps (handleSpawnComplete), keeping spawn off the
// single-writer state without any direct map writes from the goroutine.
type internalSpawnComplete struct {
	effect           state.EffSpawnFrame
	subsystemID      state.SubsystemID
	sub              rsubsystem.Subsystem
	cleanup          func() error
	token            string         // empty for non-container frames
	mounts           pathmap.Mounts // nil for non-container frames
	containerSockDir string         // raw ContainerSockDir from WrappedLaunch; empty for non-container
	bindResult       rsubsystem.BindResult
}

func (internalSpawnComplete) isInternalEvent() {}

// dispatchInternal handles runtime-internal events.
func (r *Runtime) dispatchInternal(ev internalEvent) {
	switch e := ev.(type) {
	case connOpen:
		r.handleConnOpen(e.conn)
	case connClose:
		r.handleConnClose(e.id)
	case internalBroadcastWire:
		r.broadcastWire(e.wire, e.eventName)
	case internalSetRelay:
		r.relay = e.relay
		r.syncRelayWatches()
	case internalStartRestoredTaps:
		_ = e
		r.startRestoredTaps()
	case internalSpawnComplete:
		r.handleSpawnComplete(e)
	case internalBroadcastSurface:
		r.broadcastSurfaceFromInternal(e)
	case internalSurfaceClosed:
		if r.terminalRelay != nil {
			r.terminalRelay.Unsubscribe(e.ConnID, e.SessionID)
		}
		r.dispatch(state.EvCmdSurfaceUnsubscribe{ConnID: e.ConnID, ReqID: "", SessionID: e.SessionID})
	}
}

// startRestoredTaps attaches a pane tap to each restored root frame.
// Non-root frames don't get taps because their driver state isn't
// displayed in the UI. Called from the event loop so r.taps is
// guaranteed to be initialised and state.Sessions is accessed under
// the loop's single-writer discipline.
func (r *Runtime) startRestoredTaps() {
	if r.taps == nil {
		return
	}
	for _, sess := range r.state.Sessions {
		if len(sess.Frames) == 0 {
			continue
		}
		frameID := sess.Frames[0].ID
		r.taps.start(frameID, string(frameID), r.Enqueue)
	}
}

// enqueueInternal posts an internal event onto the runtime's
// internal channel. Non-blocking; drops silently on a full channel.
// Returns true when the event was accepted by the channel, false on drop.
//
// Drops are logged at Debug (not Warn) because the daemon's own log file is
// streamed back to clients via FileRelay. Every Warn would be written to
// server.log, the FileRelay watcher would observe the write, the sweep would
// enqueue an internalBroadcastWire, and if internalCh is already saturated
// the enqueue would drop and emit ANOTHER Warn — a self-sustaining feedback
// loop that wedges the daemon (observed: 39MB of identical lines at 10/s).
// Debug-level drop messages stay out of the default log stream and break the
// cycle. Callers that genuinely cannot tolerate a drop (e.g. spawn completion)
// use sendSpawnComplete, which blocks rather than dropping. Callers that can
// recover from a drop by retrying (e.g. FileRelay.sweep) check the return
// value and roll back state to retry on the next tick.
//
// Every drop is also counted per-event-type via internalDrops so saturation
// causes can be attributed via InternalDropStats.
func (r *Runtime) enqueueInternal(ev internalEvent) bool {
	select {
	case r.internalCh <- ev:
		return true
	default:
		name := internalEventName(ev)
		if r.internalDrops != nil {
			r.internalDrops.inc(name)
		}
		slog.Debug("runtime: internal channel full, dropping", "type", name)
		return false
	}
}

// Internal event name constants. Shared by internalEventName (for log/metrics
// labels) and newInternalDropCounter (for the pre-populated counter map).
// Keeping them as const makes the catalogue greppable.
const (
	internalEventConnOpen          = "conn-open"
	internalEventConnClose         = "conn-close"
	internalEventBroadcastWire     = "broadcast-wire"
	internalEventBroadcastSurface  = "broadcast-surface"
	internalEventSurfaceClosed     = "surface-closed"
	internalEventSetRelay          = "set-relay"
	internalEventStartRestoredTaps = "start-restored-taps"
	internalEventSpawnComplete     = "spawn-complete"
	internalEventUnknown           = "unknown"
)

// internalEventName returns a short identifier for an internal event, used
// for diagnostic logs (Debug level) and drop counter labels.
func internalEventName(ev internalEvent) string {
	switch ev.(type) {
	case connOpen:
		return internalEventConnOpen
	case connClose:
		return internalEventConnClose
	case internalBroadcastWire:
		return internalEventBroadcastWire
	case internalBroadcastSurface:
		return internalEventBroadcastSurface
	case internalSurfaceClosed:
		return internalEventSurfaceClosed
	case internalSetRelay:
		return internalEventSetRelay
	case internalStartRestoredTaps:
		return internalEventStartRestoredTaps
	case internalSpawnComplete:
		return internalEventSpawnComplete
	default:
		return internalEventUnknown
	}
}

// internalDropCounter tracks per-event-type silent drops from enqueueInternal.
// Pre-populated at construction so the hot path uses an existing *atomic.Uint64
// (no map writes, no locks). Unknown event types fall back to the "unknown"
// bucket.
type internalDropCounter struct {
	byType map[string]*atomic.Uint64
}

func newInternalDropCounter() *internalDropCounter {
	names := []string{
		internalEventConnOpen,
		internalEventConnClose,
		internalEventBroadcastWire,
		internalEventBroadcastSurface,
		internalEventSurfaceClosed,
		internalEventSetRelay,
		internalEventStartRestoredTaps,
		internalEventSpawnComplete,
		internalEventUnknown,
	}
	m := make(map[string]*atomic.Uint64, len(names))
	for _, n := range names {
		m[n] = new(atomic.Uint64)
	}
	return &internalDropCounter{byType: m}
}

func (c *internalDropCounter) inc(name string) {
	if v, ok := c.byType[name]; ok {
		v.Add(1)
		return
	}
	c.byType[internalEventUnknown].Add(1)
}

// snapshot returns the current per-event-type drop counts. Only non-zero
// buckets are included so the caller can spot active producers quickly.
func (c *internalDropCounter) snapshot() map[string]uint64 {
	out := make(map[string]uint64, len(c.byType))
	for k, v := range c.byType {
		if n := v.Load(); n > 0 {
			out[k] = n
		}
	}
	return out
}

// sendSpawnComplete delivers a spawn-completion event to the loop. Unlike
// enqueueInternal it must NOT drop: handleSpawnComplete is the sole writer of
// the subsystem/cleanup maps and container registry for the frame, so losing
// this event would leak the already-launched subsystem, pane, container
// token and cleanup closure with no recovery path. Blocks until the loop
// accepts it or the daemon shuts down (r.done), so it never leaks a goroutine.
func (r *Runtime) sendSpawnComplete(ev internalEvent) {
	select {
	case r.internalCh <- ev:
	case <-r.done:
	}
}

func (r *Runtime) handleConnOpen(conn net.Conn) {
	r.nextConn++
	id := r.nextConn
	cc := newIPCConn(id, conn)
	r.conns[id] = cc
	go r.connWriter(cc)
	go r.connReader(cc)
	r.dispatch(state.EvConnOpened{ConnID: id})
}

// connReader decodes wire envelopes, translates Commands into
// state.Events, and enqueues them on the runtime event loop. On EOF
// or error, it enqueues EvConnClosed and exits.
func (r *Runtime) connReader(cc *ipcConn) {
	defer func() {
		_ = r.enqueueInternal(connClose{id: cc.id})
	}()
	dec := json.NewDecoder(cc.conn)
	for {
		select {
		case <-cc.done:
			return
		default:
		}
		var env proto.Envelope
		if err := dec.Decode(&env); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				slog.Debug("runtime: conn decode error", "conn", cc.id, "err", err)
			}
			return
		}
		if env.Type != proto.TypeCommand {
			continue
		}
		cmd, err := proto.DecodeCommand(env)
		if err != nil {
			slog.Warn("runtime: bad command", "conn", cc.id, "err", err)
			r.sendErrorImmediate(cc, env.ReqID, proto.ErrInvalidArgument, err.Error())
			continue
		}
		ev := commandToStateEvent(cc.id, env.ReqID, cmd)
		if ev == nil {
			r.sendErrorImmediate(cc, env.ReqID, proto.ErrUnsupported, "unknown command")
			continue
		}
		r.Enqueue(ev)
	}
}

// connWriter drains the outbox and writes wire bytes to the socket.
func (r *Runtime) connWriter(cc *ipcConn) {
	for {
		select {
		case <-cc.done:
			return
		case payload := <-cc.outbox:
			if err := r.writeWire(cc, payload); err != nil {
				return
			}
		}
	}
}

func (r *Runtime) handleConnClose(id state.ConnID) {
	if cc, ok := r.conns[id]; ok {
		cc.shut()
		delete(r.conns, id)
	}
	r.dispatch(state.EvConnClosed{ConnID: id})
}

// sendErrorImmediate writes an error response on a connection
// without going through the reducer (used for malformed envelopes
// caught in connReader, before the event loop ever sees them).
func (r *Runtime) sendErrorImmediate(cc *ipcConn, reqID string, code proto.ErrCode, msg string) {
	wire, err := proto.EncodeError(reqID, code, msg, nil)
	if err != nil {
		return
	}
	r.queueWire(cc, wire)
}

// queueWire enqueues raw wire bytes on a conn's outbox. Non-blocking;
// drops with a warning if the outbox is full.
func (r *Runtime) queueWire(cc *ipcConn, wire []byte) {
	select {
	case cc.outbox <- wire:
	case <-cc.done:
	default:
		slog.Warn("runtime: conn outbox full, dropping", "conn", cc.id)
	}
}

func (r *Runtime) writeWire(cc *ipcConn, wire []byte) error {
	cc.writeMu.Lock()
	defer cc.writeMu.Unlock()

	select {
	case <-cc.done:
		return net.ErrClosed
	default:
	}

	w := bufio.NewWriter(cc.conn)
	if _, err := w.Write(wire); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}

// shutdownIPC closes the listener and every active connection. Called
// from Run on shutdown.
func (r *Runtime) shutdownIPC() {
	if r.listener != nil {
		_ = r.listener.Close()
	}
	for id, cc := range r.conns {
		cc.shut()
		delete(r.conns, id)
	}
	r.shutdownContainerEndpoints()
}

// === Loop integration ===
