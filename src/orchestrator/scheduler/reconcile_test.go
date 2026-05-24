package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

// fakeWorker records Kill calls and implements Worker.
// killedCh allows tests to synchronize with goroutine-dispatched Kill calls.
type fakeWorker struct {
	mu       sync.Mutex
	killed   []string
	killErr  error
	killedCh chan string // buffered; receives each Kill reason as it fires
}

func newFakeWorker() *fakeWorker {
	return &fakeWorker{killedCh: make(chan string, 8)}
}

func (w *fakeWorker) Kill(reason string) error {
	w.mu.Lock()
	w.killed = append(w.killed, reason)
	err := w.killErr
	w.mu.Unlock()
	select {
	case w.killedCh <- reason:
	default:
	}
	return err
}

// expectKill waits up to 100 ms for Kill to be called and returns the reason.
// Use this instead of reading w.killed directly for async Kill paths.
func (w *fakeWorker) expectKill(t *testing.T) string {
	t.Helper()
	select {
	case r := <-w.killedCh:
		return r
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for Kill to be called")
		return ""
	}
}

// expectNoKill asserts that Kill was not called within a short window.
func (w *fakeWorker) expectNoKill(t *testing.T) {
	t.Helper()
	select {
	case r := <-w.killedCh:
		t.Errorf("unexpected Kill(%q)", r)
	case <-time.After(10 * time.Millisecond):
	}
}

// fakeReconcileTracker implements schedulerTrackerAPI.
type fakeReconcileTracker struct {
	refreshIssues  []ptrackerv.Issue
	refreshErr     error
	terminalIssues []ptrackerv.Issue
	terminalErr    error
}

func (f *fakeReconcileTracker) RefreshStates(_ context.Context, _ []string) ([]ptrackerv.Issue, error) {
	return f.refreshIssues, f.refreshErr
}

func (f *fakeReconcileTracker) TerminalIssues(_ context.Context) ([]ptrackerv.Issue, error) {
	return f.terminalIssues, f.terminalErr
}

// fakeWorkspace implements schedulerWorkspaceAPI.
type fakeWorkspace struct {
	removed []string
}

func (f *fakeWorkspace) Remove(_ context.Context, identifier string) error {
	f.removed = append(f.removed, identifier)
	return nil
}

// newReconcileScheduler builds a Scheduler with injected fakes for reconcile tests.
func newReconcileScheduler(tr schedulerTrackerAPI, ws schedulerWorkspaceAPI, clk Clock) *Scheduler {
	return &Scheduler{
		state:     NewState(),
		tracker:   tr,
		workspace: ws,
		clock:     clk,
		retryFire: make(chan retryFireReq, 16),
	}
}

// --- Stall tests ---

func TestReconcileStall_KillsAndEnqueuesRetry(t *testing.T) {
	w := newFakeWorker()
	s := newReconcileScheduler(nil, nil, newFakeClock(time.Unix(1000, 0)))

	started := time.Unix(0, 0)
	issue := testIssue("id1", "PROJ-1")
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, started)

	s.reconcile(context.Background(), stalledCfg(500))

	// Kill is dispatched in a goroutine; wait for it.
	if reason := w.expectKill(t); reason != "stall" {
		t.Errorf("expected Kill(stall), got %q", reason)
	}
	snap := s.state.Snapshot()
	if _, ok := snap.Running["id1"]; ok {
		t.Error("expected id1 removed from running after stall kill")
	}
	if _, ok := snap.RetryAttempts["id1"]; !ok {
		t.Error("expected retry entry enqueued")
	}
}

func TestReconcileStall_UsesStartedAtFallback(t *testing.T) {
	w := newFakeWorker()
	s := newReconcileScheduler(nil, nil, newFakeClock(time.Unix(1000, 0)))

	started := time.Unix(0, 0)
	issue := testIssue("id2", "PROJ-2")
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, started)
	// LastCodexTimestamp is zero — fallback to StartedAt

	s.reconcile(context.Background(), stalledCfg(500))

	// Kill is dispatched in a goroutine; wait for it.
	w.expectKill(t) // any reason is fine; just verify it fired
}

func TestReconcileStall_SkipsWhenTimeoutZero(t *testing.T) {
	w := newFakeWorker()
	s := newReconcileScheduler(nil, nil, newFakeClock(time.Unix(1000, 0)))

	started := time.Unix(0, 0)
	issue := testIssue("id3", "PROJ-3")
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, started)

	s.reconcile(context.Background(), stalledCfg(0)) // disabled

	if len(w.killed) != 0 {
		t.Error("expected no Kill when stall_timeout_ms=0")
	}
}

// --- Refresh tests ---

func TestReconcileRefresh_TerminalKillsAndCleansWorkspace(t *testing.T) {
	w := newFakeWorker()
	ws := &fakeWorkspace{}
	issue := testIssue("id4", "PROJ-4")
	refreshed := issue
	refreshed.State = "Done"

	tr := &fakeReconcileTracker{refreshIssues: []ptrackerv.Issue{refreshed}}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	s.reconcile(context.Background(), refreshCfg([]string{"Done"}, []string{"In Progress"}))

	// Kill is dispatched in a goroutine; wait for it.
	if reason := w.expectKill(t); reason != "terminal" {
		t.Errorf("expected Kill(terminal), got %q", reason)
	}
	snap := s.state.Snapshot()
	if _, ok := snap.Running["id4"]; ok {
		t.Error("expected id4 released from running")
	}
	if len(ws.removed) != 1 || ws.removed[0] != "PROJ-4" {
		t.Errorf("expected workspace Remove(PROJ-4), got %v", ws.removed)
	}
}

func TestReconcileRefresh_TerminalMatchIsCaseInsensitive(t *testing.T) {
	w := newFakeWorker()
	ws := &fakeWorkspace{}
	issue := testIssue("id4b", "PROJ-4B")
	refreshed := issue
	refreshed.State = "DONE" // tracker casing differs from config

	tr := &fakeReconcileTracker{refreshIssues: []ptrackerv.Issue{refreshed}}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	// config terminal state is lowercase "done" — must still match "DONE".
	s.reconcile(context.Background(), refreshCfg([]string{"done"}, []string{"in progress"}))

	// Kill is dispatched in a goroutine; wait for it.
	if reason := w.expectKill(t); reason != "terminal" {
		t.Errorf("expected Kill(terminal) on case-insensitive match, got %q", reason)
	}
	if len(ws.removed) != 1 || ws.removed[0] != "PROJ-4B" {
		t.Errorf("expected workspace Remove(PROJ-4B), got %v", ws.removed)
	}
}

func TestReconcileRefresh_ActiveUpdatesSnapshot(t *testing.T) {
	w := newFakeWorker()
	ws := &fakeWorkspace{}
	issue := testIssue("id5", "PROJ-5")
	refreshed := issue
	refreshed.State = "In Progress"
	refreshed.Title = "Updated title"

	tr := &fakeReconcileTracker{refreshIssues: []ptrackerv.Issue{refreshed}}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	s.reconcile(context.Background(), refreshCfg([]string{"Done"}, []string{"In Progress"}))

	if len(w.killed) != 0 {
		t.Error("expected no Kill for active issue")
	}
	if len(ws.removed) != 0 {
		t.Error("expected no workspace Remove for active issue")
	}
	snap := s.state.Snapshot()
	run, ok := snap.Running["id5"]
	if !ok {
		t.Fatal("expected id5 still running")
	}
	if run.Issue.Title != "Updated title" {
		t.Errorf("snapshot not updated: got %q", run.Issue.Title)
	}
}

func TestReconcileRefresh_IntermediateKillsNoWorkspaceRemove(t *testing.T) {
	w := newFakeWorker()
	ws := &fakeWorkspace{}
	issue := testIssue("id6", "PROJ-6")
	refreshed := issue
	refreshed.State = "Review" // neither terminal nor active

	tr := &fakeReconcileTracker{refreshIssues: []ptrackerv.Issue{refreshed}}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	s.reconcile(context.Background(), refreshCfg([]string{"Done"}, []string{"In Progress"}))

	// Kill is dispatched in a goroutine; wait for it.
	if reason := w.expectKill(t); reason != "non-active" {
		t.Errorf("expected Kill(non-active), got %q", reason)
	}
	if len(ws.removed) != 0 {
		t.Error("expected no workspace Remove for intermediate state")
	}
	snap := s.state.Snapshot()
	if _, ok := snap.RetryAttempts["id6"]; !ok {
		t.Error("expected retry entry for intermediate state")
	}
}

func TestReconcileRefresh_NotFoundKillsNoWorkspaceRemove(t *testing.T) {
	w := newFakeWorker()
	ws := &fakeWorkspace{}
	issue := testIssue("id8", "PROJ-8")

	// Tracker returns empty slice — id8 is absent from refresh response.
	tr := &fakeReconcileTracker{refreshIssues: []ptrackerv.Issue{}}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	s.reconcile(context.Background(), refreshCfg([]string{"Done"}, []string{"In Progress"}))

	// Kill is dispatched in a goroutine; wait for it.
	if reason := w.expectKill(t); reason != "not-found" {
		t.Errorf("expected Kill(not-found), got %q", reason)
	}
	if len(ws.removed) != 0 {
		t.Errorf("expected no workspace Remove for disappeared issue, got %v", ws.removed)
	}
	snap := s.state.Snapshot()
	if _, ok := snap.Running["id8"]; ok {
		t.Error("expected id8 removed from running after not-found stop")
	}
	if _, ok := snap.RetryAttempts["id8"]; !ok {
		t.Error("expected retry entry enqueued for disappeared issue")
	}
}

func TestReconcileRefresh_ErrorSkipsAll(t *testing.T) {
	w := newFakeWorker()
	ws := &fakeWorkspace{}
	issue := testIssue("id7", "PROJ-7")

	tr := &fakeReconcileTracker{refreshErr: errors.New("api down")}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	s.reconcile(context.Background(), refreshCfg([]string{"Done"}, []string{"In Progress"}))

	if len(w.killed) != 0 {
		t.Error("expected no Kill when RefreshStates fails")
	}
	snap := s.state.Snapshot()
	if _, ok := snap.Running["id7"]; !ok {
		t.Error("expected id7 still running after refresh failure")
	}
}

// --- helpers ---

func TestReconcileStall_RecentActivityPreventsKill(t *testing.T) {
	w := newFakeWorker()
	now := time.Unix(1000, 0)
	s := newReconcileScheduler(nil, nil, newFakeClock(now))

	// StartedAt is old enough to exceed the stall threshold.
	started := time.Unix(0, 0)
	issue := testIssue("id10", "PROJ-10")
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, started)

	// Record recent codex activity at "now" — must prevent kill.
	s.state.UpdateCodexActivity("id10", "turn/completed", "", now)

	s.reconcile(context.Background(), stalledCfg(500))

	if len(w.killed) != 0 {
		t.Errorf("worker killed despite recent codex activity: %v", w.killed)
	}
}

func stalledCfg(stallMS int) wfconfig.Config {
	cfg := wfconfig.Config{}
	cfg.Codex.StallTimeoutMS = stallMS
	return cfg
}

func refreshCfg(terminal, active []string) wfconfig.Config {
	cfg := wfconfig.Config{}
	cfg.Tracker.TerminalStates = terminal
	cfg.Tracker.ActiveStates = active
	cfg.Codex.StallTimeoutMS = 0 // disable stall for refresh tests
	return cfg
}
