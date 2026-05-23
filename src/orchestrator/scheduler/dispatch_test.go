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

var errFetch = errors.New("fetch failed")

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

// setupRetryQueued drives a state machine through Dispatch → WorkerExitNormal → EnqueueRetry,
// leaving the issue in RetryQueued (claimed, in retryAttempts, not running).
func setupRetryQueued(t *testing.T, st *State, id string) {
	t.Helper()
	iss := makeIssue(id, "In Progress")
	if err := st.Dispatch(iss, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.WorkerExitNormal(id); !ok {
		t.Fatal("WorkerExitNormal failed")
	}
	st.EnqueueRetry(RetryEntry{IssueID: id, Identifier: "P-" + id, Attempt: 2, Kind: RetryContinuation})
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

func TestDispatchOnce_FirstRunAttemptIsZero(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), nil)

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
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg, nil)

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
// The issue must be in RetryQueued state (claimed + retryAttempts) before firing.
func TestHandleRetryFire_EligibleAndSlots(t *testing.T) {
	st := NewState()
	setupRetryQueued(t, st, "1")

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

func TestHandleRetryFire_FetchFailReschedules(t *testing.T) {
	st := NewState()
	tr := &fakeTracker{callErr: errFetch}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Identifier: "P-1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	if spawn.callCount() != 0 {
		t.Error("want no spawn on fetch failure")
	}
	snap := st.Snapshot()
	entry, ok := snap.RetryAttempts["1"]
	if !ok {
		t.Fatal("want retry rescheduled after fetch failure")
	}
	if entry.Attempt != 3 {
		t.Errorf("want attempt=3 (attempt+1), got %d", entry.Attempt)
	}
	if entry.Identifier != "P-1" {
		t.Errorf("want identifier=P-1 preserved, got %q", entry.Identifier)
	}
}

// TestDispatchOnce_RetryQueuedNotRedispatched is the §7.4 acceptance test:
// a tick during the retry backoff window must not re-dispatch the same issue.
func TestDispatchOnce_RetryQueuedNotRedispatched(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	setupRetryQueued(t, st, "1")

	// Simulate a poll tick while the issue is still in the retry window.
	cands := []tracker.Issue{makeIssue("1", "In Progress")}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, dispCfg(), nil)

	// The spawn must NOT be called: issue is still claimed (RetryQueued).
	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns during retry window, got %d (double-dispatch bug)", spawn.callCount())
	}
	snap := st.Snapshot()
	if _, ok := snap.Claimed["1"]; !ok {
		t.Error("want issue still claimed during retry window")
	}
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want issue still in retryAttempts during retry window")
	}
}

// TestHandleRetryFire_NoSlots requeues the issue with attempt+1 and backoff delay (SPEC §8.4).
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

	cfg := dispCfg()
	const reqAttempt = 2
	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: reqAttempt}, tr, st, clk, fireCh, spawn.fn, cfg)

	if spawn.callCount() != 0 {
		t.Error("want no spawn when slots exhausted")
	}
	snap := st.Snapshot()
	entry, ok := snap.RetryAttempts["1"]
	if !ok {
		t.Fatal("want requeue when no slots")
	}
	// SPEC §8.4: attempt must be incremented so backoff grows.
	wantAttempt := reqAttempt + 1
	if entry.Attempt != wantAttempt {
		t.Errorf("want attempt %d, got %d", wantAttempt, entry.Attempt)
	}
	// SPEC §8.4: must use failure backoff, not the 1s continuation delay.
	wantDelay := backoffDelay(wantAttempt, cfg)
	wantDueAtMS := clk.Now().Add(wantDelay).UnixMilli()
	if entry.DueAtMS != wantDueAtMS {
		t.Errorf("want DueAtMS %d (backoff=%v), got %d", wantDueAtMS, wantDelay, entry.DueAtMS)
	}
	// The error must indicate slot exhaustion, not be nil (which would mark it as a continuation).
	if entry.Err == nil {
		t.Error("want non-nil Err for slot-exhaustion requeue")
	}
}

// TestHandleRetryFire_NoSlots_FiresWithIncrementedAttempt verifies the timer fires with attempt+1.
func TestHandleRetryFire_NoSlots_FiresWithIncrementedAttempt(t *testing.T) {
	st := NewState()
	for i := range 3 {
		id := string(rune('a' + i))
		_ = st.Dispatch(makeIssue(id, "In Progress"), 1, LiveSession{}, time.Now())
	}
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	handleRetryFire(context.Background(), retryFireReq{IssueID: "1", Attempt: 2}, tr, st, clk, fireCh, spawn.fn, dispCfg())

	// Advance clock to trigger the scheduled timer.
	clk.Advance(backoffDelay(3, dispCfg()))

	select {
	case fired := <-fireCh:
		if fired.Attempt != 3 {
			t.Errorf("want fired attempt=3, got %d", fired.Attempt)
		}
	default:
		t.Error("want timer to fire after backoff delay")
	}
}
