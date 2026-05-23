package agent

// Tests for SPEC §10.4 event taxonomy extension (DEV-188).
// Covers: turn_cancelled, startup_failed, unsupported_tool_call.

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workspace"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// ---- turn_cancelled ----

// TestSpawn_turnCancelledOnKill verifies that killing a running session emits
// turn_cancelled (not turn_failed) because the cancellation is intentional.
func TestSpawn_turnCancelledOnKill(t *testing.T) {
	fs := &fakeServer{hangTurn: true} // never completes the turn
	r := makeRunner(t, "", makeFakeProc(fs))
	iss := tracker.Issue{Identifier: "PROJ-CANCEL"}

	var mu sync.Mutex
	var events []Event
	emit := func(e Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := r.spawnWith(ctx, iss, 1, emit)
	require.NoError(t, err)

	// Kill the worker while the turn is still running.
	err = sess.Worker.Kill("test: intentional kill")
	require.NoError(t, err)

	// Wait for the cancelled event to be emitted by runLoop.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	evs := make([]Event, len(events))
	copy(evs, events)
	mu.Unlock()

	require.GreaterOrEqual(t, len(evs), 2, "want session_started + turn_cancelled")
	assert.Equal(t, EventSessionStarted, evs[0].Kind)
	assert.Equal(t, EventTurnCancelled, evs[1].Kind)
	assert.Nil(t, evs[1].Err, "turn_cancelled must carry no error")
}

// ---- startup_failed ----

// startupFailServer handles Initialize and ThreadStart but closes the runner's
// read pipe on the first TurnStart notification, causing initSession to fail
// with "codex exited before session started".
type startupFailServer struct {
	srv        *codexclient.Server
	killWriter func()
}

func (s *startupFailServer) OnServerRequest(id int64, method string, _ json.RawMessage) {
	switch method {
	case codexschema.MethodInitialize:
		_ = s.srv.Conn().Reply(id, map[string]any{})
	case codexschema.MethodThreadStart:
		_ = s.srv.Conn().Reply(id, map[string]any{"thread": map[string]any{"id": testThreadID}})
	}
}

func (s *startupFailServer) OnNotification(method string, _ json.RawMessage) {
	if method == codexschema.MethodTurnStart {
		// Close the writer end of the runner's read pipe → EOF on runner's
		// conn.Run → doneCh closes → initSession returns an error.
		s.killWriter()
	}
}

func makeStartupFailProc() (procFunc, *startupFailServer) {
	ss := &startupFailServer{}
	proc := func(ctx context.Context, dir string, env map[string]string, command string) (io.ReadCloser, io.WriteCloser, func(), error) {
		pr1, pw1 := io.Pipe()
		pr2, pw2 := io.Pipe()

		serverConn := codexclient.NewConn(codexclient.StdioTransport(pr2, pw1), 2*time.Second)
		ss.srv = codexclient.NewServer(serverConn)
		ss.killWriter = func() { _ = pw1.Close() }

		go func() {
			defer func() { _ = pw2.Close() }()
			_ = serverConn.Run(ctx, ss)
		}()
		go func() {
			<-ctx.Done()
			_ = pw1.Close()
		}()
		return pr1, pw2, func() {}, nil
	}
	return proc, ss
}

// TestSpawn_startupFailedEmitsEvent verifies that a startup_failed event is
// emitted when initSession fails (codex exits before session ready).
func TestSpawn_startupFailedEmitsEvent(t *testing.T) {
	proc, _ := makeStartupFailProc()

	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused-in-test"},
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     agentlaunch.DirectDispatcher{},
		proc:           proc,
	}
	iss := tracker.Issue{Identifier: "PROJ-SF"}

	var events []Event
	emit := func(e Event) { events = append(events, e) }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.spawnWith(ctx, iss, 1, emit)
	assert.Error(t, err, "spawnWith must return an error on startup failure")

	require.Len(t, events, 1, "want exactly one startup_failed event")
	assert.Equal(t, EventStartupFailed, events[0].Kind)
	assert.NotNil(t, events[0].Err)
}

// ---- unsupported_tool_call ----

// TestSpawn_unsupportedToolCallEmitsEvent verifies that receiving an item/tool/call
// for an unregistered tool name emits unsupported_tool_call (§10.4).
func TestSpawn_unsupportedToolCallEmitsEvent(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"query": "x"})
	ts := &toolCallServer{toolName: "nonexistent_tool", args: args}

	// No LinearClient — all tool calls are unsupported.
	r := makeRunner(t, "", makeToolCallProc(ts))

	var mu sync.Mutex
	var events []Event
	emit := func(e Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.spawnWith(ctx, testIssue(), 1, emit)
	require.NoError(t, err)

	// Wait for the unsupported_tool_call event.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		found := false
		for _, e := range events {
			if e.Kind == EventUnsupportedToolCall {
				found = true
				break
			}
		}
		mu.Unlock()
		if found {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	evs := make([]Event, len(events))
	copy(evs, events)
	mu.Unlock()

	var toolEvent *Event
	for i := range evs {
		if evs[i].Kind == EventUnsupportedToolCall {
			toolEvent = &evs[i]
			break
		}
	}
	require.NotNil(t, toolEvent, "want unsupported_tool_call event")
	assert.NotEmpty(t, toolEvent.ThreadID, "event must carry a thread id")
	assert.NotEmpty(t, toolEvent.TurnID, "event must carry a turn id")
}
