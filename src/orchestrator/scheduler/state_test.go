package scheduler

import (
	"errors"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/platform/metrics"
	"github.com/takezoh/agent-roost/platform/tracker"
)

func testIssue(id, identifier string) tracker.Issue {
	return tracker.Issue{ID: id, Identifier: identifier, Title: "t"}
}

// dispatch is a test helper that drives the pure claim → markRunning transition,
// mirroring how the shell promotes an eligible issue to running.
func dispatch(s State, issue tracker.Issue, attempt int, session LiveSession) (State, error) {
	s, err := claim(s, issue)
	if err != nil {
		return s, err
	}
	return markRunning(s, issue, attempt, session), nil
}

// redispatchRetry drives the state machine from RetryQueued back to Running:
// enqueueRetry → claimFromRetry → markRunning. The retry is always attempt 2
// (attempt 1 is the initial dispatch).
func redispatchRetry(t *testing.T, s State, issueID, identifier string, issue tracker.Issue) State {
	t.Helper()
	const attempt = 2
	s = enqueueRetry(s, RetryEntry{IssueID: issueID, Identifier: identifier, Attempt: attempt})
	s, err := claimFromRetry(s, issueID)
	if err != nil {
		t.Fatal(err)
	}
	return markRunning(s, issue, attempt, LiveSession{})
}

func TestStateDispatch_AddsToRunningAndClaimed(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	session := LiveSession{SessionID: "s1"}

	s, err := dispatch(s, issue, 1, session)
	if err != nil {
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

	s, err := dispatch(s, issue, 1, session)
	if err != nil {
		t.Fatalf("first dispatch: %v", err)
	}
	if _, err := dispatch(s, issue, 2, session); !errors.Is(err, ErrDuplicateDispatch) {
		t.Errorf("expected ErrDuplicateDispatch, got %v", err)
	}
}

func TestStateDispatch_ClearsPriorRetry(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	s = enqueueRetry(s, RetryEntry{IssueID: "id1", Kind: RetryBackoff})

	s, err := dispatch(s, issue, 2, LiveSession{})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if _, ok := s.Snapshot().RetryAttempts["id1"]; ok {
		t.Error("expected retry entry cleared on claim")
	}
}

func TestStateWorkerExitNormal_RemovesRunningAndReturnsContinuation(t *testing.T) {
	s := NewState()
	issue := testIssue("id1", "PROJ-1")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	s, entry, ok := workerExitNormal(s, "id1")
	if !ok {
		t.Fatal("expected workerExitNormal to return ok=true")
	}
	if entry.Kind != RetryContinuation {
		t.Errorf("got kind %v, want RetryContinuation", entry.Kind)
	}
	// Dispatch was at attempt=1; continuation should be attempt=2 (run.Attempt+1),
	// consistent with workerExitAbnormal which also increments.
	if entry.Attempt != 2 {
		t.Errorf("got attempt %d, want 2 (run.Attempt+1)", entry.Attempt)
	}
	if _, ok := s.Snapshot().Running["id1"]; ok {
		t.Error("expected id1 removed from running")
	}
}

func TestStateWorkerExitAbnormal_RemovesRunningAndReturnsBackoff(t *testing.T) {
	s := NewState()
	issue := testIssue("id2", "PROJ-2")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	someErr := errors.New("agent crashed")
	s, entry, ok := workerExitAbnormal(s, "id2", someErr, 1)
	if !ok {
		t.Fatal("expected workerExitAbnormal to return ok=true")
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
	if _, ok := s.Snapshot().Running["id2"]; ok {
		t.Error("expected id2 removed from running")
	}
}

func TestStateReleaseClaim_RemovesFromAllMaps(t *testing.T) {
	s := NewState()
	issue := testIssue("id3", "PROJ-3")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}
	s = enqueueRetry(s, RetryEntry{IssueID: "id3"})

	s = releaseClaim(s, "id3")

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
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	snap := s.Snapshot()
	delete(snap.Running, "id4")
	delete(snap.Claimed, "id4")

	snap2 := s.Snapshot()
	if _, ok := snap2.Running["id4"]; !ok {
		t.Error("State.Running was mutated by modifying Snapshot")
	}
	if _, ok := snap2.Claimed["id4"]; !ok {
		t.Error("State.Claimed was mutated by modifying Snapshot")
	}
}

func TestStateClaim_AddsToClaimedOnly(t *testing.T) {
	s := NewState()
	issue := testIssue("id10", "PROJ-10")

	s, err := claim(s, issue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := s.Snapshot()
	if _, ok := snap.Claimed["id10"]; !ok {
		t.Error("expected id10 in claimed")
	}
	if _, ok := snap.Running["id10"]; ok {
		t.Error("expected id10 NOT in running after claim")
	}
}

func TestStateMarkRunning_PromotesToRunning(t *testing.T) {
	s := NewState()
	issue := testIssue("id11", "PROJ-11")
	session := LiveSession{SessionID: "s11"}

	s, err := claim(s, issue)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	s = markRunning(s, issue, 2, session)

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

	s, err := claim(s, issue)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if _, err := claim(s, issue); !errors.Is(err, ErrDuplicateDispatch) {
		t.Errorf("expected ErrDuplicateDispatch, got %v", err)
	}
}

// TestStateClaim_SingleAuthority verifies the §7.4 single-authority invariant as a pure
// property: claiming an already-claimed issue is rejected. (Concurrency is no longer a
// concern — the single Run loop is the only writer of State; see scheduler.go.)
func TestStateClaim_SingleAuthority(t *testing.T) {
	s := NewState()
	issue := testIssue("id5", "PROJ-5")

	s, err := claim(s, issue)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	// Any number of subsequent claims on the same State all fail identically.
	for i := range 20 {
		if _, err := claim(s, issue); !errors.Is(err, ErrDuplicateDispatch) {
			t.Fatalf("claim %d: expected ErrDuplicateDispatch, got %v", i, err)
		}
	}
}

func TestUpdateIssueSnapshot_UpdatesRunningIssue(t *testing.T) {
	s := NewState()
	issue := testIssue("id6", "PROJ-6")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	updated := testIssue("id6", "PROJ-6")
	updated.State = "In Review"
	s = updateIssueSnapshot(s, "id6", updated)

	run, ok := s.Snapshot().Running["id6"]
	if !ok {
		t.Fatal("expected id6 in running")
	}
	if run.Issue.State != "In Review" {
		t.Errorf("got state %q, want In Review", run.Issue.State)
	}
}

func TestUpdateIssueSnapshot_NoopForUnknownID(t *testing.T) {
	s := NewState()
	s = updateIssueSnapshot(s, "unknown", testIssue("unknown", "X-1"))
	if len(s.Snapshot().Running) != 0 {
		t.Error("expected running to remain empty")
	}
}

func TestUpdateCodexActivity_SetsFields(t *testing.T) {
	s := NewState()
	issue := testIssue("id20", "PROJ-20")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	ts := time.Unix(1234567890, 0)
	s = updateCodexActivity(s, "id20", "turn/completed", "hello", ts)

	run := s.Snapshot().Running["id20"]
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
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}
	s = updateCodexActivity(s, "id21", "m1", "initial", time.Now())
	s = updateCodexActivity(s, "id21", "m2", "", time.Now())
	if got := s.Snapshot().Running["id21"].LastCodexMessage; got != "initial" {
		t.Errorf("empty message overwrote existing: got %q", got)
	}
}

func TestUpdateCodexActivity_NoopForUnknownID(t *testing.T) {
	s := NewState()
	_ = updateCodexActivity(s, "unknown", "e", "m", time.Now()) // must not panic
}

func TestRecordUsage_AggregatesAcrossReports(t *testing.T) {
	s := NewState()
	issue := testIssue("id22", "PROJ-22")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	s = recordUsage(s, "id22", metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	run := s.Snapshot().Running["id22"]
	if run.TotalInputTokens != 100 || run.TotalOutputTokens != 50 || run.TotalTokens != 150 {
		t.Errorf("after first report: got %d/%d/%d, want 100/50/150",
			run.TotalInputTokens, run.TotalOutputTokens, run.TotalTokens)
	}

	// Second report cumulative 250/120/370 — only the delta is added (no double-count).
	s = recordUsage(s, "id22", metrics.Usage{ThreadID: "t1", Input: 250, Output: 120, Total: 370})
	run = s.Snapshot().Running["id22"]
	if run.TotalInputTokens != 250 || run.TotalOutputTokens != 120 || run.TotalTokens != 370 {
		t.Errorf("after second report: got %d/%d/%d, want 250/120/370",
			run.TotalInputTokens, run.TotalOutputTokens, run.TotalTokens)
	}
}

func TestRecordUsage_NoopForUnknownID(t *testing.T) {
	s := NewState()
	_ = recordUsage(s, "unknown", metrics.Usage{ThreadID: "t1", Input: 100}) // must not panic
}

// TestRecordUsage_AccumulatorPersistsAcrossContinuation verifies §13.5 B” semantics:
// the per-issue accumulator survives workerExitNormal so that a resumed thread's
// absolute-cumulative reports do not double-count previously observed totals.
func TestRecordUsage_AccumulatorPersistsAcrossContinuation(t *testing.T) {
	s := NewState()
	issue := testIssue("id23", "PROJ-23")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}
	s = recordUsage(s, "id23", metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	s, _, _ = workerExitNormal(s, "id23")

	s = redispatchRetry(t, s, "id23", "PROJ-23", issue)
	// Thread resumes: reports 50 more input, 20 more output (absolute 150/70/220).
	s = recordUsage(s, "id23", metrics.Usage{ThreadID: "t1", Input: 150, Output: 70, Total: 220})
	snap := s.Snapshot()
	run := snap.Running["id23"]
	// Lifetime totals: first segment 100/50/150 + second-segment delta 50/20/70 = 150/70/220.
	if run.TotalInputTokens != 150 || run.TotalOutputTokens != 70 || run.TotalTokens != 220 {
		t.Errorf("wrong lifetime total: got %d/%d/%d, want 150/70/220",
			run.TotalInputTokens, run.TotalOutputTokens, run.TotalTokens)
	}
	// CodexTotals live contribution: same issue is still running, so ended counter is 0.
	if snap.CodexTotals.Input != 150 || snap.CodexTotals.Total != 220 {
		t.Errorf("CodexTotals wrong: got input=%d total=%d, want 150/220",
			snap.CodexTotals.Input, snap.CodexTotals.Total)
	}
}

// TestCodexTotals_RollupOnReleaseClaim verifies that terminal release (releaseClaim) rolls up
// the per-issue accumulator into the State-level lifetime counter (§13.5 B”).
func TestCodexTotals_RollupOnReleaseClaim(t *testing.T) {
	s := NewState()
	issue := testIssue("id30", "PROJ-30")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}
	s = recordUsage(s, "id30", metrics.Usage{ThreadID: "t1", Input: 200, Output: 80, Total: 280})
	s = addRuntime(s, "id30", 10*time.Second)

	// Before release: live contribution visible in snapshot.
	snapBefore := s.Snapshot()
	if snapBefore.CodexTotals.Input != 200 {
		t.Errorf("before release: got CodexTotals.Input=%d, want 200", snapBefore.CodexTotals.Input)
	}
	if snapBefore.CodexSecondsRunning != 10.0 {
		t.Errorf("before release: got CodexSecondsRunning=%v, want 10", snapBefore.CodexSecondsRunning)
	}

	s = releaseClaim(s, "id30")

	// After release: ended counter carries the rolled-up totals; no live entry.
	snapAfter := s.Snapshot()
	if snapAfter.CodexTotals.Input != 200 || snapAfter.CodexTotals.Total != 280 {
		t.Errorf("after release: got CodexTotals input=%d total=%d, want 200/280",
			snapAfter.CodexTotals.Input, snapAfter.CodexTotals.Total)
	}
	if snapAfter.CodexSecondsRunning != 10.0 {
		t.Errorf("after release: got CodexSecondsRunning=%v, want 10", snapAfter.CodexSecondsRunning)
	}
}

// TestCodexTotals_NoDoubleCountAcrossRetry verifies that a continuation retry followed by
// terminal release does not double-count tokens (§13.5 B”).
func TestCodexTotals_NoDoubleCountAcrossRetry(t *testing.T) {
	s := NewState()
	issue := testIssue("id31", "PROJ-31")

	// Attempt 1: thread reports 100 tokens absolute.
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}
	s = recordUsage(s, "id31", metrics.Usage{ThreadID: "t1", Input: 100, Output: 40, Total: 140})
	s, _, _ = workerExitNormal(s, "id31")

	s = redispatchRetry(t, s, "id31", "PROJ-31", issue)
	s = recordUsage(s, "id31", metrics.Usage{ThreadID: "t1", Input: 150, Output: 60, Total: 210})

	// Terminal release: should roll up accumulated 150/60/210 once.
	s = releaseClaim(s, "id31")

	snap := s.Snapshot()
	if snap.CodexTotals.Input != 150 || snap.CodexTotals.Total != 210 {
		t.Errorf("double-counted: got input=%d total=%d, want 150/210",
			snap.CodexTotals.Input, snap.CodexTotals.Total)
	}
}

// TestCodexTotals_RuntimePersistsAcrossRetry verifies per-turn runtime survives continuation.
func TestCodexTotals_RuntimePersistsAcrossRetry(t *testing.T) {
	s := NewState()
	issue := testIssue("id32", "PROJ-32")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}
	s = addRuntime(s, "id32", 5*time.Second)
	s, _, _ = workerExitNormal(s, "id32")

	s = redispatchRetry(t, s, "id32", "PROJ-32", issue)
	s = addRuntime(s, "id32", 3*time.Second)

	if got := s.Snapshot().CodexSecondsRunning; got != 8.0 {
		t.Errorf("got CodexSecondsRunning=%v, want 8", got)
	}

	s = releaseClaim(s, "id32")
	if got := s.Snapshot().CodexSecondsRunning; got != 8.0 {
		t.Errorf("after release: got %v, want 8", got)
	}
}

func TestRecordRateLimit_SetsField(t *testing.T) {
	s := NewState()
	issue := testIssue("id24", "PROJ-24")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	s = recordRateLimit(s, "id24", metrics.RateLimitSnapshot{PrimaryUsedPercent: 75, PrimaryResetsAt: 999})
	run := s.Snapshot().Running["id24"]
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
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	s = addRuntime(s, "id25", 2*time.Second)
	s = addRuntime(s, "id25", 3*time.Second)
	if got := s.Snapshot().Running["id25"].TotalRuntime; got != 5*time.Second {
		t.Errorf("got %v, want 5s", got)
	}
}

func TestIncrementTurnCount_Basic(t *testing.T) {
	s := NewState()
	issue := testIssue("id40", "PROJ-40")
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	s = incrementTurnCount(s, "id40")
	s = incrementTurnCount(s, "id40")
	s = incrementTurnCount(s, "id40")

	if got := s.Snapshot().Running["id40"].TurnCount; got != 3 {
		t.Errorf("got TurnCount=%d, want 3", got)
	}
}

func TestIncrementTurnCount_NoopForUnknown(t *testing.T) {
	s := NewState()
	s = incrementTurnCount(s, "unknown") // must not panic

	if len(s.Snapshot().Running) != 0 {
		t.Error("expected running to remain empty")
	}
}

func TestIncrementTurnCount_ResetOnRespawn(t *testing.T) {
	s := NewState()
	issue := testIssue("id41", "PROJ-41")

	// Attempt 1: increment twice and confirm it reaches 2.
	s, err := dispatch(s, issue, 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}
	s = incrementTurnCount(s, "id41")
	s = incrementTurnCount(s, "id41")
	if got := s.Snapshot().Running["id41"].TurnCount; got != 2 {
		t.Fatalf("attempt 1: got TurnCount=%d, want 2", got)
	}
	s, _, _ = workerExitNormal(s, "id41")

	// Attempt 2: re-dispatch via the RetryQueued path (claimed is retained after
	// workerExitNormal per SPEC §7.1, so a fresh claim would be rejected as duplicate).
	s = redispatchRetry(t, s, "id41", "PROJ-41", issue)
	s = incrementTurnCount(s, "id41")
	if got := s.Snapshot().Running["id41"].TurnCount; got != 1 {
		t.Errorf("got TurnCount=%d after respawn+1 increment, want 1 (carry-over from attempt 1 would give 3)", got)
	}
}
