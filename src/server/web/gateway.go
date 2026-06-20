package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/proto"
)

// errDaemonGone is returned by writeOutbound when the daemon events channel
// closes, indicating the daemon disconnected.
var errDaemonGone = errors.New("server/web: daemon disconnected")

// Attacher is the daemon-side surface AttachWS needs (proto.Client wrapper).
// Implemented by DaemonAdapter; a fake is used in gateway_terminal_test.go.
type Attacher interface {
	// SubscribeSurface starts streaming output for sessionID on this WS.
	// Returns immediately after the daemon ack and yields a chan of
	// proto.ServerEvent filtered to this session (best-effort: events
	// for other sessions may also come through if the daemon-side filter
	// is coarse — the gateway re-filters).
	SubscribeSurface(ctx context.Context, sessionID string) (<-chan proto.ServerEvent, error)
	UnsubscribeSurface(ctx context.Context, sessionID string) error
	// SendSurfaceSubscribe forwards a CmdSurfaceSubscribe to the daemon
	// WITHOUT registering a new event subscriber on the DaemonClient.
	// Used by AttachLifecycleWS, which already holds a single event channel
	// shared by every multiplexed session.
	SendSurfaceSubscribe(ctx context.Context, sessionID string) error
	WriteRaw(ctx context.Context, sessionID string, data []byte) error
	Resize(ctx context.Context, sessionID string, cols, rows uint16) error
	// SubscribeLifecycle subscribes to daemon-side lifecycle events
	// (sessions-changed) and returns a channel of ServerEvent.
	// The returned channel is closed on disconnect.
	SubscribeLifecycle(ctx context.Context) (<-chan proto.ServerEvent, error)
}

// DaemonAdapter implements Attacher on top of DaemonClient.
type DaemonAdapter struct {
	d *DaemonClient
}

// NewDaemonAdapter wraps a DaemonClient as an Attacher.
func NewDaemonAdapter(d *DaemonClient) *DaemonAdapter { return &DaemonAdapter{d: d} }

// SubscribeSurface sends CmdSurfaceSubscribe and returns the shared events channel.
func (a *DaemonAdapter) SubscribeSurface(ctx context.Context, sid string) (<-chan proto.ServerEvent, error) {
	if _, err := a.d.SendCommand(ctx, proto.CmdSurfaceSubscribe{SessionID: sid}); err != nil {
		return nil, err
	}
	return a.d.SubscribeEvents(), nil
}

// UnsubscribeSurface sends CmdSurfaceUnsubscribe to the daemon.
func (a *DaemonAdapter) UnsubscribeSurface(ctx context.Context, sid string) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceUnsubscribe{SessionID: sid})
	return err
}

// SendSurfaceSubscribe forwards CmdSurfaceSubscribe to the daemon without
// allocating a fresh DaemonClient subscriber. Used by AttachLifecycleWS,
// which multiplexes subscribe requests over its single lifecycle event channel.
func (a *DaemonAdapter) SendSurfaceSubscribe(ctx context.Context, sid string) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceSubscribe{SessionID: sid})
	return err
}

// WriteRaw sends CmdSurfaceWriteRaw to the daemon.
func (a *DaemonAdapter) WriteRaw(ctx context.Context, sid string, data []byte) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceWriteRaw{SessionID: sid, Data: data})
	return err
}

// Resize sends CmdSurfaceResize to the daemon.
func (a *DaemonAdapter) Resize(ctx context.Context, sid string, cols, rows uint16) error {
	_, err := a.d.SendCommand(ctx, proto.CmdSurfaceResize{SessionID: sid, Cols: cols, Rows: rows})
	return err
}

// SubscribeLifecycle sends CmdSubscribe for sessions-changed, session-file-line,
// and agent-notification events, and returns the shared events channel.
func (a *DaemonAdapter) SubscribeLifecycle(ctx context.Context) (<-chan proto.ServerEvent, error) {
	filters := []string{
		proto.EvtNameSessionsChanged,
		proto.EvtNameSessionFileLine,
		proto.EvtNameAgentNotification,
	}
	if _, err := a.d.SendCommand(ctx, proto.CmdSubscribe{Filters: filters}); err != nil {
		return nil, err
	}
	return a.d.SubscribeEvents(), nil
}

// writeTypedClose sends a WebSocket StatusGoingAway typed close frame.
func writeTypedClose(c *websocket.Conn, reason string) {
	_ = c.Close(websocket.StatusGoingAway, reason)
}

// AttachWS bridges one WebSocket connection to a session surface. It streams
// output events to the client (writeOutbound) and forwards client input/resize
// (readInbound goroutine). Returns when the connection or daemon closes.
func AttachWS(ctx context.Context, sess Attacher, sessionID string, c *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch, err := sess.SubscribeSurface(ctx, sessionID)
	if err != nil {
		var eb *proto.ErrorBody
		if errors.As(err, &eb) {
			_ = c.Write(ctx, websocket.MessageText, controlFrame("c", 0, string(eb.Code)))
		}
		writeTypedClose(c, "subscribe-failed")
		return err
	}
	defer func() { _ = sess.UnsubscribeSurface(context.Background(), sessionID) }()

	go func() { readInbound(ctx, sess, sessionID, c); cancel() }()
	return writeOutbound(ctx, sessionID, c, ch)
}

// helloFrame is the first server→browser frame for a lifecycle WebSocket.
// It seeds the browser with the current sessions / activeSessionID / features
// / connectors so the React store can render the initial view before any
// subsequent view-update arrives.
type helloFrame struct {
	K               string                `json:"k"` // always "h"
	Sessions        []proto.SessionInfo   `json:"sessions"`
	ActiveSessionID string                `json:"activeSessionID,omitempty"`
	Features        []string              `json:"features"`
	Connectors      []proto.ConnectorInfo `json:"connectors,omitempty"`
	ServerTime      int64                 `json:"serverTime"`
}

// encodeHelloFrame encodes EvtSessionsChanged as the initial hello frame.
// nil slices are replaced with empty slices so the browser always gets arrays.
func encodeHelloFrame(sc proto.EvtSessionsChanged, serverTime int64) []byte {
	sessions := sc.Sessions
	if sessions == nil {
		sessions = []proto.SessionInfo{}
	}
	features := sc.Features
	if features == nil {
		features = []string{}
	}
	h := helloFrame{
		K:               "h",
		Sessions:        sessions,
		ActiveSessionID: sc.ActiveSessionID,
		Features:        features,
		Connectors:      sc.Connectors, // omitempty: nil/empty stays out of wire
		ServerTime:      serverTime,
	}
	b, err := json.Marshal(h)
	if err != nil {
		slog.Error("server/web: encode hello failed", "err", err)
		return nil
	}
	return b
}

// lifecycleSubSet tracks the set of session IDs a single AttachLifecycleWS
// connection has subscribed to. It is used both to filter EvtSurfaceOutput
// (only forward events for currently subscribed sessions) and to issue
// CmdSurfaceUnsubscribe for each on connection teardown.
type lifecycleSubSet struct {
	mu  sync.Mutex
	ids map[string]struct{}
}

func newLifecycleSubSet() *lifecycleSubSet {
	return &lifecycleSubSet{ids: make(map[string]struct{})}
}

func (s *lifecycleSubSet) add(id string)    { s.mu.Lock(); s.ids[id] = struct{}{}; s.mu.Unlock() }
func (s *lifecycleSubSet) remove(id string) { s.mu.Lock(); delete(s.ids, id); s.mu.Unlock() }
func (s *lifecycleSubSet) contains(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.ids[id]
	return ok
}
func (s *lifecycleSubSet) drain() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.ids))
	for id := range s.ids {
		out = append(out, id)
	}
	s.ids = make(map[string]struct{})
	return out
}

// AttachLifecycleWS bridges one WebSocket connection to daemon lifecycle
// events. Used when the client connects without a ?session= query param.
// The single WebSocket is multiplexed: it carries lifecycle frames
// (k:"h" hello, k:"v" view-update, k:"tt" / k:"et" tail, k:"n" notification)
// and per-session surface frames (asciicast output array) for any session the
// browser has subscribed to via inbound k:"s" frames. Inbound k:"u" frames
// unsubscribe. k:"i"/"r" frames forward input and resize for a specific
// sessionId. Sends a 2-step close (ADR 0011) on daemon disconnect.
//
// The read goroutine is required not only to dispatch inbound frames but to
// drain the coder/websocket control frames (pings) — without it, the browser
// keep-alive times out and forcibly closes the connection.
func AttachLifecycleWS(ctx context.Context, sess Attacher, c *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch, err := sess.SubscribeLifecycle(ctx)
	if err != nil {
		writeTypedClose(c, "lifecycle-subscribe-failed")
		return err
	}
	subs := newLifecycleSubSet()
	defer func() {
		// Cleanup: unsubscribe daemon-side for every session this WS held open.
		for _, id := range subs.drain() {
			_ = sess.UnsubscribeSurface(context.Background(), id)
		}
	}()

	go func() { readLifecycleInbound(ctx, sess, c, subs); cancel() }()

	helloSent := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				_ = c.Write(ctx, websocket.MessageText, controlFrame("c", 0, "daemon-disconnected"))
				writeTypedClose(c, "daemon-disconnected")
				return errDaemonGone
			}
			var frame []byte
			switch e := ev.(type) {
			case proto.EvtSessionsChanged:
				if !helloSent {
					frame = encodeHelloFrame(e, time.Now().Unix())
					helloSent = true
				} else {
					frame = encodeServerEvent(e)
				}
			case proto.EvtSessionFileLine, proto.EvtAgentNotification:
				frame = encodeServerEvent(e)
			case proto.EvtSurfaceOutput:
				// Multiplexed surface output: only forward if this WS has
				// actively subscribed to the producing session.
				if !subs.contains(e.SessionID) {
					continue
				}
				frame = encodeServerEvent(e)
			default:
				continue
			}
			if frame == nil {
				continue
			}
			if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
				return err
			}
		}
	}
}

// readLifecycleInbound drains the WebSocket read path, processing inbound
// {k:"s"} subscribe / {k:"u"} unsubscribe / {k:"i"} input / {k:"r"} resize
// frames. Reading is also necessary so coder/websocket can autorespond to
// ping control frames. On read error (browser close, transport failure) the
// goroutine returns and the caller's cancel() tears down the parent ctx.
func readLifecycleInbound(ctx context.Context, sess Attacher, c *websocket.Conn, subs *lifecycleSubSet) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var msg inbound
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.K {
		case "s":
			if msg.SessionID == "" {
				continue
			}
			if err := sess.SendSurfaceSubscribe(ctx, msg.SessionID); err != nil {
				slog.Warn("server/web: lifecycle surface subscribe", "err", err, "sid", msg.SessionID)
				continue
			}
			subs.add(msg.SessionID)
		case "u":
			if msg.SessionID == "" {
				continue
			}
			subs.remove(msg.SessionID)
			if err := sess.UnsubscribeSurface(ctx, msg.SessionID); err != nil {
				slog.Warn("server/web: lifecycle surface unsubscribe", "err", err, "sid", msg.SessionID)
			}
		case "i":
			if msg.SessionID == "" {
				continue
			}
			if err := sess.WriteRaw(ctx, msg.SessionID, []byte(msg.D)); err != nil {
				slog.Warn("server/web: lifecycle write raw", "err", err, "sid", msg.SessionID)
			}
		case "r":
			if msg.SessionID == "" || msg.Cols <= 0 || msg.Rows <= 0 {
				continue
			}
			if err := sess.Resize(ctx, msg.SessionID, uint16(msg.Cols), uint16(msg.Rows)); err != nil {
				slog.Warn("server/web: lifecycle resize", "err", err, "sid", msg.SessionID)
			}
		}
	}
}

// writeOutbound reads proto.ServerEvent values from ch and encodes them as WS
// frames. On daemon disconnect (ch closed) it sends the 2-step close defined
// in ADR 0011: control frame then StatusGoingAway typed close.
func writeOutbound(ctx context.Context, sessionID string, c *websocket.Conn, ch <-chan proto.ServerEvent) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				// Daemon disconnected: 2-step close (ADR 0011).
				_ = c.Write(ctx, websocket.MessageText, controlFrame("c", 0, "daemon-disconnected"))
				writeTypedClose(c, "daemon-disconnected")
				return errDaemonGone
			}
			// Filter: only forward surface-scoped events belonging to this
			// session. Any other event type (notably EvtSessionsChanged, which
			// is part of the lifecycle stream) must be dropped here so that
			// terminal-only WS clients do not receive lifecycle traffic.
			switch e := ev.(type) {
			case proto.EvtSurfaceOutput:
				if e.SessionID != sessionID {
					continue
				}
			case proto.EvtSessionFileLine:
				if e.SessionID != sessionID {
					continue
				}
			case proto.EvtAgentNotification:
				if e.SessionID != sessionID {
					continue
				}
			default:
				// Not a surface-scoped event (e.g. EvtSessionsChanged). The
				// AttachWS path is the per-session terminal stream; everything
				// else is owned by AttachLifecycleWS.
				continue
			}
			frame := encodeServerEvent(ev)
			if frame == nil {
				continue
			}
			if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
				return err
			}
		}
	}
}

// readInbound forwards client messages (input, resize) to the session until
// the connection or context closes. Errors are logged at warn level and cause
// the function to return; the caller goroutine then invokes cancel().
func readInbound(ctx context.Context, sess Attacher, sessionID string, c *websocket.Conn) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		applyInboundProto(ctx, sess, sessionID, data)
	}
}

// applyInboundProto decodes a raw browser frame and dispatches to sess.
// "i" → WriteRaw; "r" (positive cols+rows) → Resize. Unknown kinds are
// silently dropped.
func applyInboundProto(ctx context.Context, sess Attacher, sessionID string, data []byte) {
	var in inbound
	if json.Unmarshal(data, &in) != nil {
		return
	}
	switch in.K {
	case "i":
		if err := sess.WriteRaw(ctx, sessionID, []byte(in.D)); err != nil {
			slog.Warn("server/web: write raw to session", "err", err)
		}
	case "r":
		if in.Cols > 0 && in.Rows > 0 {
			if err := sess.Resize(ctx, sessionID, uint16(in.Cols), uint16(in.Rows)); err != nil {
				slog.Warn("server/web: resize session", "err", err)
			}
		}
	}
}
