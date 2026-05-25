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

// fakeRevalidator implements schedulerTrackerAPI for dispatch revalidation tests.
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

func (f *fakeRevalidator) TerminalIssues(_ context.Context) ([]tracker.Issue, error) {
	return nil, nil
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

// dispatchScheduler builds a scheduler for driving the dispatch path directly via step.
// revalidator may be nil (then EffRevalidate passes through without an I/O re-check).
func dispatchScheduler(cfg wfconfig.Config, spawn SpawnFunc, clk Clock, revalidator schedulerTrackerAPI) *Scheduler {
	return New("", cfg, "", Deps{Spawn: spawn, Clock: clk, RefreshTracker: revalidator})
}

// TestDispatch_EligibleIssueSpawned verifies a basic eligible dispatch.
func TestDispatch_EligibleIssueSpawned(t *testing.T) {
	spawn := &fakeSpawn{}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), nil)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn, got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Running["1"]; !ok {
		t.Error("want issue 1 in running")
	}
}

// TestDispatch_GlobalSlotsCap verifies only MaxConcurrentAgents issues are dispatched.
func TestDispatch_GlobalSlotsCap(t *testing.T) {
	spawn := &fakeSpawn{}
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgents = 2
	s := dispatchScheduler(cfg, spawn.fn, newFakeClock(time.Now()), nil)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
		makeIssue("3", "In Progress"),
		makeIssue("4", "In Progress"),
	}})

	if spawn.callCount() != 2 {
		t.Errorf("want 2 spawns (global cap), got %d", spawn.callCount())
	}
}

// TestDispatch_PerStateSlotsCap verifies per-state limits are respected.
func TestDispatch_PerStateSlotsCap(t *testing.T) {
	spawn := &fakeSpawn{}
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgentsByState = map[string]int{"in progress": 1}
	s := dispatchScheduler(cfg, spawn.fn, newFakeClock(time.Now()), nil)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
		makeIssue("3", "Todo"),
	}})

	// "In Progress" cap=1, "Todo" uses global (3) — so 1 + 1 = 2 total.
	if spawn.callCount() != 2 {
		t.Errorf("want 2 spawns, got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Running["3"]; !ok {
		t.Error("want todo issue dispatched")
	}
}

// TestDispatch_SpawnFailSchedulesRetry verifies spawn failure leads to retry.
func TestDispatch_SpawnFailSchedulesRetry(t *testing.T) {
	spawn := &fakeSpawn{err: errors.New("oops")}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), nil)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	snap := s.Snapshot()
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

func TestDispatch_RevalidationActiveProceeds(t *testing.T) {
	spawn := &fakeSpawn{}
	rv := &fakeRevalidator{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), rv)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn for active issue, got %d", spawn.callCount())
	}
	if rv.calls != 1 {
		t.Errorf("want 1 revalidation call, got %d", rv.calls)
	}
}

func TestDispatch_RevalidationStaleSkipped(t *testing.T) {
	spawn := &fakeSpawn{}
	// Issue transitioned to "Done" between candidate fetch and dispatch.
	rv := &fakeRevalidator{issues: []tracker.Issue{makeIssue("1", "Done")}}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), rv)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns for stale issue, got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Claimed["1"]; ok {
		t.Error("want claim released after stale revalidation")
	}
}

func TestDispatch_RevalidationMissingSkipped(t *testing.T) {
	spawn := &fakeSpawn{}
	rv := &fakeRevalidator{issues: []tracker.Issue{}} // not found
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), rv)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns for missing issue, got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Claimed["1"]; ok {
		t.Error("want claim released for missing issue")
	}
}

func TestDispatch_RevalidationErrorSkipped(t *testing.T) {
	spawn := &fakeSpawn{}
	rv := &fakeRevalidator{callErr: errors.New("tracker unavailable")}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), rv)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns on revalidation error, got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Claimed["1"]; ok {
		t.Error("want claim released after revalidation error")
	}
}

func TestDispatch_RevalidationPartial(t *testing.T) {
	spawn := &fakeSpawn{}
	// Issue "2" went terminal; issue "1" is still active.
	rv := &fakeRevalidator{issues: []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "Done"),
	}}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), rv)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{
		makeIssue("1", "In Progress"),
		makeIssue("2", "In Progress"),
	}})

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn (only active issue), got %d", spawn.callCount())
	}
	snap := s.Snapshot()
	if _, ok := snap.Running["1"]; !ok {
		t.Error("want issue 1 running")
	}
	if _, ok := snap.Claimed["2"]; ok {
		t.Error("want claim released for stale issue 2")
	}
}

func TestDispatch_FirstRunAttemptIsZero(t *testing.T) {
	spawn := &fakeSpawn{}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), nil)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	if spawn.callCount() != 1 {
		t.Fatalf("want 1 spawn, got %d", spawn.callCount())
	}
	if got := spawn.calls[0].Attempt; got != 0 {
		t.Errorf("first run: want attempt=0, got %d", got)
	}
	if run, ok := s.Snapshot().Running["1"]; !ok {
		t.Error("want issue 1 in running")
	} else if run.Attempt != 0 {
		t.Errorf("RunAttempt.Attempt: want 0, got %d", run.Attempt)
	}
}

func TestDispatch_SpawnFail_FirstBackoff10s(t *testing.T) {
	spawn := &fakeSpawn{err: errors.New("spawn error")}
	clk := newFakeClock(time.Now())
	cfg := dispCfg()
	s := dispatchScheduler(cfg, spawn.fn, clk, nil)

	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	entry, ok := s.Snapshot().RetryAttempts["1"]
	if !ok {
		t.Fatal("want retry entry after spawn fail")
	}
	if entry.Attempt != 1 {
		t.Errorf("first retry: want attempt=1, got %d", entry.Attempt)
	}
	want10s := backoffDelay(1, cfg)
	if gotDelayMS := entry.DueAtMS - clk.Now().UnixMilli(); gotDelayMS != want10s.Milliseconds() {
		t.Errorf("first backoff: want %dms (10s), got %dms", want10s.Milliseconds(), gotDelayMS)
	}
}

// --- Retry-fire tests (EvRetryDue → EffRetryFetch → EvRetryResolved) ---

// retryScheduler builds a scheduler whose Tracker yields the given candidates on retry fetch.
func retryScheduler(cfg wfconfig.Config, tr CandidateSource, spawn SpawnFunc, clk Clock) *Scheduler {
	return New("", cfg, "", Deps{Tracker: tr, Spawn: spawn, Clock: clk})
}

func TestRetryFire_IssueNotFound(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{}} // empty — not found
	spawn := &fakeSpawn{}
	s := retryScheduler(dispCfg(), tr, spawn.fn, newFakeClock(time.Now()))
	s.cur = enqueueRetry(s.cur, RetryEntry{IssueID: "1", Identifier: "P-1"})
	s.publish()

	s.step(context.Background(), EvRetryDue{IssueID: "1", Attempt: 2})

	if _, ok := s.Snapshot().RetryAttempts["1"]; ok {
		t.Error("want retry cleared after not-found release")
	}
}

func TestRetryFire_NotActive(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "Done")}}
	spawn := &fakeSpawn{}
	s := retryScheduler(dispCfg(), tr, spawn.fn, newFakeClock(time.Now()))
	s.cur = enqueueRetry(s.cur, RetryEntry{IssueID: "1"})
	s.publish()

	s.step(context.Background(), EvRetryDue{IssueID: "1", Attempt: 2})

	if spawn.callCount() != 0 {
		t.Error("want no spawn for non-active issue")
	}
}

func TestRetryFire_EligibleAndSlots(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	s := retryScheduler(dispCfg(), tr, spawn.fn, newFakeClock(time.Now()))
	s.seedRetryQueued(makeIssue("1", "In Progress"))

	s.step(context.Background(), EvRetryDue{IssueID: "1", Identifier: "P-1", Attempt: 2})

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn, got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Running["1"]; !ok {
		t.Error("want issue in running after retry dispatch")
	}
}

func TestRetryFire_FetchFailReschedules(t *testing.T) {
	tr := &fakeTracker{callErr: errFetch}
	spawn := &fakeSpawn{}
	s := retryScheduler(dispCfg(), tr, spawn.fn, newFakeClock(time.Now()))

	s.step(context.Background(), EvRetryDue{IssueID: "1", Identifier: "P-1", Attempt: 2})

	if spawn.callCount() != 0 {
		t.Error("want no spawn on fetch failure")
	}
	entry, ok := s.Snapshot().RetryAttempts["1"]
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

// TestDispatch_RetryQueuedNotRedispatched is the §7.4 acceptance test: a tick during the
// retry backoff window must not re-dispatch the same issue.
func TestDispatch_RetryQueuedNotRedispatched(t *testing.T) {
	spawn := &fakeSpawn{}
	s := dispatchScheduler(dispCfg(), spawn.fn, newFakeClock(time.Now()), nil)
	s.seedRetryQueued(makeIssue("1", "In Progress"))

	// Simulate a poll tick while the issue is still in the retry window.
	s.step(context.Background(), EvCandidatesFetched{Issues: []tracker.Issue{makeIssue("1", "In Progress")}})

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns during retry window, got %d (double-dispatch bug)", spawn.callCount())
	}
	snap := s.Snapshot()
	if _, ok := snap.Claimed["1"]; !ok {
		t.Error("want issue still claimed during retry window")
	}
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want issue still in retryAttempts during retry window")
	}
}

// TestRetryFire_NoSlots requeues the issue with attempt+1 and backoff delay (SPEC §8.4).
func TestRetryFire_NoSlots(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	cfg := dispCfg()
	s := retryScheduler(cfg, tr, spawn.fn, clk)
	// Fill all global slots.
	for i := range 3 {
		id := string(rune('a' + i))
		s.seedRunning(makeIssue(id, "In Progress"), time.Now(), nil)
	}

	const reqAttempt = 2
	s.step(context.Background(), EvRetryDue{IssueID: "1", Attempt: reqAttempt})

	if spawn.callCount() != 0 {
		t.Error("want no spawn when slots exhausted")
	}
	entry, ok := s.Snapshot().RetryAttempts["1"]
	if !ok {
		t.Fatal("want requeue when no slots")
	}
	wantAttempt := reqAttempt + 1
	if entry.Attempt != wantAttempt {
		t.Errorf("want attempt %d, got %d", wantAttempt, entry.Attempt)
	}
	wantDelay := backoffDelay(wantAttempt, cfg)
	if wantDueAtMS := clk.Now().Add(wantDelay).UnixMilli(); entry.DueAtMS != wantDueAtMS {
		t.Errorf("want DueAtMS %d (backoff=%v), got %d", wantDueAtMS, wantDelay, entry.DueAtMS)
	}
	if entry.Err == nil {
		t.Error("want non-nil Err for slot-exhaustion requeue")
	}
}

// TestRetryFire_NoSlots_FiresWithIncrementedAttempt verifies the requeued timer fires with attempt+1.
func TestRetryFire_NoSlots_FiresWithIncrementedAttempt(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	s := retryScheduler(dispCfg(), tr, spawn.fn, clk)
	for i := range 3 {
		id := string(rune('a' + i))
		s.seedRunning(makeIssue(id, "In Progress"), time.Now(), nil)
	}

	s.step(context.Background(), EvRetryDue{IssueID: "1", Attempt: 2})

	clk.Advance(backoffDelay(3, dispCfg()))

	select {
	case fired := <-s.retryFire:
		if fired.Attempt != 3 {
			t.Errorf("want fired attempt=3, got %d", fired.Attempt)
		}
	default:
		t.Error("want timer to fire after backoff delay")
	}
}

// TestRetryFire_SingleSlotReclaim verifies a RetryQueued issue re-dispatches into the slot it
// already holds, even at max_concurrent_agents=1. Regression guard: the firing issue used to
// count its own RetryQueued reservation against the global limit and starve forever (§7.1/§8.3).
func TestRetryFire_SingleSlotReclaim(t *testing.T) {
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgents = 1
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	s := retryScheduler(cfg, tr, spawn.fn, newFakeClock(time.Now()))
	s.seedRetryQueued(makeIssue("1", "In Progress"))

	s.step(context.Background(), EvRetryDue{IssueID: "1", Identifier: "P-1", Attempt: 2})

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn (issue reclaims its own slot at cap=1), got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Running["1"]; !ok {
		t.Error("want issue running after retry reclaim")
	}
}

// TestRetryFire_NoSlotWhenOtherRunning verifies the reclaim is still blocked when a different
// issue occupies the only slot — the fix must not let a retry preempt a running agent.
func TestRetryFire_NoSlotWhenOtherRunning(t *testing.T) {
	cfg := dispCfg()
	cfg.Agent.MaxConcurrentAgents = 1
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	s := retryScheduler(cfg, tr, spawn.fn, newFakeClock(time.Now()))
	s.seedRunning(makeIssue("other", "In Progress"), time.Now(), nil) // occupies the only slot
	s.seedRetryQueued(makeIssue("1", "In Progress"))

	s.step(context.Background(), EvRetryDue{IssueID: "1", Identifier: "P-1", Attempt: 2})

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns (slot occupied by a running agent), got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().RetryAttempts["1"]; !ok {
		t.Error("want issue requeued when the slot is genuinely occupied")
	}
}
