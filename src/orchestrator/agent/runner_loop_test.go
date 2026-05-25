package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workspace"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// scriptedServer handles multiple MethodTurnStart calls. outcomes[i] controls turn i:
// false = turn/completed, true = turn/failed. First call emits thread/started.
type scriptedServer struct {
	mu       sync.Mutex
	srv      *codexclient.Server
	turn     int
	outcomes []bool
}

func (s *scriptedServer) OnNotification(method string, params json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		return
	}
	s.mu.Lock()
	n := s.turn
	fail := n < len(s.outcomes) && s.outcomes[n]
	s.turn++
	s.mu.Unlock()

	if n == 0 {
		_ = s.srv.EmitThreadStarted(testThreadID, "/ws")
	}
	turnID := fmt.Sprintf("turn-%d", n+1)
	_ = s.srv.EmitTurnStarted(testThreadID, turnID)
	if fail {
		_ = s.srv.EmitTurnFailed(testThreadID, "scripted failure")
	} else {
		_ = s.srv.EmitTurnCompleted(testThreadID, turnID, "done")
	}
}

func (s *scriptedServer) OnServerRequest(id int64, method string, _ json.RawMessage) {
	switch method {
	case codexschema.MethodInitialize:
		_ = s.srv.Conn().Reply(id, map[string]any{})
	case codexschema.MethodThreadStart:
		_ = s.srv.Conn().Reply(id, map[string]any{"thread": map[string]any{"id": testThreadID}})
	}
}

// makeScriptedProc wires a runner to a scriptedServer over an in-memory pipe.
func makeScriptedProc(ss *scriptedServer) spawnFunc {
	return func(ctx context.Context, _ agentlaunch.WrappedLaunch, _ agentlaunch.SpawnOptions) (agentlaunch.SpawnResult, error) {
		pr1, pw1 := io.Pipe()
		pr2, pw2 := io.Pipe()

		serverConn := codexclient.NewConn(codexclient.StdioTransport(pr2, pw1), 2*time.Second)
		ss.srv = codexclient.NewServer(serverConn)

		go func() {
			defer pw2.Close()
			_ = serverConn.Run(ctx, ss)
		}()
		go func() {
			<-ctx.Done()
			_ = pw1.Close()
		}()
		return agentlaunch.SpawnResult{Stdout: pr1, Stdin: pw2, Wait: func() error { return nil }}, nil
	}
}

// fakeStateRefresher returns scripted per-call responses for shouldContinue.
type fakeStateRefresher struct {
	mu        sync.Mutex
	responses [][]tracker.Issue
	call      int
}

func (f *fakeStateRefresher) RefreshStates(_ context.Context, _ []string) ([]tracker.Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.call >= len(f.responses) {
		return nil, nil
	}
	r := f.responses[f.call]
	f.call++
	return r, nil
}

func (f *fakeStateRefresher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.call
}

// makeLoopRunner builds a Runner with multi-turn support for loop tests.
func makeLoopRunner(t *testing.T, maxTurns int, spawn spawnFunc, tr stateRefresher) *Runner {
	t.Helper()
	wsRoot := t.TempDir()
	cfg := wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: wsRoot},
		Codex:     wfconfig.CodexConfig{Command: "unused-in-test"},
		Agent:     wfconfig.AgentConfig{MaxTurns: maxTurns},
		Tracker:   wfconfig.TrackerConfig{ActiveStates: []string{"In Progress"}},
	}
	return &Runner{
		Workspace:  workspace.New(cfg),
		Cfg:        cfg,
		Dispatcher: agentlaunch.DirectDispatcher{},
		Tracker:    tr,
		spawn:      spawn,
	}
}

type loopResult struct {
	events []Event
	exit   scheduler.WorkerExit
}

// runAndCollect spawns the runner and waits for WorkerDone before returning.
func runAndCollect(t *testing.T, r *Runner, issue tracker.Issue) loopResult {
	t.Helper()
	workerDone := make(chan scheduler.WorkerExit, 1)
	r.WorkerDone = workerDone

	var mu sync.Mutex
	var events []Event
	emit := func(e Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.spawnWith(ctx, issue, 1, emit)
	require.NoError(t, err)

	var exit scheduler.WorkerExit
	select {
	case exit = <-workerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for WorkerDone")
	}

	mu.Lock()
	out := make([]Event, len(events))
	copy(out, events)
	mu.Unlock()
	return loopResult{events: out, exit: exit}
}

// TestRunLoop_TwoTurns_ActiveThenNoMore verifies: turn 1 completes, refresher returns
// active → turn 2; turn 2 completes, refresher returns nil → loop stops normally.
func TestRunLoop_TwoTurns_ActiveThenNoMore(t *testing.T) {
	ss := &scriptedServer{outcomes: []bool{false, false}}
	tr := &fakeStateRefresher{
		responses: [][]tracker.Issue{
			{{ID: "i1", Identifier: "P-1", State: "In Progress"}}, // after turn 1 → active
			// after turn 2 → nil → stops
		},
	}
	r := makeLoopRunner(t, 10, makeScriptedProc(ss), tr)
	iss := tracker.Issue{ID: "i1", Identifier: "P-1", State: "In Progress"}

	res := runAndCollect(t, r, iss)

	require.GreaterOrEqual(t, len(res.events), 3, "want session_started + 2 turn_completed")
	assert.Equal(t, EventSessionStarted, res.events[0].Kind)
	assert.Equal(t, EventTurnCompleted, res.events[1].Kind)
	assert.Equal(t, EventTurnCompleted, res.events[2].Kind)
	assert.NoError(t, res.exit.Err, "want clean exit")
}

// TestRunLoop_MaxTurnsReached verifies that the loop stops at MaxTurns even when the
// issue remains active.
func TestRunLoop_MaxTurnsReached(t *testing.T) {
	ss := &scriptedServer{outcomes: []bool{false, false}}
	tr := &fakeStateRefresher{
		responses: [][]tracker.Issue{
			{{ID: "i2", Identifier: "P-2", State: "In Progress"}}, // after turn 1 → active
		},
	}
	r := makeLoopRunner(t, 2, makeScriptedProc(ss), tr)
	iss := tracker.Issue{ID: "i2", Identifier: "P-2", State: "In Progress"}

	res := runAndCollect(t, r, iss)

	require.GreaterOrEqual(t, len(res.events), 3, "want session_started + 2 turn_completed")
	assert.Equal(t, EventTurnCompleted, res.events[1].Kind)
	assert.Equal(t, EventTurnCompleted, res.events[2].Kind)
	assert.NoError(t, res.exit.Err)
	assert.Equal(t, 1, tr.callCount(), "want 1 RefreshStates call (turn 2 stops at MaxTurns)")
}

// TestRunLoop_TurnFailed_AbnormalExit verifies that a turn failure stops the loop and
// delivers a non-nil WorkerExit.Err.
func TestRunLoop_TurnFailed_AbnormalExit(t *testing.T) {
	ss := &scriptedServer{outcomes: []bool{true}} // turn 1 fails
	tr := &fakeStateRefresher{}
	r := makeLoopRunner(t, 10, makeScriptedProc(ss), tr)
	iss := tracker.Issue{ID: "i3", Identifier: "P-3", State: "In Progress"}

	res := runAndCollect(t, r, iss)

	require.GreaterOrEqual(t, len(res.events), 2, "want session_started + turn_failed")
	assert.Equal(t, EventTurnFailed, res.events[1].Kind)
	assert.NotNil(t, res.events[1].Err)
	assert.Error(t, res.exit.Err)
	assert.Equal(t, 0, tr.callCount(), "want no RefreshStates on turn failure")
}

// TestRunLoop_GracefulKill_NoTurnFailed verifies that killing a worker with reason
// "terminal" or "non-active" (orchestrator-initiated handoff/completion) does not
// emit turn_failed and produces a nil WorkerExit.Err.
func TestRunLoop_GracefulKill_NoTurnFailed(t *testing.T) {
	for _, reason := range []string{"terminal", "non-active"} {
		t.Run(reason, func(t *testing.T) {
			// hangTurn: session starts but the turn never resolves — simulates the
			// agent mid-turn when reconcile fires.
			fs := &fakeServer{hangTurn: true}
			r := makeLoopRunner(t, 10, makeFakeProc(fs), nil)
			iss := tracker.Issue{ID: "i-graceful", Identifier: "P-G", State: "In Progress"}

			workerDone := make(chan scheduler.WorkerExit, 1)
			r.WorkerDone = workerDone

			var mu sync.Mutex
			var events []Event
			emit := func(e Event) { mu.Lock(); events = append(events, e); mu.Unlock() }

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			session, err := r.spawnWith(ctx, iss, 1, emit)
			require.NoError(t, err)

			// Wait for session_started before killing.
			require.Eventually(t, func() bool {
				mu.Lock()
				defer mu.Unlock()
				return len(events) > 0
			}, 2*time.Second, 10*time.Millisecond)

			// Simulate reconcile killing the worker (handoff or Done transition).
			require.NoError(t, session.Worker.Kill(reason))

			var exit scheduler.WorkerExit
			select {
			case exit = <-workerDone:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for WorkerDone")
			}

			mu.Lock()
			evts := make([]Event, len(events))
			copy(evts, events)
			mu.Unlock()

			for _, e := range evts {
				assert.NotEqual(t, EventTurnFailed, e.Kind,
					"graceful kill(%s) must not emit turn_failed, got event %v", reason, e.Kind)
			}
			assert.NoError(t, exit.Err, "graceful kill(%s) must produce nil WorkerExit.Err", reason)
		})
	}
}

// TestRunLoop_StallKill_EmitsTurnFailed verifies that killing a worker with reason "stall"
// (orchestrator stall-timeout, a real failure) still emits turn_failed and a non-nil exit.
func TestRunLoop_StallKill_EmitsTurnFailed(t *testing.T) {
	fs := &fakeServer{hangTurn: true}
	r := makeLoopRunner(t, 10, makeFakeProc(fs), nil)
	iss := tracker.Issue{ID: "i-stall", Identifier: "P-S", State: "In Progress"}

	workerDone := make(chan scheduler.WorkerExit, 1)
	r.WorkerDone = workerDone

	var mu sync.Mutex
	var events []Event
	emit := func(e Event) { mu.Lock(); events = append(events, e); mu.Unlock() }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := r.spawnWith(ctx, iss, 1, emit)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) > 0
	}, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, session.Worker.Kill("stall"))

	var exit scheduler.WorkerExit
	select {
	case exit = <-workerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for WorkerDone")
	}

	mu.Lock()
	evts := make([]Event, len(events))
	copy(evts, events)
	mu.Unlock()

	hasTurnFailed := false
	for _, e := range evts {
		if e.Kind == EventTurnFailed {
			hasTurnFailed = true
		}
	}
	assert.True(t, hasTurnFailed, "stall kill must emit turn_failed")
	assert.Error(t, exit.Err, "stall kill must produce non-nil WorkerExit.Err")
}

// TestRunLoop_SecondTurnFailed_AbnormalExit verifies that a failure on turn 2 (after
// turn 1 succeeded) results in turn_completed + turn_failed events and a non-nil exit.
func TestRunLoop_SecondTurnFailed_AbnormalExit(t *testing.T) {
	ss := &scriptedServer{outcomes: []bool{false, true}}
	tr := &fakeStateRefresher{
		responses: [][]tracker.Issue{
			{{ID: "i4", Identifier: "P-4", State: "In Progress"}}, // after turn 1 → active
		},
	}
	r := makeLoopRunner(t, 10, makeScriptedProc(ss), tr)
	iss := tracker.Issue{ID: "i4", Identifier: "P-4", State: "In Progress"}

	res := runAndCollect(t, r, iss)

	require.GreaterOrEqual(t, len(res.events), 3, "want session_started + turn_completed + turn_failed")
	assert.Equal(t, EventTurnCompleted, res.events[1].Kind)
	assert.Equal(t, EventTurnFailed, res.events[2].Kind)
	assert.Error(t, res.exit.Err)
}
