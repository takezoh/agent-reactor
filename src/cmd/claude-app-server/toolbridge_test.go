package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

// mockOrchestrator is a test codexclient.Handler that intercepts item/tool/call
// requests, records them, and replies with a canned response.
type mockOrchestrator struct {
	conn      *codexclient.Conn
	toolCalls []json.RawMessage
	reply     map[string]any
	mu        sync.Mutex
}

func (m *mockOrchestrator) OnNotification(_ string, _ json.RawMessage) {}
func (m *mockOrchestrator) OnServerRequest(id int64, method string, params json.RawMessage) {
	if method != codexschema.MethodItemToolCall {
		return
	}
	m.mu.Lock()
	m.toolCalls = append(m.toolCalls, append([]byte{}, params...))
	m.mu.Unlock()
	_ = m.conn.Reply(id, m.reply)
}

func (m *mockOrchestrator) recorded() []json.RawMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]json.RawMessage{}, m.toolCalls...)
}

// newBridgeWithMockOrchestrator wires a toolBridge to a mock orchestrator via
// in-memory pipes, returning the bridge and a cancel func.
func newBridgeWithMockOrchestrator(t *testing.T, orchestratorReply map[string]any) (*toolBridge, *mockOrchestrator) {
	t.Helper()

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	// shim  writes → pw2 → pr2 → orch reads
	// orch  writes → pw1 → pr1 → shim reads
	shimTransport := codexclient.StdioTransport(pr1, pw2)
	orchTransport := codexclient.StdioTransport(pr2, pw1)

	shimConn := codexclient.NewConn(shimTransport, 5*time.Second)
	orchConn := codexclient.NewConn(orchTransport, 5*time.Second)

	orch := &mockOrchestrator{conn: orchConn, reply: orchestratorReply}

	ctx, cancel := context.WithCancel(context.Background())
	// orchConn.Run dispatches incoming requests to orch (e.g. item/tool/call).
	go func() { _ = orchConn.Run(ctx, orch) }()
	// shimConn.Run resolves pending Request channels when responses arrive.
	go func() { _ = shimConn.Run(ctx, nopHandler{}) }()

	t.Cleanup(func() {
		cancel()
		pw1.Close()
		pw2.Close()
		pr1.Close()
		pr2.Close()
	})

	// Use t.TempDir()-rooted paths so each test gets unique sockets that are
	// cleaned up on exit (socketPathFor passes absolute paths through unchanged).
	tmpDir := t.TempDir()
	idN := 0
	var idMu sync.Mutex
	newID := func() string {
		idMu.Lock()
		defer idMu.Unlock()
		idN++
		return fmt.Sprintf("%s/bridge-%d.sock", tmpDir, idN)
	}

	bridge, err := newToolBridge(shimConn, "thread-1", "turn-1", newID)
	require.NoError(t, err)
	t.Cleanup(bridge.Close)

	return bridge, orch
}

// TestToolBridge_RoundTrip verifies that a CLI client connecting to the bridge
// socket causes an item/tool/call to be forwarded to the orchestrator, and the
// orchestrator's response is returned to the CLI client.
func TestToolBridge_RoundTrip(t *testing.T) {
	orchReply := map[string]any{"success": true, "output": `{"data":{"issueUpdate":{"success":true}}}`}
	bridge, orch := newBridgeWithMockOrchestrator(t, orchReply)

	// Simulate the CLI connecting to the bridge.
	conn, err := net.DialTimeout("unix", bridge.SocketPath(), 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	req := bridgeReq{
		Tool:      "linear_graphql",
		Arguments: json.RawMessage(`{"query":"mutation{issueUpdate}"}`),
	}
	require.NoError(t, json.NewEncoder(conn).Encode(req))

	var resp bridgeResp
	require.NoError(t, json.NewDecoder(conn).Decode(&resp))

	assert.True(t, resp.Success)
	assert.Contains(t, resp.Output, "issueUpdate")

	// Orchestrator received exactly one item/tool/call with the right fields.
	calls := orch.recorded()
	require.Len(t, calls, 1)
	var params map[string]any
	require.NoError(t, json.Unmarshal(calls[0], &params))
	assert.Equal(t, "linear_graphql", params["tool"])
	assert.Equal(t, "thread-1", params["threadId"])
	assert.Equal(t, "turn-1", params["turnId"])
}

// TestToolBridge_OrchestratorError verifies that an orchestrator error is
// propagated to the CLI client as a bridgeResp with a non-empty Error field.
func TestToolBridge_OrchestratorError(t *testing.T) {
	// mockOrchestrator replies with a JSON-RPC error if we give it a nil reply
	// and patch OnServerRequest to send an error reply.
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	shimTransport := codexclient.StdioTransport(pr1, pw2)
	orchTransport := codexclient.StdioTransport(pr2, pw1)

	shimConn := codexclient.NewConn(shimTransport, 2*time.Second)
	orchConn := codexclient.NewConn(orchTransport, 2*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = orchConn.Run(ctx, &errorOrchestrator{conn: orchConn}) }()
	go func() { _ = shimConn.Run(ctx, nopHandler{}) }()

	t.Cleanup(func() {
		cancel()
		pw1.Close()
		pw2.Close()
		pr1.Close()
		pr2.Close()
	})

	tmpDir := t.TempDir()
	n := 0
	var mu sync.Mutex
	newID := func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("%s/bridge-%d.sock", tmpDir, n)
	}

	bridge, err := newToolBridge(shimConn, "thread-1", "turn-1", newID)
	require.NoError(t, err)
	t.Cleanup(bridge.Close)

	conn, err := net.DialTimeout("unix", bridge.SocketPath(), 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	req := bridgeReq{Tool: "linear_graphql", Arguments: json.RawMessage(`{}`)}
	require.NoError(t, json.NewEncoder(conn).Encode(req))

	var resp bridgeResp
	require.NoError(t, json.NewDecoder(conn).Decode(&resp))

	assert.NotEmpty(t, resp.Error, "bridge should surface orchestrator error")
}

// errorOrchestrator replies to all item/tool/call requests with an RPC error.
type errorOrchestrator struct{ conn *codexclient.Conn }

func (e *errorOrchestrator) OnNotification(_ string, _ json.RawMessage) {}
func (e *errorOrchestrator) OnServerRequest(id int64, method string, _ json.RawMessage) {
	if method == codexschema.MethodItemToolCall {
		_ = e.conn.ReplyError(id, "tool execution failed")
	}
}

// TestToolBridge_ConcurrentCalls verifies that multiple concurrent CLI clients
// each receive the correct response (no response mixing).
func TestToolBridge_ConcurrentCalls(t *testing.T) {
	const n = 5
	orchReply := map[string]any{"success": true, "output": "pong"}
	bridge, orch := newBridgeWithMockOrchestrator(t, orchReply)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("unix", bridge.SocketPath(), 2*time.Second)
			require.NoError(t, err)
			defer conn.Close()

			require.NoError(t, json.NewEncoder(conn).Encode(bridgeReq{
				Tool:      "linear_graphql",
				Arguments: json.RawMessage(`{}`),
			}))

			var resp bridgeResp
			require.NoError(t, json.NewDecoder(conn).Decode(&resp))
			assert.True(t, resp.Success)
			assert.Equal(t, "pong", resp.Output)
		}()
	}
	wg.Wait()

	assert.Len(t, orch.recorded(), n, "each concurrent call should produce one item/tool/call")
}
