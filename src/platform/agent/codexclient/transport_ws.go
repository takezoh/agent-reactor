package codexclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

type wsTransport struct {
	conn    *websocket.Conn
	writeMu chan struct{} // token-based write lock
}

// DialUDS opens a WebSocket connection to a Codex app-server over a unix domain
// socket, retrying the underlying dial every 50 ms until timeout elapses.
func DialUDS(sockPath string, timeout time.Duration) (Transport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				for {
					c, err := net.Dial("unix", sockPath)
					if err == nil {
						return c, nil
					}
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(50 * time.Millisecond):
					}
				}
			},
		},
	}
	conn, _, err := websocket.Dial(ctx, "ws://localhost/", &websocket.DialOptions{
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("codexclient: websocket upgrade %s: %w", sockPath, err)
	}
	// Codex frames can exceed the default 32 KB read limit.
	conn.SetReadLimit(-1)
	t := &wsTransport{
		conn:    conn,
		writeMu: make(chan struct{}, 1),
	}
	t.writeMu <- struct{}{} // pre-fill to indicate unlocked
	return t, nil
}

func (t *wsTransport) ReadMessage(ctx context.Context) ([]byte, error) {
	_, data, err := t.conn.Read(ctx)
	return data, err
}

func (t *wsTransport) WriteMessage(ctx context.Context, data []byte) error {
	<-t.writeMu
	defer func() { t.writeMu <- struct{}{} }()
	return t.conn.Write(ctx, websocket.MessageText, data)
}

func (t *wsTransport) Close() error {
	return t.conn.Close(websocket.StatusNormalClosure, "")
}
