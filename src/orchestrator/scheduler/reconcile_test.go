package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

// fakeWorker records Kill calls and implements Worker.
type fakeWorker struct {
	killed  []string
	killErr error
}

func (w *fakeWorker) Kill(reason string) error {
	w.killed = append(w.killed, reason)
	return w.killErr
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
	s := &Scheduler{
		state:     NewState(),
		tracker:   tr,
		workspace: ws,
		clock:     clk,
		retryFire: make(chan retryFireReq, 16),
	}
	s.deps.Reconcile = func(context.Context) error { return nil }
	return s
}

// --- Stall tests ---

func TestReconcileStall_KillsAndEnqueuesRetry(t *testing.T) {
	w := &fakeWorker{}
	s := newReconcileScheduler(&fakeReconcileTracker{}, &fakeWorkspace{}, newFakeClock(time.Unix(1000, 0)))

	started := time.Unix(0, 0)
	issue := testIssue("id1", "PROJ-1")
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, started)

	s.reconcile(context.Background(), stalledCfg(500))

	if len(w.killed) != 1 || w.killed[0] != "stall" {
		t.Errorf("expected Kill(stall), got %v", w.killed)
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
	w := &fakeWorker{}
	s := newReconcileScheduler(&fakeReconcileTracker{}, &fakeWorkspace{}, newFakeClock(time.Unix(1000, 0)))

	started := time.Unix(0, 0)
	issue := testIssue("id2", "PROJ-2")
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, started)
	// LastCodexTimestamp is zero — fallback to StartedAt

	s.reconcile(context.Background(), stalledCfg(500))

	if len(w.killed) == 0 {
		t.Error("expected worker killed via StartedAt fallback")
	}
}

func TestReconcileStall_SkipsWhenTimeoutZero(t *testing.T) {
	w := &fakeWorker{}
	s := newReconcileScheduler(&fakeReconcileTracker{}, &fakeWorkspace{}, newFakeClock(time.Unix(1000, 0)))

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
	w := &fakeWorker{}
	ws := &fakeWorkspace{}
	issue := testIssue("id4", "PROJ-4")
	refreshed := issue
	refreshed.State = "Done"

	tr := &fakeReconcileTracker{refreshIssues: []ptrackerv.Issue{refreshed}}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	s.reconcile(context.Background(), refreshCfg([]string{"Done"}, []string{"In Progress"}))

	if len(w.killed) != 1 || w.killed[0] != "terminal" {
		t.Errorf("expected Kill(terminal), got %v", w.killed)
	}
	snap := s.state.Snapshot()
	if _, ok := snap.Running["id4"]; ok {
		t.Error("expected id4 released from running")
	}
	if len(ws.removed) != 1 || ws.removed[0] != "PROJ-4" {
		t.Errorf("expected workspace Remove(PROJ-4), got %v", ws.removed)
	}
}

func TestReconcileRefresh_ActiveUpdatesSnapshot(t *testing.T) {
	w := &fakeWorker{}
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
	w := &fakeWorker{}
	ws := &fakeWorkspace{}
	issue := testIssue("id6", "PROJ-6")
	refreshed := issue
	refreshed.State = "Review" // neither terminal nor active

	tr := &fakeReconcileTracker{refreshIssues: []ptrackerv.Issue{refreshed}}
	s := newReconcileScheduler(tr, ws, newFakeClock(time.Now()))
	_ = s.state.Dispatch(issue, 1, LiveSession{Worker: w}, time.Now())

	s.reconcile(context.Background(), refreshCfg([]string{"Done"}, []string{"In Progress"}))

	if len(w.killed) != 1 || w.killed[0] != "non-active" {
		t.Errorf("expected Kill(non-active), got %v", w.killed)
	}
	if len(ws.removed) != 0 {
		t.Error("expected no workspace Remove for intermediate state")
	}
	snap := s.state.Snapshot()
	if _, ok := snap.RetryAttempts["id6"]; !ok {
		t.Error("expected retry entry for intermediate state")
	}
}

func TestReconcileRefresh_ErrorSkipsAll(t *testing.T) {
	w := &fakeWorker{}
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
