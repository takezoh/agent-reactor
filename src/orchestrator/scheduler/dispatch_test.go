package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// fakeRevalidator implements IssueRevalidator for dispatch tests.
type fakeRevalidator struct {
	mu      sync.Mutex
	issues  []tracker.Issue
	callErr error
	calls   int
}

func (f *fakeRevalidator) RefreshStates(_ context.Context, ids []string) ([]tracker.Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.callErr != nil {
		return nil, f.callErr
	}
	var out []tracker.Issue
	for _, iss := range f.issues {
		for _, id := range ids {
			if iss.ID == id {
				out = append(out, iss)
			}
		}
	}
	return out, nil
}

func dispCfg() wfconfig.Config {
	return wfconfig.Config{
		Tracker: wfconfig.TrackerConfig{
			ActiveStates:   []string{"In Progress", "Todo"},
			TerminalStates: []string{"Done"},
		},
		Agent: wfconfig.AgentConfig{
			MaxConcurrentAgents: 3,
			MaxRetryBackoffMS:   60_000,
		},
	}
}

func makeIssue(id, state string) tracker.Issue {
	return tracker.Issue{ID: id, Identifier: "P-" + id, Title: "t", State: state}
}

// TestDispatchOnce_EligibleIssueSpawned verifies a basic eligible dispatch.
func TestDispatchOnce_EligibleIssueSpawned(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), nil)

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; !ok {
		t.Error("want issue 1 in running")
	}
}

// TestDispatchOnce_GlobalSlotsCap verifies only MaxConcurrentAgents issues are dispatched.
func TestDispatchOnce_GlobalSlotsCap(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
		makeIssue("3", "In Progress"),
		makeIssue("4", "In Progress"),
	}
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgents = 2
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg, nil)

	if spawn.callCount() != 2 {
		t.Errorf("want 2 spawns (global cap), got %d", spawn.callCount())
	}
}

// TestDispatchOnce_PerStateSlotsCap verifies per-state limits are respected.
func TestDispatchOnce_PerStateSlotsCap(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
		makeIssue("3", "Todo"),
	}
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgentsByState = map[string]int{"in progress": 1}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg, nil)

	snap := st.Snapshot()
	// "In Progress" cap=1, "Todo" uses global (3) — so 1 + 1 = 2 total
	if spawn.callCount() != 2 {
		t.Errorf("want 2 spawns, got %d", spawn.callCount())
	}
	if _, ok := snap.Running["3"]; !ok {
		t.Error("want todo issue dispatched")
	}
}

// TestDispatchOnce_SpawnFailSchedulesRetry verifies spawn failure leads to retry.
func TestDispatchOnce_SpawnFailSchedulesRetry(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{err: errors.New("oops")}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), nil)

	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue 1 not in running after spawn fail")
	}
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released after spawn fail")
	}
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want retry entry after spawn fail")
	}
}

// --- Revalidation tests (SPEC §16.4) ---

// TestDispatchOnce_RevalidationActiveProceeds verifies that an issue confirmed active is spawned.
func TestDispatchOnce_RevalidationActiveProceeds(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	rv := &fakeRevalidator{issues: []tracker.Issue{makeIssue("1", "In Progress")}}

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), rv)

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn for active issue, got %d", spawn.callCount())
	}
	if rv.calls != 1 {
		t.Errorf("want 1 revalidation call, got %d", rv.calls)
	}
}

// TestDispatchOnce_RevalidationStaleSkipped verifies a stale (non-active) issue is skipped.
func TestDispatchOnce_RevalidationStaleSkipped(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	// Issue transitioned to "Done" between candidate fetch and dispatch.
	rv := &fakeRevalidator{issues: []tracker.Issue{makeIssue("1", "Done")}}

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), rv)

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns for stale issue, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released after stale revalidation")
	}
}

// TestDispatchOnce_RevalidationMissingSkipped verifies a missing issue is skipped.
func TestDispatchOnce_RevalidationMissingSkipped(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	// Empty response — issue not found.
	rv := &fakeRevalidator{issues: []tracker.Issue{}}

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), rv)

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns for missing issue, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released for missing issue")
	}
}

// TestDispatchOnce_RevalidationErrorSkipped verifies a revalidation error causes skip and claim release.
func TestDispatchOnce_RevalidationErrorSkipped(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	rv := &fakeRevalidator{callErr: errors.New("tracker unavailable")}

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), rv)

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns on revalidation error, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released after revalidation error")
	}
}

// TestDispatchOnce_RevalidationPartial verifies only revalidated-active issues are spawned
// when multiple candidates exist but some are stale.
func TestDispatchOnce_RevalidationPartial(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	// Issue "2" went terminal; issue "1" is still active.
	rv := &fakeRevalidator{issues: []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "Done"),
	}}

	cands := []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
	}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), rv)

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn (only active issue), got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; !ok {
		t.Error("want issue 1 running")
	}
	if _, ok := snap.Claimed["2"]; ok {
		t.Error("want claim released for stale issue 2")
	}
}

// TestHandleRetryFire_IssueNotFound releases the claim.
func TestHandleRetryFire_IssueNotFound(t *testing.T) {
	st := NewState()
	// Put issue in RetryAttempts to simulate retry state.
	st.EnqueueRetry(RetryEntry{IssueID: "1", Identifier: "P-1"})
	tr := &fakeTracker{issues: []tracker.Issue{}} // empty — not found
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	snap := st.Snapshot()
	if _, ok := snap.RetryAttempts["1"]; ok {
		t.Error("want retry cleared after not-found release")
	}
}

// TestHandleRetryFire_NotActive releases the claim.
func TestHandleRetryFire_NotActive(t *testing.T) {
	st := NewState()
	st.EnqueueRetry(RetryEntry{IssueID: "1"})
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "Done")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 0 {
		t.Error("want no spawn for non-active issue")
	}
}

// TestHandleRetryFire_EligibleAndSlots dispatches and marks running.
func TestHandleRetryFire_EligibleAndSlots(t *testing.T) {
	st := NewState()
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; !ok {
		t.Error("want issue in running after retry dispatch")
	}
}

// TestHandleRetryFire_NoSlots requeues the issue.
func TestHandleRetryFire_NoSlots(t *testing.T) {
	st := NewState()
	// Fill all global slots.
	for i := range 3 {
		id := string(rune('a' + i))
		iss := makeIssue(id, "In Progress")
		_ = st.Dispatch(iss, 1, LiveSession{}, time.Now())
	}
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 0 {
		t.Error("want no spawn when slots exhausted")
	}
	snap := st.Snapshot()
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want requeue when no slots")
	}
}
