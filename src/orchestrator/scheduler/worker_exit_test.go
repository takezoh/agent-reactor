package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/platform/tracker"
)

func makeRunningScheduler(t *testing.T) (*Scheduler, *fakeClock, chan retryFireReq) {
	t.Helper()
	st := NewState()
	iss := tracker.Issue{ID: "1", Identifier: "P-1", State: "In Progress"}
	if err := st.Dispatch(iss, 0, LiveSession{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)
	s := &Scheduler{
		workflowPath: writeWorkflow(t),
		state:        st,
		clock:        clk,
		retryFire:    fireCh,
		workerDone:   make(chan WorkerExit, 4),
	}
	return s, clk, fireCh
}

// TestHandleWorkerExit_Normal_ReleasesSlotAndSchedulesContinuation verifies that a
// clean worker exit removes the issue from running and enqueues a continuation retry
// with the 1s fixed delay (§16.6 / §8.4).
func TestHandleWorkerExit_Normal_ReleasesSlotAndSchedulesContinuation(t *testing.T) {
	s, clk, fireCh := makeRunningScheduler(t)

	s.handleWorkerExit(context.Background(), WorkerExit{IssueID: "1", Err: nil})

	snap := s.state.Snapshot()
	if _, ok := snap.Running["1"]; ok {
		t.Error("want issue removed from running after normal exit")
	}
	if _, ok := snap.RetryAttempts["1"]; !ok {
		t.Error("want continuation retry enqueued")
	}
	if snap.RetryAttempts["1"].Kind != RetryContinuation {
		t.Errorf("want RetryContinuation kind, got %v", snap.RetryAttempts["1"].Kind)
	}

	clk.Advance(continuationDelay + time.Millisecond)
	select {
	case req := <-fireCh:
		if req.IssueID != "1" {
			t.Errorf("fireCh got IssueID %q, want %q", req.IssueID, "1")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("want retryFire event after continuation delay")
	}
}

// TestHandleWorkerExit_Normal_ActiveIssue_Respawns verifies the full continuation path:
// worker exits normally → 1s retry fires → active issue → second spawn.
func TestHandleWorkerExit_Normal_ActiveIssue_Respawns(t *testing.T) {
	s, clk, fireCh := makeRunningScheduler(t)
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "In Progress")}}
	spawn := &fakeSpawn{}
	cfg := dispCfg()
	ctx := context.Background()

	s.handleWorkerExit(ctx, WorkerExit{IssueID: "1", Err: nil})

	clk.Advance(continuationDelay + time.Millisecond)
	req := <-fireCh
	handleRetryFire(ctx, req, tr, s.state, clk, fireCh, spawn.fn, cfg)

	if spawn.callCount() != 1 {
		t.Errorf("want 1 spawn on continuation, got %d", spawn.callCount())
	}
}

// TestHandleWorkerExit_Normal_TerminalIssue_NoRespawn verifies that a terminal issue
// is released without spawning a new worker.
func TestHandleWorkerExit_Normal_TerminalIssue_NoRespawn(t *testing.T) {
	s, clk, fireCh := makeRunningScheduler(t)
	tr := &fakeTracker{issues: []tracker.Issue{makeIssue("1", "Done")}}
	spawn := &fakeSpawn{}
	cfg := dispCfg()
	ctx := context.Background()

	s.handleWorkerExit(ctx, WorkerExit{IssueID: "1", Err: nil})

	clk.Advance(continuationDelay + time.Millisecond)
	req := <-fireCh
	handleRetryFire(ctx, req, tr, s.state, clk, fireCh, spawn.fn, cfg)

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawns for terminal issue, got %d", spawn.callCount())
	}
	snap := s.state.Snapshot()
	if _, ok := snap.Claimed["1"]; ok {
		t.Error("want claim released for terminal issue")
	}
}

// TestHandleWorkerExit_Abnormal_EnqueuesBackoffRetry verifies that an abnormal exit
// schedules a backoff retry (§8.4).
func TestHandleWorkerExit_Abnormal_EnqueuesBackoffRetry(t *testing.T) {
	s, _, _ := makeRunningScheduler(t)

	s.handleWorkerExit(context.Background(), WorkerExit{
		IssueID: "1",
		Err:     errors.New("turn failed"),
		Attempt: 1,
	})

	snap := s.state.Snapshot()
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
