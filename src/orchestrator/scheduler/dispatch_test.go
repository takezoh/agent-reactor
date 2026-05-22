package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

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
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg())

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
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg)

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
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg)

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
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg())

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

// TestDispatchOnce_SkipsRetryQueuedIssue verifies that a poll tick during the retry backoff
// window does not re-dispatch the same issue (SPEC §7.1/§7.4).
func TestDispatchOnce_SkipsRetryQueuedIssue(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	ctx := context.Background()

	iss := makeIssue("1", "In Progress")

	// Simulate a completed turn: worker ran, exited normally, retry is now queued.
	if err := st.Dispatch(iss, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	entry, ok := st.WorkerExitNormal("1")
	if !ok {
		t.Fatal("WorkerExitNormal returned false")
	}
	scheduleRetry(st, clk, fireCh, ctx, entry, continuationDelay)

	snap := st.Snapshot()
	if _, inRunning := snap.Running["1"]; inRunning {
		t.Fatal("precondition: issue must not be running during retry window")
	}
	if _, inRetry := snap.RetryAttempts["1"]; !inRetry {
		t.Fatal("precondition: issue must be in retry queue")
	}

	// Poll tick fires during retry window — must not re-dispatch.
	dispatchOnce(ctx, []tracker.Issue{iss}, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns during retry window, got %d", spawn.callCount())
	}
	snap = st.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue not in running during retry window")
	}
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want retry entry preserved after tick")
	}
}

// TestDispatchOnce_SkipsAbnormalExitRetry verifies the same guarantee for backoff retries.
func TestDispatchOnce_SkipsAbnormalExitRetry(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	ctx := context.Background()

	iss := makeIssue("2", "In Progress")

	if err := st.Dispatch(iss, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	entry, ok := st.WorkerExitAbnormal("2", errors.New("turn failed"), 1)
	if !ok {
		t.Fatal("WorkerExitAbnormal returned false")
	}
	scheduleRetry(st, clk, fireCh, ctx, entry, backoffDelay(entry.Attempt, dispCfg()))

	// Poll tick during backoff window must not re-dispatch.
	dispatchOnce(ctx, []tracker.Issue{iss}, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns during backoff window, got %d", spawn.callCount())
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

// TestHandleRetryFire_WithRetryAttemptEntry verifies that handleRetryFire dispatches
// correctly when the issue IS in retryAttempts (the normal timer-fire path).
// This is the regression test for the eligible() change: adding retryAttempts check to
// eligible() must not break the retry-fire dispatch path.
func TestHandleRetryFire_WithRetryAttemptEntry(t *testing.T) {
	st := NewState()
	// Populate retryAttempts as scheduleRetry would after a worker exit.
	st.EnqueueRetry(RetryEntry{IssueID: "1", Identifier: "P-1", Attempt: 2, Kind: RetryContinuation})
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn on retry fire, got %d", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Running["1"]; !ok {
		t.Error("want issue in running after retry fire dispatch")
	}
	if _, ok := snap.RetryAttempts["1"]; ok {
		t.Error("want retry entry consumed after dispatch")
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
