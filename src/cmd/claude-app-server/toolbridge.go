package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

// bridgeReq is the request sent by the CLI to the tool bridge socket.
type bridgeReq struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

// bridgeResp is the response from the tool bridge to the CLI.
type bridgeResp struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// toolBridge is a Unix-domain socket server that bridges CLI tool calls to the
// orchestrator via item/tool/call.  One bridge is created per turn; it is
// closed after the turn completes.
//
// conn.Request is used concurrently from socket-handler goroutines.  The
// transport's internal write mutex serialises the wire writes, so no
// additional lock is required here.
type toolBridge struct {
	socketPath string
	listener   net.Listener
	conn       *codexclient.Conn
	threadID   string
	turnID     string
	newID      func() string
	wg         sync.WaitGroup
}

// socketPathFor converts an ID to a Unix socket path.  If id already looks
// like an absolute path (starts with '/') it is returned unchanged, allowing
// tests to place sockets inside t.TempDir().  Otherwise the path is
// /tmp/tool-bridge-<id>.sock.
func socketPathFor(id string) string {
	if len(id) > 0 && id[0] == '/' {
		return id
	}
	return fmt.Sprintf("/tmp/tool-bridge-%s.sock", id)
}

// newToolBridge creates and starts a tool bridge at a temporary Unix socket.
// In production newID returns a UUID; tests may return an absolute path under
// t.TempDir() to avoid cross-test socket collisions.
func newToolBridge(conn *codexclient.Conn, threadID, turnID string, newID func() string) (*toolBridge, error) {
	socketPath := socketPathFor(newID())
	// Remove stale socket file left by a previous crash.
	os.Remove(socketPath) //nolint:errcheck
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("tool bridge listen: %w", err)
	}
	b := &toolBridge{
		socketPath: socketPath,
		listener:   ln,
		conn:       conn,
		threadID:   threadID,
		turnID:     turnID,
		newID:      newID,
	}
	b.wg.Add(1)
	go b.serve()
	return b, nil
}

// SocketPath returns the Unix socket path for use in the TOOL_BRIDGE_SOCKET env var.
func (b *toolBridge) SocketPath() string { return b.socketPath }

// Close shuts down the bridge listener and waits for in-flight connections.
func (b *toolBridge) Close() {
	b.listener.Close()
	b.wg.Wait()
	_ = os.Remove(b.socketPath)
}

func (b *toolBridge) serve() {
	defer b.wg.Done()
	for {
		c, err := b.listener.Accept()
		if err != nil {
			return // listener closed
		}
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.handleConn(c)
		}()
	}
}

func (b *toolBridge) handleConn(c net.Conn) {
	defer c.Close()

	var req bridgeReq
	if err := json.NewDecoder(c).Decode(&req); err != nil {
		b.sendResp(c, bridgeResp{Error: fmt.Sprintf("decode request: %v", err)})
		return
	}

	result, err := b.conn.Request(codexschema.MethodItemToolCall, map[string]any{
		"tool":      req.Tool,
		"arguments": req.Arguments,
		"callId":    b.newID(),
		"threadId":  b.threadID,
		"turnId":    b.turnID,
	})
	if err != nil {
		b.sendResp(c, bridgeResp{Error: fmt.Sprintf("tool call failed: %v", err)})
		return
	}

	var r struct {
		Success *bool  `json:"success"`
		Output  string `json:"output"`
	}
	_ = json.Unmarshal(result, &r)
	output := r.Output
	if output == "" {
		output = string(result)
	}
	b.sendResp(c, bridgeResp{
		Success: r.Success == nil || *r.Success,
		Output:  output,
	})
}

func (b *toolBridge) sendResp(c net.Conn, resp bridgeResp) {
	if err := json.NewEncoder(c).Encode(resp); err != nil {
		slog.Error("tool bridge write response", "err", err)
	}
}
