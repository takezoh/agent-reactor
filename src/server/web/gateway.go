package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

// outputFrame encodes an asciicast v2 output event: [time, "o", data].
// Kept here (alongside encodeEvent) until gateway.go is migrated to proto events.
func outputFrame(t float64, data []byte) []byte {
	b, _ := json.Marshal([]any{t, "o", string(data)})
	return b
}

func controlFrame(kind string, code int, data string) []byte {
	b, _ := json.Marshal(controlMsg{K: kind, Code: code, Data: data})
	return b
}

// encodeEvent renders a termvt.Event as a single WebSocket text frame.
// Deprecated: will be replaced by encodeServerEvent once gateway.go is migrated.
func encodeEvent(elapsed float64, ev termvt.Event) []byte {
	switch ev.Kind {
	case termvt.EventOutput:
		return outputFrame(elapsed, ev.Data)
	case termvt.EventControl:
		return controlFrame(ev.Ctl.Kind, ev.Ctl.Code, ev.Ctl.Data)
	case termvt.EventExit:
		return controlFrame("exit", 0, "")
	default:
		return nil
	}
}

// Attacher is the subset of *termvt.Session the gateway needs. Declared as an
// interface so the bridge can be tested with a fake.
type Attacher interface {
	Subscribe() (int, <-chan termvt.Event)
	Unsubscribe(id int)
	WriteInput(b []byte) error
	Resize(cols, rows int) error
}

// AttachWS bridges one WebSocket connection to a session: it streams output and
// control events to the client (writer loop) and forwards client input/resize
// (reader goroutine). It returns when the connection or session closes.
func AttachWS(ctx context.Context, sess Attacher, c *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	id, ch := sess.Subscribe()
	defer sess.Unsubscribe(id)

	start := time.Now()
	go func() { readInbound(ctx, sess, c); cancel() }()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			frame := encodeEvent(time.Since(start).Seconds(), ev)
			if frame == nil {
				continue
			}
			if err := c.Write(ctx, websocket.MessageText, frame); err != nil {
				return err
			}
		}
	}
}

// readInbound forwards client messages (input, resize) to the session until the
// connection closes.
func readInbound(ctx context.Context, sess Attacher, c *websocket.Conn) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		applyInbound(sess, data)
	}
}

// applyInbound decodes data and applies it to sess: "i" writes input, "r"
// resizes — but only with positive dimensions (the absolute upper bound that
// keeps the pty/VT grid safe is enforced downstream by termvt.normalizeSize).
// Malformed JSON and unknown kinds are ignored. Returns true if the frame
// produced an action. Split out from readInbound so the untrusted-input decode
// path is unit- and fuzz-testable (FuzzInbound).
func applyInbound(sess Attacher, data []byte) bool {
	var in inbound
	if json.Unmarshal(data, &in) != nil {
		return false
	}
	switch in.K {
	case "i":
		// Best-effort: a failed write (closed pty) is logged but does not change
		// the inbound-frame disposition — the session teardown path handles the
		// closed pty, and the WebSocket reader will see its own error next read.
		if err := sess.WriteInput([]byte(in.D)); err != nil {
			slog.Warn("web: write input to session", "err", err)
		}
		return true
	case "r":
		if in.Cols > 0 && in.Rows > 0 {
			_ = sess.Resize(in.Cols, in.Rows)
			return true
		}
	}
	return false
}
