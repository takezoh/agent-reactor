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

func TestDispatchOnce_FirstRunAttemptIsZero(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 1 {
		t.Fatalf("want 1 spawn, got %d", spawn.callCount())
	}
	if got := spawn.calls[0].Attempt; got != 0 {
		t.Errorf("first run: want attempt=0, got %d", got)
	}
	snap := st.Snapshot()
	if run, ok := snap.Running["1"]; !ok {
		t.Error("want issue 1 in running")
	} else if run.Attempt != 0 {
		t.Errorf("RunAttempt.Attempt: want 0, got %d", run.Attempt)
	}
}

func TestDispatchOnce_SpawnFail_FirstBackoff10s(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{err: errors.New("spawn error")}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cfg := dispCfg()
	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg)

	snap := st.Snapshot()
	entry, ok := snap.RetryAttempts["1"]
	if !ok {
		t.Fatal("want retry entry after spawn fail")
	}
	if entry.Attempt != 1 {
		t.Errorf("first retry: want attempt=1, got %d", entry.Attempt)
	}
	want10s := backoffDelay(1, cfg)
	gotDelayMS := entry.DueAtMS - clk.Now().UnixMilli()
	if gotDelayMS != want10s.Milliseconds() {
		t.Errorf("first backoff: want %dms (10s), got %dms", want10s.Milliseconds(), gotDelayMS)
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
