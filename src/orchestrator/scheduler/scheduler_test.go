package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const validFrontMatter = `---
tracker:
  kind: linear
  api_key: lin_api_test
  project_slug: test-proj
codex:
  command: codex app-server
---
`

func writeWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(validFrontMatter), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeTracker implements CandidateSource for tests.
type fakeTracker struct {
	mu      sync.Mutex
	issues  []tracker.Issue
	callErr error
	calls   int
}

func (f *fakeTracker) Candidates(_ context.Context) ([]tracker.Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.callErr != nil {
		return nil, f.callErr
	}
	return f.issues, nil
}

// fakeSpawn records calls and returns the configured session/error.
type fakeSpawn struct {
	mu      sync.Mutex
	calls   []spawnCall
	session LiveSession
	err     error
}

type spawnCall struct {
	Issue   tracker.Issue
	Attempt int
}

func (f *fakeSpawn) fn(ctx context.Context, iss tracker.Issue, attempt int) (LiveSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, spawnCall{iss, attempt})
	return f.session, f.err
}

func (f *fakeSpawn) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// fakeClock drives time manually for backoff tests.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	fireAt time.Time
	fn     func()
	done   bool
}

func (t *fakeTimer) Stop() bool {
	if t.done {
		return false
	}
	t.done = true
	return true
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) NewTimer(d time.Duration, fn func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	ft := &fakeTimer{fireAt: c.now.Add(d), fn: fn}
	c.timers = append(c.timers, ft)
	return ft
}

// Advance moves time forward and fires all due timers.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*fakeTimer
	for _, ft := range c.timers {
		if !ft.done && !ft.fireAt.After(c.now) {
			ft.done = true
			due = append(due, ft)
		}
	}
	c.mu.Unlock()
	for _, ft := range due {
		ft.fn()
	}
}

func minDeps(tr CandidateSource, spawn SpawnFunc, clk Clock) Deps {
	return Deps{Tracker: tr, Spawn: spawn, Clock: clk}
}

func schedCfg() wfconfig.Config {
	return wfconfig.Config{
		Polling: wfconfig.PollingConfig{IntervalMS: 1},
		Tracker: wfconfig.TrackerConfig{
			Kind:           "linear",
			APIKey:         "lin_api_test",
			ProjectSlug:    "test-proj",
			ActiveStates:   []string{"In Progress", "Todo"},
			TerminalStates: []string{"Done", "Cancelled"},
		},
		Agent: wfconfig.AgentConfig{
			MaxConcurrentAgents: 5,
			MaxRetryBackoffMS:   60_000,
		},
		Codex: wfconfig.CodexConfig{Command: "codex app-server"},
	}
}

// TestRunGracefulShutdown verifies Run exits cleanly when ctx is cancelled.
func TestRunGracefulShutdown(t *testing.T) {
	path := writeWorkflow(t)
	cfg := wfconfig.Config{Polling: wfconfig.PollingConfig{IntervalMS: 1}}
	s := New(path, cfg, Deps{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not shut down in time")
	}
}

// TestRunContinuesAfterTickPreflightFailure verifies the loop survives preflight errors.
func TestRunContinuesAfterTickPreflightFailure(t *testing.T) {
	path := writeWorkflow(t)
	cfg := wfconfig.Config{Polling: wfconfig.PollingConfig{IntervalMS: 1}}
	s := New(path, cfg, Deps{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	badContent := strings.ReplaceAll(validFrontMatter, "project_slug: test-proj\n", "")
	if err := os.WriteFile(path, []byte(badContent), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not shut down in time")
	}
}

// TestTickDispatchesEligibleIssues verifies dispatch calls spawn for eligible issues.
func TestTickDispatchesEligibleIssues(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{
		{ID: "1", Identifier: "P-1", Title: "issue one", State: "In Progress"},
	}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())

	s := New("", schedCfg(), minDeps(tr, spawn.fn, clk))
	s.workflowPath = writeWorkflow(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.tickOnce(ctx)

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn call, got %d", spawn.callCount())
	}
}

// TestTickSpawnFailSchedulesRetry verifies spawn failure leads to retry scheduling.
func TestTickSpawnFailSchedulesRetry(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{
		{ID: "1", Identifier: "P-1", Title: "issue", State: "In Progress"},
	}}
	spawn := &fakeSpawn{err: context.DeadlineExceeded}
	clk := newFakeClock(time.Now())

	s := New("", schedCfg(), minDeps(tr, spawn.fn, clk))
	s.workflowPath = writeWorkflow(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.tickOnce(ctx)

	snap := s.state.Snapshot()
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want retry entry after spawn failure")
	}
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released after spawn failure")
	}
}

// TestRunReconcileErrorContinues verifies reconcile error does not stop the loop.
func TestRunReconcileErrorContinues(t *testing.T) {
	path := writeWorkflow(t)
	cfg := wfconfig.Config{Polling: wfconfig.PollingConfig{IntervalMS: 1}}
	called := false
	deps := Deps{Reconcile: func(context.Context) error {
		called = true
		return context.DeadlineExceeded
	}}
	s := New(path, cfg, deps)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx) //nolint:errcheck

	if !called {
		t.Error("want reconcile to be called")
	}
}
