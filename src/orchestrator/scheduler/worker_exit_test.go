package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/platform/tracker"
)

// TestWorkerExit_Normal_ReleasesSlotAndSchedulesContinuation verifies that a clean worker
// exit removes the issue from running and enqueues a continuation retry with the 1s fixed
// delay; the armed timer fires after that delay (§16.6 / §8.4).
func TestWorkerExit_Normal_ReleasesSlotAndSchedulesContinuation(t *testing.T) {
	clk := newFakeClock(time.Now())
	s := New("", schedCfg(), "", Deps{Clock: clk})
	s.seedRunning(testIssue("1", "P-1"), time.Now(), nil)

	s.step(context.Background(), EvWorkerExit{IssueID: "1", Err: nil})

	snap := s.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue removed from running after normal exit")
	}
	entry, ok := snap.RetryAttempts["1"]
	if !ok {
		t.Fatal("want continuation retry enqueued")
	}
	if entry.Kind != RetryContinuation {
		t.Errorf("want RetryContinuation kind, got %v", entry.Kind)
	}

	clk.Advance(continuationDelay + time.Millisecond)
	select {
	case req := <-s.retryFire:
		if req.IssueID != "1" {
			t.Errorf("fireCh got IssueID %q, want %q", req.IssueID, "1")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("want retryFire event after continuation delay")
	}
}

// TestWorkerExit_Normal_ActiveIssue_Respawns verifies the full continuation path:
// worker exits normally → retry due → active issue → second spawn.
func TestWorkerExit_Normal_ActiveIssue_Respawns(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	s := New("", schedCfg(), "", Deps{Tracker: tr, Spawn: spawn.fn, Clock: newFakeClock(time.Now())})
	s.seedRunning(makeIssue("1", "In Progress"), time.Now(), nil)
	ctx := context.Background()

	s.step(ctx, EvWorkerExit{IssueID: "1", Err: nil})
	s.step(ctx, EvRetryDue{IssueID: "1", Identifier: "P-1", Attempt: 2})

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn on continuation, got %d", spawn.callCount())
	}
}

// TestWorkerExit_Normal_TerminalIssue_NoRespawn verifies that a terminal issue is released
// without spawning a new worker.
func TestWorkerExit_Normal_TerminalIssue_NoRespawn(t *testing.T) {
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "Done")}}
	spawn := &fakeSpawn{}
	s := New("", schedCfg(), "", Deps{Tracker: tr, Spawn: spawn.fn, Clock: newFakeClock(time.Now())})
	s.seedRunning(makeIssue("1", "In Progress"), time.Now(), nil)
	ctx := context.Background()

	s.step(ctx, EvWorkerExit{IssueID: "1", Err: nil})
	s.step(ctx, EvRetryDue{IssueID: "1", Identifier: "P-1", Attempt: 2})

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns for terminal issue, got %d", spawn.callCount())
	}
	if _, ok := s.Snapshot().Claimed["1"]; ok {
		t.Error("want claim released for terminal issue")
	}
}

// TestWorkerExit_Abnormal_EnqueuesBackoffRetry verifies that an abnormal exit schedules a
// backoff retry (§8.4).
func TestWorkerExit_Abnormal_EnqueuesBackoffRetry(t *testing.T) {
	s := New("", schedCfg(), "", Deps{Clock: newFakeClock(time.Now())})
	s.seedRunning(testIssue("1", "P-1"), time.Now(), nil)

	s.step(context.Background(), EvWorkerExit{IssueID: "1", Err: errors.New("turn failed"), Attempt: 1})

	snap := s.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue removed from running after abnormal exit")
	}
	entry, ok := snap.RetryAttempts["1"]
	if !ok {
		t.Fatal("want backoff retry enqueued")
	}
	if entry.Kind != RetryBackoff {
		t.Errorf("want RetryBackoff kind, got %v", entry.Kind)
	}
	if entry.Attempt != 2 {
		t.Errorf("want attempt=2 (incremented), got %d", entry.Attempt)
	}
}

// TestWorkerExitNormal_ClaimedRetained verifies SPEC §7.1: a normal worker exit keeps the
// issue in claimed (RetryQueued) so dispatch cannot re-dispatch it.
func TestWorkerExitNormal_ClaimedRetained(t *testing.T) {
	s, err := dispatch(NewState(), testIssue("1", "P-1"), 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	s, _, ok := workerExitNormal(s, "1")
	if !ok {
		t.Fatal("workerExitNormal: expected ok=true for running issue")
	}

	snap := s.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue removed from running after normal exit")
	}
	if _, ok := snap.Claimed["1"]; !ok {
		t.Error("want issue retained in claimed (RetryQueued) after normal exit (SPEC §7.1)")
	}
}

// TestWorkerExitAbnormal_ClaimedRetained verifies SPEC §7.1: an abnormal worker exit keeps
// the issue in claimed (RetryQueued).
func TestWorkerExitAbnormal_ClaimedRetained(t *testing.T) {
	s, err := dispatch(NewState(), testIssue("1", "P-1"), 1, LiveSession{})
	if err != nil {
		t.Fatal(err)
	}

	s, _, ok := workerExitAbnormal(s, "1", errors.New("failed"), 1)
	if !ok {
		t.Fatal("workerExitAbnormal: expected ok=true for running issue")
	}

	snap := s.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue removed from running after abnormal exit")
	}
	if _, ok := snap.Claimed["1"]; !ok {
		t.Error("want issue retained in claimed (RetryQueued) after abnormal exit (SPEC §7.1)")
	}
}
