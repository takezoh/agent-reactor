package scheduler

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
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
	issues  []ptrackerv.Issue
	callErr error
	calls   int
}

func (f *fakeTracker) Candidates(_ context.Context) ([]ptrackerv.Issue, error) {
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
	Issue   ptrackerv.Issue
	Attempt int
}

func (f *fakeSpawn) fn(ctx context.Context, iss ptrackerv.Issue, attempt int) (LiveSession, error) {
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
	tr := &fakeTracker{issues: []ptrackerv.Issue{
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

// --- Hot-reload tests (issue 023) ---

// warnCapture is a minimal slog.Handler that counts Warn-level records.
type warnCapture struct {
	mu    sync.Mutex
	warns int
}

func (c *warnCapture) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}
func (c *warnCapture) Handle(_ context.Context, r slog.Record) error {
	if r.Level == slog.LevelWarn {
		c.mu.Lock()
		c.warns++
		c.mu.Unlock()
	}
	return nil
}
func (c *warnCapture) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *warnCapture) WithGroup(_ string) slog.Handler      { return c }

func (c *warnCapture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.warns
}

// installWarnCapture replaces the default slog handler and restores it on test cleanup.
func installWarnCapture(t *testing.T) *warnCapture {
	t.Helper()
	cap := &warnCapture{}
	old := slog.Default()
	slog.SetDefault(slog.New(cap))
	t.Cleanup(func() { slog.SetDefault(old) })
	return cap
}

// writeFrontMatter writes a WORKFLOW.md with the given front-matter content.
func writeFrontMatter(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestApplyIntervalUpdatesOnChange verifies that applyInterval resets the ticker
// when lastGood has a different Polling.IntervalMS than s.interval.
func TestApplyIntervalUpdatesOnChange(t *testing.T) {
	path := writeWorkflow(t)           // validFrontMatter; default interval = 30000ms
	s := New(path, schedCfg(), Deps{}) // schedCfg has interval 1ms

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.tickOnce(ctx) // loads WORKFLOW.md → lastGood.Polling.IntervalMS = 30000

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.applyInterval(ticker)

	const wantMS = 30000
	if s.interval != wantMS*time.Millisecond {
		t.Errorf("want interval %dms, got %v", wantMS, s.interval)
	}
}

// TestDispatchGatingOnBadReload verifies that on an invalid reload:
//   - reconcile runs on last-known-good (terminal issue is cleaned up), and
//   - new dispatch is gated (spawn is not called).
func TestDispatchGatingOnBadReload(t *testing.T) {
	path := writeWorkflow(t)
	tr := &fakeTracker{issues: []ptrackerv.Issue{
		{ID: "1", Identifier: "P-1", Title: "issue", State: "In Progress"},
	}}
	spawn := &fakeSpawn{}
	rt := &fakeReconcileTracker{
		refreshIssues: []ptrackerv.Issue{{ID: "1", Identifier: "P-1", State: "Done"}},
	}
	ws := &fakeWorkspace{}
	s := New(path, schedCfg(), Deps{
		Tracker:        tr,
		Spawn:          spawn.fn,
		Clock:          newFakeClock(time.Now()),
		RefreshTracker: rt,
		Workspace:      ws,
	})

	// Put issue "1" in running state so reconcile has something to process.
	_ = s.state.Dispatch(testIssue("1", "P-1"), 1, LiveSession{Worker: &fakeWorker{}}, time.Now())

	// Write invalid config (missing project_slug fails Preflight).
	writeFrontMatter(t, path, strings.ReplaceAll(validFrontMatter, "project_slug: test-proj\n", ""))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.tickOnce(ctx)

	// Dispatch must be gated.
	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawn calls on bad reload, got %d", spawn.callCount())
	}
	// Reconcile must have run: terminal state "Done" should have cleaned up running["1"].
	if _, ok := s.state.Snapshot().Running["1"]; ok {
		t.Error("reconcile should have cleaned up terminal issue '1' using last-known-good config")
	}
}

// TestDispatchResumesAfterRecovery verifies that dispatch is re-enabled once
// a previously invalid WORKFLOW.md is corrected.
func TestDispatchResumesAfterRecovery(t *testing.T) {
	path := writeWorkflow(t)
	tr := &fakeTracker{issues: []ptrackerv.Issue{
		{ID: "2", Identifier: "P-2", Title: "issue two", State: "In Progress"},
	}}
	spawn := &fakeSpawn{}
	s := New(path, schedCfg(), minDeps(tr, spawn.fn, newFakeClock(time.Now())))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Degrade: write bad config.
	writeFrontMatter(t, path, strings.ReplaceAll(validFrontMatter, "project_slug: test-proj\n", ""))
	s.tickOnce(ctx)
	if spawn.callCount() != 0 {
		t.Fatalf("want 0 spawn calls while degraded, got %d", spawn.callCount())
	}

	// Recover: restore valid config.
	writeFrontMatter(t, path, validFrontMatter)
	s.tickOnce(ctx)
	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn call after recovery, got %d", spawn.callCount())
	}
}

// TestDegradedWarnEmittedOnce verifies that the operator-visible Warn is emitted
// once when the workflow turns bad, and not repeated on subsequent bad ticks.
// Also verifies degraded resets to false on recovery.
func TestDegradedWarnEmittedOnce(t *testing.T) {
	cap := installWarnCapture(t)

	path := writeWorkflow(t)
	s := New(path, schedCfg(), Deps{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	badContent := strings.ReplaceAll(validFrontMatter, "project_slug: test-proj\n", "")
	writeFrontMatter(t, path, badContent)

	s.tickOnce(ctx)
	s.tickOnce(ctx)
	s.tickOnce(ctx)

	if got := cap.count(); got != 1 {
		t.Errorf("want exactly 1 warn on repeated bad reloads, got %d", got)
	}
	if !s.degraded {
		t.Error("want s.degraded == true while workflow is invalid")
	}

	// Restore valid config: degraded should clear.
	writeFrontMatter(t, path, validFrontMatter)
	s.tickOnce(ctx)
	if s.degraded {
		t.Error("want s.degraded == false after successful reload")
	}
}

// TestTickSpawnFailSchedulesRetry verifies spawn failure leads to retry scheduling.
func TestTickSpawnFailSchedulesRetry(t *testing.T) {
	tr := &fakeTracker{issues: []ptrackerv.Issue{
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

// TestTickRevalidationSkipsStaleIssue verifies that tickOnce skips an issue that went
// non-active between candidate fetch and dispatch (SPEC §16.4).
func TestTickRevalidationSkipsStaleIssue(t *testing.T) {
	tr := &fakeTracker{issues: []ptrackerv.Issue{
		{ID: "1", Identifier: "P-1", Title: "issue", State: "In Progress"},
	}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	// RefreshTracker returns the issue as "Done" — simulates state change between fetch and spawn.
	rt := &fakeReconcileTracker{
		refreshIssues: []ptrackerv.Issue{{ID: "1", Identifier: "P-1", Title: "issue", State: "Done"}},
	}

	s := New("", schedCfg(), Deps{
		Tracker:        tr,
		Spawn:          spawn.fn,
		Clock:          clk,
		RefreshTracker: rt,
	})
	s.workflowPath = writeWorkflow(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.tickOnce(ctx)

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns for stale issue, got %d", spawn.callCount())
	}
	snap := s.state.Snapshot()
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released for stale issue")
	}
}

// TestHandleCodexActivity_TurnCompleted_IncrementsTurnCount verifies that a
// CodexActivity with TurnCompleted=true increments the TurnCount in State (SPEC §4.1.6).
func TestHandleCodexActivity_TurnCompleted_IncrementsTurnCount(t *testing.T) {
	issue := ptrackerv.Issue{ID: "tc-1", Identifier: "TC-1", Title: "t", State: "In Progress"}
	session := LiveSession{SessionID: "s1"}

	s := New("", schedCfg(), Deps{})
	if err := s.state.Dispatch(issue, 1, session, time.Now()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	s.handleCodexActivity(CodexActivity{
		IssueID:       "tc-1",
		Event:         "turn/completed",
		Timestamp:     time.Now(),
		TurnCompleted: true,
	})
	s.handleCodexActivity(CodexActivity{
		IssueID:       "tc-1",
		Event:         "turn/completed",
		Timestamp:     time.Now(),
		TurnCompleted: true,
	})

	snap := s.state.Snapshot()
	if got := snap.Running["tc-1"].TurnCount; got != 2 {
		t.Errorf("got TurnCount=%d, want 2", got)
	}
}

// TestHandleCodexActivity_NonTurnCompleted_DoesNotIncrementTurnCount verifies that
// other events do not modify TurnCount.
func TestHandleCodexActivity_NonTurnCompleted_DoesNotIncrementTurnCount(t *testing.T) {
	issue := ptrackerv.Issue{ID: "tc-2", Identifier: "TC-2", Title: "t", State: "In Progress"}
	session := LiveSession{SessionID: "s2"}

	s := New("", schedCfg(), Deps{})
	if err := s.state.Dispatch(issue, 1, session, time.Now()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	s.handleCodexActivity(CodexActivity{
		IssueID:       "tc-2",
		Event:         "item/agentMessage/delta",
		Message:       "hello",
		Timestamp:     time.Now(),
		TurnCompleted: false,
	})

	snap := s.state.Snapshot()
	if got := snap.Running["tc-2"].TurnCount; got != 0 {
		t.Errorf("got TurnCount=%d, want 0 for non-turn-completed event", got)
	}
}
