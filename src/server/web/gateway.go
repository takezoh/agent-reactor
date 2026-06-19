package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

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
	WriteRaw(ctx context.Context, sessionID string, data []byte) error
	Resize(ctx context.Context, sessionID string, cols, rows uint16) error
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

// writeTypedClose sends a WebSocket typed close frame.
func writeTypedClose(c *websocket.Conn, status websocket.StatusCode, reason string) {
	_ = c.Close(status, reason)
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
		writeTypedClose(c, websocket.StatusGoingAway, "subscribe-failed")
		return err
	}
	defer func() { _ = sess.UnsubscribeSurface(context.Background(), sessionID) }()

	go func() { readInbound(ctx, sess, sessionID, c); cancel() }()
	return writeOutbound(ctx, sessionID, c, ch)
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
				writeTypedClose(c, websocket.StatusGoingAway, "daemon-disconnected")
				return errDaemonGone
			}
			// Filter: only forward events belonging to this session.
			if out, ok2 := ev.(proto.EvtSurfaceOutput); ok2 && out.SessionID != sessionID {
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
