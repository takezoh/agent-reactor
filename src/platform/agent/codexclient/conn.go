package codexclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// rpcMessage is the JSON-RPC 2.0 envelope used by the Codex app-server protocol.
type rpcMessage struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

// Handler receives inbound messages dispatched by Conn.Run.
type Handler interface {
	// OnNotification is called for server-initiated notifications (no reply expected).
	OnNotification(method string, params json.RawMessage)
	// OnServerRequest is called for server-initiated requests.  The handler must
	// call conn.Reply or conn.ReplyError before returning to unblock the peer.
	OnServerRequest(id int64, method string, params json.RawMessage)
}

// Conn is a transport-agnostic JSON-RPC framing layer for the Codex app-server
// protocol.  It can be used as either the initiating side (client role) or the
// responding side (server role); both directions use the same framing.
type Conn struct {
	t           Transport
	readTimeout time.Duration
	mu          sync.Mutex
	pending     map[int64]chan rpcMessage
	nextID      int64
}

// NewConn wraps t in a Conn.  readTimeout is applied to each client-initiated
// request; zero means 15 seconds (the historic default).
func NewConn(t Transport, readTimeout time.Duration) *Conn {
	if readTimeout <= 0 {
		readTimeout = 15 * time.Second
	}
	return &Conn{
		t:           t,
		readTimeout: readTimeout,
		pending:     make(map[int64]chan rpcMessage),
	}
}

// Run starts the read loop and dispatches messages to h until the transport
// returns an error.  It blocks until the loop exits.
func (c *Conn) Run(ctx context.Context, h Handler) error {
	for {
		data, err := c.t.ReadMessage(ctx)
		if err != nil {
			return err
		}
		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		// Response to a pending client request.
		if msg.ID != nil && msg.Method == "" {
			c.resolvePending(*msg.ID, msg)
			continue
		}
		if msg.Method == "" {
			continue
		}
		if msg.ID != nil {
			h.OnServerRequest(*msg.ID, msg.Method, msg.Params)
		} else {
			h.OnNotification(msg.Method, msg.Params)
		}
	}
}

// Request sends a request and waits for the corresponding response.
func (c *Conn) Request(method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan rpcMessage, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.writeMsg(rpcMessage{ID: &id, Method: method, Params: mustJSON(params)}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	select {
	case msg := <-ch:
		if len(msg.Error) > 0 && string(msg.Error) != "null" {
			return msg.Result, fmt.Errorf("codexclient: %s error: %s", method, msg.Error)
		}
		return msg.Result, nil
	case <-time.After(c.readTimeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("codexclient: timeout waiting for %s", method)
	}
}

// Notify sends a notification (no response expected).
func (c *Conn) Notify(method string, params any) error {
	return c.writeMsg(rpcMessage{Method: method, Params: mustJSON(params)})
}

// Reply sends a success response to a server-initiated request.
func (c *Conn) Reply(id int64, result any) error {
	return c.writeMsg(rpcMessage{ID: &id, Result: mustJSON(result)})
}

// ReplyError sends an error response to a server-initiated request.
func (c *Conn) ReplyError(id int64, errMsg string) error {
	return c.writeMsg(rpcMessage{ID: &id, Error: mustJSON(map[string]any{"message": errMsg})})
}

// Close tears down the underlying transport.
func (c *Conn) Close() error { return c.t.Close() }

func (c *Conn) resolvePending(id int64, msg rpcMessage) {
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- msg
	}
}

func (c *Conn) writeMsg(msg rpcMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.t.WriteMessage(context.Background(), data)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
