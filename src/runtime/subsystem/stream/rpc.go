package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

type rpcMessage struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

// initialize performs the JSON-RPC handshake with the codex app-server:
// `initialize` request followed by an `initialized` notification.
func (b *Backend) initialize() error {
	if _, err := b.request("initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "roost", "version": "0"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		return err
	}
	return b.notify("initialized", map[string]any{})
}

func (b *Backend) resumeThread(threadID, startDir string) (json.RawMessage, error) {
	params := map[string]any{"threadId": threadID}
	if startDir != "" {
		params["cwd"] = startDir
	}
	msg, err := b.request("thread/resume", params)
	if err != nil {
		return nil, err
	}
	return msg.Result, nil
}

func (b *Backend) startTurn(threadID, startDir string, stdin []byte) error {
	params := map[string]any{}
	if threadID != "" {
		params["threadId"] = threadID
	}
	if startDir != "" {
		params["cwd"] = startDir
	}
	if len(stdin) > 0 {
		params["message"] = string(stdin)
	}
	return b.notify("turn/start", params)
}

func (b *Backend) runReadLoop() error {
	ctx := context.Background()
	for {
		_, data, err := b.wsConn.Read(ctx)
		if err != nil {
			return err
		}
		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.ID != nil && msg.Method == "" {
			b.resolvePending(*msg.ID, msg)
			continue
		}
		if msg.Method == "" {
			continue
		}
		if msg.ID != nil {
			b.handleRequest(msg)
			continue
		}
		b.handleNotification(msg)
	}
}

func (b *Backend) request(method string, params map[string]any) (rpcMessage, error) {
	id := atomic.AddInt64(&b.nextID, 1)
	ch := make(chan rpcMessage, 1)
	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()
	if err := b.writeRPC(rpcMessage{ID: &id, Method: method, Params: mustJSON(params)}); err != nil {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return rpcMessage{}, err
	}
	select {
	case msg := <-ch:
		if len(msg.Error) > 0 && string(msg.Error) != "null" {
			return msg, fmt.Errorf("stream backend: %s error: %s", method, msg.Error)
		}
		return msg, nil
	case <-time.After(15 * time.Second):
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return rpcMessage{}, fmt.Errorf("stream backend: timeout waiting for %s", method)
	}
}

func (b *Backend) notify(method string, params map[string]any) error {
	return b.writeRPC(rpcMessage{Method: method, Params: mustJSON(params)})
}

func (b *Backend) reply(id int64, result any) error {
	return b.writeRPC(rpcMessage{ID: &id, Result: mustJSON(result)})
}

func (b *Backend) replyError(id int64, errMsg string) error {
	return b.writeRPC(rpcMessage{ID: &id, Error: mustJSON(map[string]any{"message": errMsg})})
}

func (b *Backend) writeRPC(msg rpcMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.wsConn.Write(context.Background(), websocket.MessageText, data)
}

func (b *Backend) resolvePending(id int64, msg rpcMessage) {
	b.mu.Lock()
	ch := b.pending[id]
	delete(b.pending, id)
	b.mu.Unlock()
	if ch != nil {
		ch <- msg
	}
}

// dialWebSocketUDS opens a WebSocket connection to codex app-server over a
// unix domain socket. Retries the underlying unix dial every 50 ms until the
// socket file appears or the timeout elapses.
func dialWebSocketUDS(sockPath string, timeout time.Duration) (*websocket.Conn, error) {
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
		return nil, fmt.Errorf("stream backend: websocket upgrade %s: %w", sockPath, err)
	}
	// codex frames can exceed default read limit (32 KB). Disable the cap.
	conn.SetReadLimit(-1)
	return conn, nil
}
