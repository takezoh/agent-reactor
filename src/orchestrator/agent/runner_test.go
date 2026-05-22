package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

const (
	testThreadID = "thread-abc"
	testTurnID   = "turn-xyz"
)

// fakeServer simulates a codex app-server over an in-memory pipe.
// It handles initialize, then responds to turn/start by emitting the standard sequence.
type fakeServer struct {
	srv              *codexclient.Server
	failTurn         bool // if true, emits error instead of turn/completed
	hangTurn         bool // if true, starts the session but never completes the turn
	mu               sync.Mutex
	lastCWD          string          // cwd from the most recent turn/start notification
	lastDynamicTools json.RawMessage // params from the most recent thread/start request
}

func (f *fakeServer) OnNotification(method string, params json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		return
	}

	// Capture the cwd field from the params for test assertions.
	var p struct {
		CWD string `json:"cwd"`
	}
	if err := json.Unmarshal(params, &p); err == nil {
		f.mu.Lock()
		f.lastCWD = p.CWD
		f.mu.Unlock()
	}

	f.mu.Lock()
	fail := f.failTurn
	hang := f.hangTurn
	f.mu.Unlock()

	_ = f.srv.EmitThreadStarted(testThreadID, "/ws")
	_ = f.srv.EmitTurnStarted(testThreadID, testTurnID)
	switch {
	case hang:
		// session is live but the turn never resolves — exercises turn_timeout_ms.
	case fail:
		_ = f.srv.EmitTurnFailed(testThreadID, "simulated failure")
	default:
		_ = f.srv.EmitTurnCompleted(testThreadID, testTurnID, "done")
	}
}

func (f *fakeServer) getLastCWD() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastCWD
}

func (f *fakeServer) OnServerRequest(id int64, method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodInitialize:
		_ = f.srv.Conn().Reply(id, map[string]any{})
	case codexschema.MethodThreadStart:
		f.mu.Lock()
		f.lastDynamicTools = params
		f.mu.Unlock()
		_ = f.srv.Conn().Reply(id, map[string]any{"thread": map[string]any{"id": testThreadID}})
	}
}

// makeFakeProc returns a procFunc that wires runner ↔ fakeServer via io.Pipe.
func makeFakeProc(fs *fakeServer) procFunc {
	return func(ctx context.Context, dir string, env map[string]string, command string) (io.ReadCloser, io.WriteCloser, func(), error) {
		// runner reads pr1; server reads pr2
		pr1, pw1 := io.Pipe()
		pr2, pw2 := io.Pipe()

		serverConn := codexclient.NewConn(
			codexclient.StdioTransport(pr2, pw1),
			2*time.Second,
		)
		fs.srv = codexclient.NewServer(serverConn)

		go func() {
			defer pw2.Close()
			_ = serverConn.Run(ctx, fs)
		}()

		// The stdio transport is not context-aware; emulate process death on
		// cancellation by closing the runner's read end so its loop sees EOF
		// (a real bash subprocess dies and EOFs its stdout the same way).
		go func() {
			<-ctx.Done()
			_ = pw1.Close()
		}()

		return pr1, pw2, func() {}, nil
	}
}

func makeRunner(t *testing.T, tmpl string, proc procFunc) *Runner {
	t.Helper()
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused-in-test"},
	}
	ws := workspace.New(cfg)
	return &Runner{
		Workspace:      ws,
		Cfg:            cfg,
		PromptTemplate: tmpl,
		Dispatcher:     agentlaunch.DirectDispatcher{},
		proc:           proc,
	}
}

func collectEvents(t *testing.T, r *Runner, issue tracker.Issue) []Event {
	t.Helper()
	var mu sync.Mutex
	var events []Event
	emit := func(e Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := r.spawnWith(ctx, issue, 1, emit)
	require.NoError(t, err)
	assert.Equal(t, testThreadID+"-"+testTurnID, sess.SessionID)
	assert.NotNil(t, sess.Worker)

	// wait for monitor to deliver turn_completed
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n >= 2 { //nolint:mnd
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	out := make([]Event, len(events))
	copy(out, events)
	mu.Unlock()
	return out
}

func TestSpawn_sessionStartedAndTurnCompleted(t *testing.T) {
	fs := &fakeServer{}
	r := makeRunner(t, "Work on {{ issue.identifier }}", makeFakeProc(fs))
	iss := tracker.Issue{Identifier: "PROJ-1", Title: "Test issue"}

	events := collectEvents(t, r, iss)

	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventSessionStarted, events[0].Kind)
	assert.Equal(t, testThreadID+"-"+testTurnID, events[0].SessionID)
	assert.Equal(t, EventTurnCompleted, events[1].Kind)
	assert.Nil(t, events[1].Err)
}

func TestSpawn_turnFailedEmitsEvent(t *testing.T) {
	fs := &fakeServer{failTurn: true}
	r := makeRunner(t, "", makeFakeProc(fs))
	iss := tracker.Issue{Identifier: "PROJ-2"}

	events := collectEvents(t, r, iss)

	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventSessionStarted, events[0].Kind)
	assert.Equal(t, EventTurnFailed, events[1].Kind)
	assert.NotNil(t, events[1].Err)
}

func TestSpawn_turnTimeoutKillsAndFails(t *testing.T) {
	fs := &fakeServer{hangTurn: true}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused", TurnTimeoutMS: 50},
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     agentlaunch.DirectDispatcher{},
		proc:           makeFakeProc(fs),
	}
	iss := tracker.Issue{Identifier: "PROJ-T"}

	events := collectEvents(t, r, iss)

	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventSessionStarted, events[0].Kind)
	assert.Equal(t, EventTurnFailed, events[1].Kind)
	assert.ErrorContains(t, events[1].Err, "turn timeout")
}

func TestSpawn_workspaceEnsureCreatesDir(t *testing.T) {
	fs := &fakeServer{}
	r := makeRunner(t, "", makeFakeProc(fs))
	iss := tracker.Issue{Identifier: "PROJ-3"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	require.NoError(t, err)

	wsPath, _ := r.Workspace.Path(iss.Identifier)
	info, statErr := os.Stat(wsPath)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestSpawn_beforeRunFailureAborts(t *testing.T) {
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Hooks: wfconfig.HooksConfig{
			BeforeRun: "exit 1",
			TimeoutMS: 2000,
		},
		Codex: wfconfig.CodexConfig{Command: "unused"},
	}
	ws := workspace.New(cfg)
	// pre-create the workspace dir so Ensure succeeds before hook runs
	iss := tracker.Issue{Identifier: "PROJ-4"}
	wsPath := filepath.Join(wsRoot, iss.Identifier)
	require.NoError(t, os.MkdirAll(wsPath, 0o755))

	r := &Runner{
		Workspace:      ws,
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     agentlaunch.DirectDispatcher{},
		proc:           makeFakeProc(&fakeServer{}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	assert.Error(t, err, "before_run failure should abort spawn")
}

// TestSPEC_9_4_AfterRunCalledOnBeforeRunFailure verifies SPEC §9.4: after_run
// must execute even when before_run fails (workspace already exists).
func TestSPEC_9_4_AfterRunCalledOnBeforeRunFailure(t *testing.T) {
	wsRoot := t.TempDir()
	markerFile := filepath.Join(t.TempDir(), "after_run_called")
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Hooks: wfconfig.HooksConfig{
			BeforeRun: "exit 1",
			AfterRun:  "touch " + markerFile,
			TimeoutMS: 2000,
		},
		Codex: wfconfig.CodexConfig{Command: "unused"},
	}
	iss := tracker.Issue{Identifier: "PROJ-AR1"}
	// Pre-create workspace so Ensure succeeds before before_run hook runs.
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, iss.Identifier), 0o755))

	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     agentlaunch.DirectDispatcher{},
		proc:           makeFakeProc(&fakeServer{}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	require.Error(t, err, "before_run failure should abort spawn")

	_, statErr := os.Stat(markerFile)
	assert.NoError(t, statErr, "after_run must be called even when before_run fails (SPEC §9.4)")
}

// TestSPEC_9_4_AfterRunCalledOnLaunchConnFailure verifies SPEC §9.4: after_run
// must execute when session launch fails after the workspace was created.
func TestSPEC_9_4_AfterRunCalledOnLaunchConnFailure(t *testing.T) {
	wsRoot := t.TempDir()
	markerFile := filepath.Join(t.TempDir(), "after_run_called")
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Hooks: wfconfig.HooksConfig{
			AfterRun:  "touch " + markerFile,
			TimeoutMS: 2000,
		},
		Codex: wfconfig.CodexConfig{Command: "unused"},
	}
	iss := tracker.Issue{Identifier: "PROJ-AR2"}

	failProc := func(_ context.Context, _ string, _ map[string]string, _ string) (io.ReadCloser, io.WriteCloser, func(), error) {
		return nil, nil, nil, errors.New("proc: simulated launch failure")
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     agentlaunch.DirectDispatcher{},
		proc:           failProc,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	require.Error(t, err, "proc failure should abort spawn")

	_, statErr := os.Stat(markerFile)
	assert.NoError(t, statErr, "after_run must be called even when launchConn fails (SPEC §9.4)")
}

// ---- Dispatcher seam tests (Issue 015) ----

// fakeDispatcher records Wrap calls and optionally returns a custom WrappedLaunch.
type fakeDispatcher struct {
	mu      sync.Mutex
	calls   []fakeWrapCall
	wrapped agentlaunch.WrappedLaunch
}

type fakeWrapCall struct {
	frameID string
	plan    agentlaunch.LaunchPlan
}

func (f *fakeDispatcher) Wrap(_ context.Context, frameID string, plan agentlaunch.LaunchPlan) (agentlaunch.WrappedLaunch, error) {
	f.mu.Lock()
	f.calls = append(f.calls, fakeWrapCall{frameID: frameID, plan: plan})
	w := f.wrapped
	f.mu.Unlock()
	if w.Command == "" {
		w.Command = plan.Command
	}
	if w.StartDir == "" {
		w.StartDir = plan.StartDir
	}
	if w.Env == nil {
		w.Env = plan.Env
	}
	return w, nil
}

func (f *fakeDispatcher) AdoptFrame(_ context.Context, _, _ string) (func(context.Context) error, []agentlaunch.Mount, error) {
	return nil, nil, nil
}
func (f *fakeDispatcher) EnsureProject(_ context.Context, _ string) error { return nil }
func (f *fakeDispatcher) IsContainer(_ string) bool                       { return false }

func (f *fakeDispatcher) wrapCalls() []fakeWrapCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeWrapCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestSpawn_dispatcherWrapInvoked(t *testing.T) {
	fs := &fakeServer{}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "my-codex"},
	}
	d := &fakeDispatcher{}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     d,
		proc:           makeFakeProc(fs),
	}
	iss := tracker.Issue{Identifier: "PROJ-D1"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, iss, 2, func(Event) {})
	require.NoError(t, err)

	calls := d.wrapCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "PROJ-D1#2", calls[0].frameID)
	assert.Equal(t, "my-codex", calls[0].plan.Command)
	// Project is the workspace root so every issue shares one per-project
	// container; the per-issue cwd is carried by StartDir, which pathmap
	// translates to a subdir of the project mount inside the container.
	// SandboxResolver.ResolveProjectScope still resolves correctly because the
	// upward search from the root reaches the same .roost/settings.toml.
	assert.Equal(t, filepath.Clean(wsRoot), calls[0].plan.Project)
	assert.Equal(t, filepath.Join(wsRoot, "PROJ-D1"), calls[0].plan.StartDir)
}

// TestSpawn_perProjectContainerKey is a regression guard for per-project
// container sharing: two different issues must yield the same plan.Project
// (the workspace root, i.e. the container key) but distinct StartDir (cwd).
func TestSpawn_perProjectContainerKey(t *testing.T) {
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "my-codex"},
	}
	d := &fakeDispatcher{}

	for _, id := range []string{"PROJ-A1", "PROJ-B2"} {
		fs := &fakeServer{}
		r := &Runner{
			Workspace:      workspace.New(cfg),
			Cfg:            cfg,
			PromptTemplate: "",
			Dispatcher:     d,
			proc:           makeFakeProc(fs),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := r.spawnWith(ctx, tracker.Issue{Identifier: id}, 1, func(Event) {})
		cancel()
		require.NoError(t, err)
	}

	calls := d.wrapCalls()
	require.Len(t, calls, 2)
	// Same container key for both issues.
	assert.Equal(t, filepath.Clean(wsRoot), calls[0].plan.Project)
	assert.Equal(t, calls[0].plan.Project, calls[1].plan.Project)
	// Distinct per-issue working directories.
	assert.Equal(t, filepath.Join(wsRoot, "PROJ-A1"), calls[0].plan.StartDir)
	assert.Equal(t, filepath.Join(wsRoot, "PROJ-B2"), calls[1].plan.StartDir)
	assert.NotEqual(t, calls[0].plan.StartDir, calls[1].plan.StartDir)
}

func TestSpawn_wrappedFieldsPropagateToProc(t *testing.T) {
	fs := &fakeServer{}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "original"},
	}

	var gotDir, gotCmd string
	var gotEnv map[string]string
	capturingProc := func(ctx context.Context, dir string, env map[string]string, command string) (io.ReadCloser, io.WriteCloser, func(), error) {
		gotDir = dir
		gotEnv = env
		gotCmd = command
		return makeFakeProc(fs)(ctx, dir, env, command)
	}

	overrideDir := t.TempDir()
	d := &fakeDispatcher{
		wrapped: agentlaunch.WrappedLaunch{
			Command:  "overridden-cmd",
			StartDir: overrideDir,
			Env:      map[string]string{"MY_KEY": "MY_VAL"},
		},
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     d,
		proc:           capturingProc,
	}
	iss := tracker.Issue{Identifier: "PROJ-D2"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	require.NoError(t, err)

	assert.Equal(t, "overridden-cmd", gotCmd)
	assert.Equal(t, overrideDir, gotDir)
	assert.Equal(t, "MY_VAL", gotEnv["MY_KEY"])
	// Direct-mode regression guard: when wrapped.StartDir equals the host path,
	// the proc working directory is the host wsPath (not a container path).
	wsPath := filepath.Join(wsRoot, "PROJ-D2")
	assert.NotEqual(t, wsPath, gotDir, "overrideDir should differ from host wsPath")
}

func TestSpawn_cleanupCalledOnceAfterTurnCompleted(t *testing.T) {
	fs := &fakeServer{}

	var mu sync.Mutex
	cleanupCount := 0
	cleanup := func(_ context.Context) error {
		mu.Lock()
		cleanupCount++
		mu.Unlock()
		return nil
	}

	d := &fakeDispatcher{
		wrapped: agentlaunch.WrappedLaunch{Cleanup: cleanup},
	}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused"},
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     d,
		proc:           makeFakeProc(fs),
	}
	iss := tracker.Issue{Identifier: "PROJ-CL1"}

	events := collectEvents(t, r, iss)
	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventTurnCompleted, events[1].Kind)

	// Wait a bit for monitor goroutine to run cleanup after emitting the event.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := cleanupCount
	mu.Unlock()
	assert.Equal(t, 1, count, "cleanup should be called exactly once after turn completes")
}

func TestSpawn_cleanupCalledOnceOnKill(t *testing.T) {
	fs := &fakeServer{hangTurn: true}

	var mu sync.Mutex
	cleanupCount := 0
	cleanup := func(_ context.Context) error {
		mu.Lock()
		cleanupCount++
		mu.Unlock()
		return nil
	}

	d := &fakeDispatcher{
		wrapped: agentlaunch.WrappedLaunch{Cleanup: cleanup},
	}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused"},
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     d,
		proc:           makeFakeProc(fs),
	}
	iss := tracker.Issue{Identifier: "PROJ-CL2"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := r.spawnWith(ctx, iss, 1, func(Event) {})
	require.NoError(t, err)

	// Kill the worker; turn will never complete, so monitor will not run cleanup.
	err = sess.Worker.Kill(fmt.Sprintf("test kill %s", iss.Identifier))
	require.NoError(t, err)

	// Give monitor goroutine a moment to also attempt cleanup (should be no-op via Once).
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := cleanupCount
	mu.Unlock()
	assert.Equal(t, 1, count, "cleanup should be called exactly once on Kill")
}

// TestSpawn_startTurnUsesWrappedStartDir verifies that when DevcontainerLauncher
// translates StartDir to a container path, StartTurn receives the container path.
func TestSpawn_startTurnUsesWrappedStartDir(t *testing.T) {
	containerDir := "/container/ws"
	fs := &fakeServer{}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused"},
	}
	d := &fakeDispatcher{
		wrapped: agentlaunch.WrappedLaunch{
			StartDir: containerDir,
		},
	}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     d,
		proc:           makeFakeProc(fs),
	}
	iss := tracker.Issue{Identifier: "PROJ-SDC"}

	events := collectEvents(t, r, iss)
	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventTurnCompleted, events[1].Kind)

	// StartTurn must have used the container path, not the host wsPath.
	assert.Equal(t, containerDir, fs.getLastCWD())
}

// TestSpawn_startTurnUsesWrappedStartDir_directFallback is a regression guard:
// in direct mode the dispatcher returns StartDir == "" so runner falls back to
// the host wsPath.  StartTurn must still receive the host path.
func TestSpawn_startTurnUsesWrappedStartDir_directFallback(t *testing.T) {
	fs := &fakeServer{}
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused"},
	}
	// fakeDispatcher with empty wrapped.StartDir — mimics DirectDispatcher which
	// reflects plan.StartDir unchanged.
	d := &fakeDispatcher{}
	r := &Runner{
		Workspace:      workspace.New(cfg),
		Cfg:            cfg,
		PromptTemplate: "",
		Dispatcher:     d,
		proc:           makeFakeProc(fs),
	}
	iss := tracker.Issue{Identifier: "PROJ-SDD"}

	events := collectEvents(t, r, iss)
	require.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, EventTurnCompleted, events[1].Kind)

	// Direct mode: StartTurn cwd must be the host workspace path.
	expectedWS := filepath.Join(wsRoot, "PROJ-SDD")
	assert.Equal(t, expectedWS, fs.getLastCWD())
}

// thread/start must advertise linear_graphql as a dynamicTool when the Linear
// client is configured (§10.5), and advertise none when it is not.
func threadStartDynamicTools(t *testing.T, fs *fakeServer) []map[string]any {
	t.Helper()
	fs.mu.Lock()
	raw := fs.lastDynamicTools
	fs.mu.Unlock()
	require.NotEmpty(t, raw, "thread/start should have been sent")
	var p struct {
		DynamicTools []map[string]any `json:"dynamicTools"`
	}
	require.NoError(t, json.Unmarshal(raw, &p))
	return p.DynamicTools
}

func TestSpawn_advertisesLinearGraphqlWhenConfigured(t *testing.T) {
	fs := &fakeServer{}
	r := makeRunner(t, "", makeFakeProc(fs))
	r.LinearClient = makeLinearServer(t, `{"data":{}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, tracker.Issue{Identifier: "PROJ-DT1"}, 1, func(Event) {})
	require.NoError(t, err)

	tools := threadStartDynamicTools(t, fs)
	require.Len(t, tools, 1)
	assert.Equal(t, "linear_graphql", tools[0]["name"])
}

func TestSpawn_noDynamicToolsWhenLinearUnconfigured(t *testing.T) {
	fs := &fakeServer{}
	r := makeRunner(t, "", makeFakeProc(fs)) // LinearClient nil by default

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, tracker.Issue{Identifier: "PROJ-DT2"}, 1, func(Event) {})
	require.NoError(t, err)

	assert.Empty(t, threadStartDynamicTools(t, fs))
}
