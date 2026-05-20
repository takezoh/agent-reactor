package scheduler

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/platform/metrics"
	"github.com/takezoh/agent-roost/platform/tracker"
)

func testIssue(id, identifier string) tracker.Issue {
	return tracker.Issue{ID: id, Identifier: identifier, Title: "t"}
}

func TestStateDispatch_AddsToRunningAndClaimed(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	session := LiveSession{SessionID: "s1"}

	if err := s.Dispatch(issue, 1, session, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := s.Snapshot()
	if _, ok := snap.Claimed["id1"]; !ok {
		t.Error("expected id1 in claimed")
	}
	run, ok := snap.Running["id1"]
	if !ok {
		t.Fatal("expected id1 in running")
	}
	if run.Issue.Identifier != "PROJ-1" {
		t.Errorf("got identifier %q, want PROJ-1", run.Issue.Identifier)
	}
	if run.Attempt != 1 {
		t.Errorf("got attempt %d, want 1", run.Attempt)
	}
}

func TestStateDispatch_DuplicateRejected(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	session := LiveSession{SessionID: "s1"}

	if err := s.Dispatch(issue, 1, session, time.Now()); err != nil {
		t.Fatalf("first dispatch: %v", err)
	}
	err := s.Dispatch(issue, 2, session, time.Now())
	if !errors.Is(err, ErrDuplicateDispatch) {
		t.Errorf("expected ErrDuplicateDispatch, got %v", err)
	}
}

func TestStateDispatch_ClearsPriorRetry(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	s.EnqueueRetry(RetryEntry{IssueID: "id1", Kind: RetryBackoff})

	if err := s.Dispatch(issue, 2, LiveSession{}, time.Now()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	snap := s.Snapshot()
	if _, ok := snap.RetryAttempts["id1"]; ok {
		t.Error("expected retry entry cleared on Dispatch")
	}
}

func TestStateWorkerExitNormal_RemovesRunningAndReturnsContinuation(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	entry, ok := s.WorkerExitNormal("id1")
	if !ok {
		t.Fatal("expected WorkerExitNormal to return ok=true")
	}
	if entry.Kind != RetryContinuation {
		t.Errorf("got kind %v, want RetryContinuation", entry.Kind)
	}
	if entry.Attempt != 1 {
		t.Errorf("got attempt %d, want 1", entry.Attempt)
	}

	snap := s.Snapshot()
	if _, ok := snap.Running["id1"]; ok {
		t.Error("expected id1 removed from running")
	}
}

func TestStateWorkerExitAbnormal_RemovesRunningAndReturnsBackoff(t *testing.T) {
	s := NewState()
	issue := testIssue("id2", "PROJ-2")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	someErr := errors.New("agent crashed")
	entry, ok := s.WorkerExitAbnormal("id2", someErr, 1)
	if !ok {
		t.Fatal("expected WorkerExitAbnormal to return ok=true")
	}
	if entry.Kind != RetryBackoff {
		t.Errorf("got kind %v, want RetryBackoff", entry.Kind)
	}
	if entry.Attempt != 2 {
		t.Errorf("got attempt %d, want 2 (attempt+1)", entry.Attempt)
	}
	if !errors.Is(entry.Err, someErr) {
		t.Errorf("got err %v, want someErr", entry.Err)
	}

	snap := s.Snapshot()
	if _, ok := snap.Running["id2"]; ok {
		t.Error("expected id2 removed from running")
	}
}

func TestStateReleaseClaim_RemovesFromAllMaps(t *testing.T) {
	s := NewState()
	issue := testIssue("id3", "PROJ-3")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.EnqueueRetry(RetryEntry{IssueID: "id3"})

	s.ReleaseClaim("id3")

	snap := s.Snapshot()
	if _, ok := snap.Running["id3"]; ok {
		t.Error("expected id3 removed from running")
	}
	if _, ok := snap.Claimed["id3"]; ok {
		t.Error("expected id3 removed from claimed")
	}
	if _, ok := snap.RetryAttempts["id3"]; ok {
		t.Error("expected id3 removed from retryAttempts")
	}
}

func TestStateSnapshot_IsDeepCopy(t *testing.T) {
	s := NewState()
	issue := testIssue("id4", "PROJ-4")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	snap := s.Snapshot()
	delete(snap.Running, "id4")
	delete(snap.Claimed, "id4")

	snap2 := s.Snapshot()
	if _, ok := snap2.Running["id4"]; !ok {
		t.Error("State.running was mutated by modifying Snapshot")
	}
	if _, ok := snap2.Claimed["id4"]; !ok {
		t.Error("State.claimed was mutated by modifying Snapshot")
	}
}

func TestStateClaim_AddsToClaimedOnly(t *testing.T) {
	s := NewState()
	issue := testIssue("id10", "PROJ-10")

	if err := s.Claim(issue, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := s.Snapshot()
	if _, ok := snap.Claimed["id10"]; !ok {
		t.Error("expected id10 in claimed")
	}
	if _, ok := snap.Running["id10"]; ok {
		t.Error("expected id10 NOT in running after Claim")
	}
}

func TestStateMarkRunning_PromotesToRunning(t *testing.T) {
	s := NewState()
	issue := testIssue("id11", "PROJ-11")
	session := LiveSession{SessionID: "s11"}

	if err := s.Claim(issue, 2); err != nil {
		t.Fatalf("claim: %v", err)
	}
	s.MarkRunning(issue.ID, issue, 2, session, time.Now())

	snap := s.Snapshot()
	if _, ok := snap.Claimed["id11"]; !ok {
		t.Error("expected id11 still in claimed")
	}
	run, ok := snap.Running["id11"]
	if !ok {
		t.Fatal("expected id11 in running")
	}
	if run.Attempt != 2 {
		t.Errorf("got attempt %d, want 2", run.Attempt)
	}
}

func TestStateClaim_DuplicateRejected(t *testing.T) {
	s := NewState()
	issue := testIssue("id12", "PROJ-12")

	if err := s.Claim(issue, 1); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if err := s.Claim(issue, 2); !errors.Is(err, ErrDuplicateDispatch) {
		t.Errorf("expected ErrDuplicateDispatch, got %v", err)
	}
}

func TestStateConcurrentDispatch_NoDuplicate(t *testing.T) {
	s := NewState()
	issue := testIssue("id5", "PROJ-5")

	const goroutines = 20
	results := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			results[idx] = s.Dispatch(issue, 1, LiveSession{}, time.Now())
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrDuplicateDispatch) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful Dispatch, got %d", successes)
	}
}

func TestUpdateIssueSnapshot_UpdatesRunningIssue(t *testing.T) {
	s := NewState()
	issue := testIssue("id6", "PROJ-6")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	updated := testIssue("id6", "PROJ-6")
	updated.State = "In Review"
	s.UpdateIssueSnapshot("id6", updated)

	snap := s.Snapshot()
	run, ok := snap.Running["id6"]
	if !ok {
		t.Fatal("expected id6 in running")
	}
	if run.Issue.State != "In Review" {
		t.Errorf("got state %q, want In Review", run.Issue.State)
	}
}

func TestUpdateIssueSnapshot_NoopForUnknownID(t *testing.T) {
	s := NewState()
	s.UpdateIssueSnapshot("unknown", testIssue("unknown", "X-1"))
	snap := s.Snapshot()
	if len(snap.Running) != 0 {
		t.Error("expected running to remain empty")
	}
}

func TestUpdateCodexActivity_SetsFields(t *testing.T) {
	s := NewState()
	issue := testIssue("id20", "PROJ-20")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	ts := time.Unix(1234567890, 0)
	s.UpdateCodexActivity("id20", "turn/completed", "hello", ts)

	snap := s.Snapshot()
	run := snap.Running["id20"]
	if run.LastCodexEvent != "turn/completed" {
		t.Errorf("got event %q, want turn/completed", run.LastCodexEvent)
	}
	if !run.LastCodexTimestamp.Equal(ts) {
		t.Errorf("got ts %v, want %v", run.LastCodexTimestamp, ts)
	}
	if run.LastCodexMessage != "hello" {
		t.Errorf("got message %q, want hello", run.LastCodexMessage)
	}
}

func TestUpdateCodexActivity_EmptyMessageNotOverwritten(t *testing.T) {
	s := NewState()
	issue := testIssue("id21", "PROJ-21")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.UpdateCodexActivity("id21", "m1", "initial", time.Now())
	s.UpdateCodexActivity("id21", "m2", "", time.Now())
	snap := s.Snapshot()
	if snap.Running["id21"].LastCodexMessage != "initial" {
		t.Errorf("empty message overwrote existing: got %q", snap.Running["id21"].LastCodexMessage)
	}
}

func TestUpdateCodexActivity_NoopForUnknownID(t *testing.T) {
	s := NewState()
	s.UpdateCodexActivity("unknown", "e", "m", time.Now()) // must not panic
}

func TestRecordUsage_AggregatesAcrossReports(t *testing.T) {
	s := NewState()
	issue := testIssue("id22", "PROJ-22")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	s.RecordUsage("id22", metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	snap := s.Snapshot()
	run := snap.Running["id22"]
	if run.TotalInputTokens != 100 || run.TotalOutputTokens != 50 || run.TotalTokens != 150 {
		t.Errorf("after first report: got %d/%d/%d, want 100/50/150",
			run.TotalInputTokens, run.TotalOutputTokens, run.TotalTokens)
	}

	// Second report cumulative 250/120/370 — only the delta is added (no double-count).
	s.RecordUsage("id22", metrics.Usage{ThreadID: "t1", Input: 250, Output: 120, Total: 370})
	snap = s.Snapshot()
	run = snap.Running["id22"]
	if run.TotalInputTokens != 250 || run.TotalOutputTokens != 120 || run.TotalTokens != 370 {
		t.Errorf("after second report: got %d/%d/%d, want 250/120/370",
			run.TotalInputTokens, run.TotalOutputTokens, run.TotalTokens)
	}
}

func TestRecordUsage_NoopForUnknownID(t *testing.T) {
	s := NewState()
	s.RecordUsage("unknown", metrics.Usage{ThreadID: "t1", Input: 100}) // must not panic
}

func TestRecordUsage_AccumulatorCleanedUpOnExit(t *testing.T) {
	s := NewState()
	issue := testIssue("id23", "PROJ-23")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.RecordUsage("id23", metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	s.WorkerExitNormal("id23")

	// Re-dispatch: accumulator must start fresh (no stale lastSeen).
	if err := s.Dispatch(issue, 2, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.RecordUsage("id23", metrics.Usage{ThreadID: "t1", Input: 50, Output: 20, Total: 70})
	snap := s.Snapshot()
	run := snap.Running["id23"]
	if run.TotalInputTokens != 50 || run.TotalOutputTokens != 20 || run.TotalTokens != 70 {
		t.Errorf("stale accumulator: got %d/%d/%d, want 50/20/70",
			run.TotalInputTokens, run.TotalOutputTokens, run.TotalTokens)
	}
}

func TestRecordRateLimit_SetsField(t *testing.T) {
	s := NewState()
	issue := testIssue("id24", "PROJ-24")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	s.RecordRateLimit("id24", metrics.RateLimitSnapshot{PrimaryUsedPercent: 75, PrimaryResetsAt: 999})
	snap := s.Snapshot()
	run := snap.Running["id24"]
	if run.RateLimit == nil {
		t.Fatal("expected RateLimit to be set")
	}
	if run.RateLimit.PrimaryUsedPercent != 75 {
		t.Errorf("got %d, want 75", run.RateLimit.PrimaryUsedPercent)
	}
}

func TestAddRuntime_Accumulates(t *testing.T) {
	s := NewState()
	issue := testIssue("id25", "PROJ-25")
	if err := s.Dispatch(issue, 1, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}

	s.AddRuntime("id25", 2*time.Second)
	s.AddRuntime("id25", 3*time.Second)
	snap := s.Snapshot()
	if got := snap.Running["id25"].TotalRuntime; got != 5*time.Second {
		t.Errorf("got %v, want 5s", got)
	}
}
