package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/lib/claude/streamjson"
)

// pipeShim wires the shim server to a client Conn via two io.Pipe pairs.
// ids is a deterministic sequence for newID; returns the client conn, cancel, and a done chan.
func pipeShim(t *testing.T, launch claudeLauncher, ids []string) (*codexclient.Conn, context.CancelFunc, <-chan struct{}) {
	t.Helper()
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	shimTransport := codexclient.StdioTransport(pr1, pw2)
	clientTransport := codexclient.StdioTransport(pr2, pw1)

	ctx, cancel := context.WithCancel(context.Background())

	idIdx := 0
	var idMu sync.Mutex
	newID := func() string {
		idMu.Lock()
		defer idMu.Unlock()
		if idIdx < len(ids) {
			v := ids[idIdx]
			idIdx++
			return v
		}
		return "id-extra"
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer pw2.Close()
		runWith(ctx, shimTransport, launch, newID)
	}()

	t.Cleanup(func() {
		cancel()
		pw1.Close()
		<-done
	})

	clientConn := codexclient.NewConn(clientTransport, 5*time.Second)
	return clientConn, cancel, done
}

// notifEvent is a single recorded server→client notification.
type notifEvent struct {
	method string
	params json.RawMessage
}

// notificationCollector records inbound notifications on a client Conn.
type notificationCollector struct {
	mu     sync.Mutex
	events []notifEvent
}

func (c *notificationCollector) OnNotification(method string, params json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, notifEvent{method, append([]byte{}, params...)})
}

func (c *notificationCollector) OnServerRequest(_ int64, _ string, _ json.RawMessage) {}

func (c *notificationCollector) received() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.events))
	for i, e := range c.events {
		out[i] = e.method
	}
	return out
}

// lastParams returns the decoded params of the last notification with the given method.
func (c *notificationCollector) lastParams(method string) map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.events) - 1; i >= 0; i-- {
		if c.events[i].method == method {
			var m map[string]any
			_ = json.Unmarshal(c.events[i].params, &m)
			return m
		}
	}
	return nil
}

// nthParams returns the decoded params of the n-th (0-based) notification with the given method.
func (c *notificationCollector) nthParams(method string, n int) map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, e := range c.events {
		if e.method == method {
			if count == n {
				var m map[string]any
				_ = json.Unmarshal(e.params, &m)
				return m
			}
			count++
		}
	}
	return nil
}

// fakeLauncherSequence returns a launcher that, for each successive call,
// returns the corresponding line sequence from sequences. The last sequence
// is reused for any extra calls.
func fakeLauncherSequence(calls *[][]string, sequences ...[]string) claudeLauncher {
	i := 0
	var mu sync.Mutex
	return func(ctx context.Context, cwd, resumeID, prompt string) (io.ReadCloser, func() error, error) {
		mu.Lock()
		idx := i
		if i < len(sequences)-1 {
			i++
		} else {
			i = len(sequences) - 1
		}
		mu.Unlock()
		*calls = append(*calls, []string{cwd, resumeID, prompt})
		body := strings.Join(sequences[idx], "\n") + "\n"
		return io.NopCloser(strings.NewReader(body)), func() error { return nil }, nil
	}
}

// stream-json fixtures.
const (
	lineSystemInit = `{"type":"system","subtype":"init","session_id":"claude-sess-1"}`
	lineAssistant  = `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`
	lineResultOK   = `{"type":"result","subtype":"success","result":"done","is_error":false,"usage":{"input_tokens":10,"output_tokens":5}}`
	lineResultFail = `{"type":"result","subtype":"error","result":"oops","is_error":true,"usage":{"input_tokens":1,"output_tokens":0}}`
	lineToolUse    = `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu-1","name":"Bash","input":{"command":"ls"}}]}}`
	lineToolResult = `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu-1","content":"file1.txt","is_error":false}]}}`
)

func waitForMethods(t *testing.T, nc *notificationCollector, want []string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got := nc.received()
		if len(got) >= len(want) {
			assert.Equal(t, want, got[:len(want)])
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %v, got %v", want, nc.received())
}

func TestShim_OneTurn(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineAssistant, lineResultOK},
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("hi")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodItemAgentMessageDelta,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})
}

func TestShim_SessionID(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls, []string{lineSystemInit, lineResultOK})
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("hi")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	turnStartedParams := nc.lastParams(codexschema.MethodTurnStarted)
	assert.Equal(t, "thread-1-turn-1", turnStartedParams["sessionId"])

	turnCompletedParams := nc.lastParams(codexschema.MethodTurnCompleted)
	assert.Equal(t, "thread-1-turn-1", turnCompletedParams["sessionId"])
	assert.Equal(t, "done", turnCompletedParams["text"])
}

func TestShim_ContinuationResume(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineResultOK}, // first turn: new session
		[]string{lineSystemInit, lineResultOK}, // second turn: resume
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1", "turn-2"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))

	// First turn: no threadId → new thread/turn.
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("first")))
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	// Second turn: pass threadId → shim should call launcher with --resume session id.
	require.NoError(t, codexclient.StartTurn(clientConn, "thread-1", "/ws", []byte("second")))
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	require.Len(t, calls, 2)
	assert.Equal(t, "claude-sess-1", calls[1][1], "second turn should resume with claude session id")
}

func TestShim_TurnFailed(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls, []string{lineSystemInit, lineResultFail})
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("fail me")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodError,
	})

	errParams := nc.lastParams(codexschema.MethodError)
	assert.Equal(t, "oops", errParams["message"])
}

// TestShim_ToolEvents verifies that tool_use and tool_result are emitted as
// dynamicToolCall item/started and item/completed notifications.
func TestShim_ToolEvents(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineToolUse, lineToolResult, lineResultOK},
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("run a tool")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodItemStarted,
		codexschema.MethodItemCompleted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	// item/started: type=dynamicToolCall, tool=Bash, id=tu-1
	startedParams := nc.lastParams(codexschema.MethodItemStarted)
	require.NotNil(t, startedParams)
	item, ok := startedParams["item"].(map[string]any)
	require.True(t, ok, "item/started params must have an 'item' object")
	assert.Equal(t, "dynamicToolCall", item["type"])
	assert.Equal(t, "Bash", item["tool"])
	assert.Equal(t, "tu-1", item["id"])

	// item/completed: same id, status=completed, output present
	completedParams := nc.lastParams(codexschema.MethodItemCompleted)
	require.NotNil(t, completedParams)
	completedItem, ok := completedParams["item"].(map[string]any)
	require.True(t, ok, "item/completed params must have an 'item' object")
	assert.Equal(t, "dynamicToolCall", completedItem["type"])
	assert.Equal(t, "tu-1", completedItem["id"])
	assert.Equal(t, "completed", completedItem["status"])
	assert.Equal(t, "file1.txt", completedItem["output"])
}

// TestShim_ToolEventErrored verifies that an errored tool_result emits status=failed.
func TestShim_ToolEventErrored(t *testing.T) {
	erroredResult := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu-1","content":"permission denied","is_error":true}]}}`
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineToolUse, erroredResult, lineResultOK},
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("run a tool")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodItemStarted,
		codexschema.MethodItemCompleted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	completedParams := nc.lastParams(codexschema.MethodItemCompleted)
	completedItem := completedParams["item"].(map[string]any)
	assert.Equal(t, "failed", completedItem["status"])
	assert.Equal(t, "permission denied", completedItem["output"])
}

// TestShim_TokenUsage verifies per-turn token usage is emitted before turn/completed.
func TestShim_TokenUsage(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls, []string{lineSystemInit, lineResultOK})
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("hi")))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	usageParams := nc.lastParams(codexschema.MethodThreadTokenUsageUpdated)
	require.NotNil(t, usageParams)
	tu, ok := usageParams["tokenUsage"].(map[string]any)
	require.True(t, ok)

	last, ok := tu["last"].(map[string]any)
	require.True(t, ok)
	// lineResultOK has input:10, output:5
	assert.EqualValues(t, 10, last["inputTokens"])
	assert.EqualValues(t, 5, last["outputTokens"])
	assert.EqualValues(t, 15, last["totalTokens"])

	total, ok := tu["total"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 10, total["inputTokens"])
	assert.EqualValues(t, 5, total["outputTokens"])
}

// TestShim_TokenUsageCumulative verifies that a second turn accumulates into the thread total.
func TestShim_TokenUsageCumulative(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineResultOK},
		[]string{lineSystemInit, lineResultOK},
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1", "turn-2"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("first")))
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	require.NoError(t, codexclient.StartTurn(clientConn, "thread-1", "/ws", []byte("second")))
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	// Second usage notification: last is per-turn, total is cumulative (2 turns × 10/5)
	secondUsage := nc.nthParams(codexschema.MethodThreadTokenUsageUpdated, 1)
	require.NotNil(t, secondUsage, "second token usage notification must exist")
	tu := secondUsage["tokenUsage"].(map[string]any)

	last := tu["last"].(map[string]any)
	assert.EqualValues(t, 10, last["inputTokens"], "last should be per-turn")

	total := tu["total"].(map[string]any)
	assert.EqualValues(t, 20, total["inputTokens"], "total should be cumulative across 2 turns")
	assert.EqualValues(t, 10, total["outputTokens"])
}

// TestShim_ApprovalSandboxNoEnforce verifies that a turn/start carrying
// approvalPolicy/sandboxPolicy fields still completes normally (not enforced).
func TestShim_ApprovalSandboxNoEnforce(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls, []string{lineSystemInit, lineResultOK})
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))

	// Send turn/start with approval and sandbox fields present.
	params, _ := json.Marshal(map[string]any{
		"threadId":       "",
		"cwd":            "/ws",
		"message":        "do something",
		"approvalPolicy": "localSandboxed",
		"sandboxPolicy":  "sandbox-all",
	})
	require.NoError(t, clientConn.Notify(codexschema.MethodTurnStart, params))

	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodThreadTokenUsageUpdated,
		codexschema.MethodTurnCompleted,
	})

	// Turn completed normally — approval not enforced.
	assert.Len(t, calls, 1, "launch must have been called once")
}

// TestParseTurnStart_ApprovalSandboxFields verifies that parseTurnStart extracts
// approvalPolicy and sandboxPolicy from the wire params.
func TestParseTurnStart_ApprovalSandboxFields(t *testing.T) {
	params, err := json.Marshal(map[string]any{
		"threadId":       "t1",
		"cwd":            "/work",
		"message":        "prompt",
		"approvalPolicy": "localSandboxed",
		"sandboxPolicy":  `{"type":"sandbox-all"}`,
	})
	require.NoError(t, err)

	req := parseTurnStart(params)
	assert.Equal(t, "t1", req.threadID)
	assert.Equal(t, "/work", req.cwd)
	assert.Equal(t, "prompt", req.prompt)
	assert.Equal(t, `"localSandboxed"`, req.approvalPolicy)
	assert.NotEmpty(t, req.sandboxPolicy)
}

// TestShim_ApprovalSandboxWarnLog verifies that receiving approval/sandbox fields
// produces a slog.Warn entry.
func TestShim_ApprovalSandboxWarnLog(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(handler))

	called := make(chan struct{}, 1)
	launch := func(ctx context.Context, cwd, resumeID, prompt string) (io.ReadCloser, func() error, error) {
		called <- struct{}{}
		body := strings.Join([]string{lineSystemInit, lineResultOK}, "\n") + "\n"
		return io.NopCloser(strings.NewReader(body)), func() error { return nil }, nil
	}

	conn, _ := newTestRunnerConn(t, launch, []string{"t", "u"})
	conn.runTurn(turnReq{
		prompt:         "hi",
		approvalPolicy: `"localSandboxed"`,
		sandboxPolicy:  `"sandbox-all"`,
	})

	assert.Contains(t, buf.String(), "approval/sandbox policy received but not enforced by shim")
}

// nopHandler is a no-op codexclient.Handler used for drain goroutines.
type nopHandler struct{}

func (nopHandler) OnNotification(_ string, _ json.RawMessage)           {}
func (nopHandler) OnServerRequest(_ int64, _ string, _ json.RawMessage) {}

// newTestRunnerConn creates a turnRunner wired to a drained server for unit
// tests that call runTurn directly (bypassing the full stdio stack).
func newTestRunnerConn(t *testing.T, launch claudeLauncher, ids []string) (*turnRunner, *codexclient.Server) {
	t.Helper()
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	shimT := codexclient.StdioTransport(pr1, pw2)
	clientT := codexclient.StdioTransport(pr2, pw1)

	conn := codexclient.NewConn(shimT, 0)
	srv := codexclient.NewServer(conn)

	// Drain the client side so conn writes don't block.
	clientConn := codexclient.NewConn(clientT, 0)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = clientConn.Run(ctx, nopHandler{}) }()

	t.Cleanup(func() {
		cancel()
		pw1.Close()
		pw2.Close()
		pr1.Close()
		pr2.Close()
	})

	idIdx := 0
	var mu sync.Mutex
	newID := func() string {
		mu.Lock()
		defer mu.Unlock()
		if idIdx < len(ids) {
			v := ids[idIdx]
			idIdx++
			return v
		}
		return "x"
	}

	writeMu := &sync.Mutex{}
	runner := &turnRunner{
		ctx:      context.Background(),
		srv:      srv,
		writeMu:  writeMu,
		threads:  make(map[string]string),
		cumUsage: make(map[string]streamjson.Usage),
		launch:   launch,
		newID:    newID,
	}
	return runner, srv
}

// TestShim_ConformanceEventOrder verifies the full event sequence emitted by the
// shim matches the Codex app-server protocol contract (§10.4).
func TestShim_ConformanceEventOrder(t *testing.T) {
	var calls [][]string
	launch := fakeLauncherSequence(&calls,
		[]string{lineSystemInit, lineToolUse, lineToolResult, lineAssistant, lineResultOK},
	)
	clientConn, _, _ := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("full turn")))

	// Full §10.4 sequence: thread lifecycle → turn start → item events → usage → turn end.
	waitForMethods(t, nc, []string{
		codexschema.MethodThreadStarted,
		codexschema.MethodTurnStarted,
		codexschema.MethodItemStarted,             // tool_use
		codexschema.MethodItemCompleted,           // tool_result
		codexschema.MethodItemAgentMessageDelta,   // streaming text
		codexschema.MethodThreadTokenUsageUpdated, // token accounting
		codexschema.MethodTurnCompleted,
	})
}

func TestShim_KillPropagation(t *testing.T) {
	blocked := make(chan struct{})
	var launchCtx context.Context
	var launchMu sync.Mutex
	launch := func(ctx context.Context, cwd, resumeID, prompt string) (io.ReadCloser, func() error, error) {
		launchMu.Lock()
		launchCtx = ctx
		launchMu.Unlock()
		close(blocked)
		pr, pw := io.Pipe()
		go func() {
			<-ctx.Done()
			_ = pw.Close()
		}()
		return pr, func() error {
			<-ctx.Done()
			return ctx.Err()
		}, nil
	}

	clientConn, cancel, shimDone := pipeShim(t, launch, []string{"thread-1", "turn-1"})

	nc := &notificationCollector{}
	go func() { _ = clientConn.Run(context.Background(), nc) }()

	require.NoError(t, codexclient.Initialize(clientConn))
	require.NoError(t, codexclient.StartTurn(clientConn, "", "/ws", []byte("block")))

	select {
	case <-blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("launcher was never called")
	}

	cancel()

	select {
	case <-shimDone:
	case <-time.After(3 * time.Second):
		t.Fatal("shim did not exit after cancel")
	}

	launchMu.Lock()
	lctx := launchCtx
	launchMu.Unlock()
	select {
	case <-lctx.Done():
	default:
		t.Error("launcher ctx was not cancelled when shim was cancelled")
	}
}

// TestProcessGroupKill verifies the process-group kill mechanism: spawning a
// process with Setpgid=true and a group-kill Cancel function terminates
// grandchildren when the context is cancelled.
func TestProcessGroupKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group kill not applicable on windows")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	dir := t.TempDir()
	scriptPath := dir + "/parent.sh"
	script := "#!/usr/bin/env bash\nsleep 9999 &\necho \"$!\"\nwait\n"
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "bash", scriptPath) //nolint:gosec
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	// Read the grandchild PID.
	buf := make([]byte, 32)
	n, _ := stdout.Read(buf)
	grandchildPID := strings.TrimSpace(string(buf[:n]))
	require.NotEmpty(t, grandchildPID)

	// Kill via context cancellation.
	cancel()
	_ = cmd.Wait()

	time.Sleep(100 * time.Millisecond)

	out, _ := exec.Command("kill", "-0", grandchildPID).CombinedOutput()
	assert.Contains(t, string(out), "No such process",
		"grandchild PID %s should be dead after process group kill", grandchildPID)
}
