package stream

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestDialWebSocketUDS verifies the daemon performs the HTTP Upgrade
// handshake when connecting to a unix-socket WebSocket server.
func TestDialWebSocketUDS(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer c.Close(websocket.StatusNormalClosure, "")
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			_ = c.Write(r.Context(), websocket.MessageText, data)
		}),
	}
	go srv.Serve(ln) //nolint:errcheck

	conn, err := dialWebSocketUDS(sockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("dialWebSocketUDS: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"id":1}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	mt, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.MessageText {
		t.Fatalf("message type = %v, want text", mt)
	}
	if string(data) != `{"id":1}` {
		t.Fatalf("echo = %q, want %q", data, `{"id":1}`)
	}
}

func TestDialWebSocketUDS_timeout(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "missing.sock")
	start := time.Now()
	_, err := dialWebSocketUDS(sockPath, 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("dial took %v, want < 2s", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) && !isUnixDialErr(err) {
		t.Logf("err: %v", err)
	}
}

func isUnixDialErr(err error) bool {
	var ne *net.OpError
	return errors.As(err, &ne) && ne.Op == "dial"
}
