package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

func retryCfg(maxMS int) wfconfig.Config {
	return wfconfig.Config{
		Agent: wfconfig.AgentConfig{MaxRetryBackoffMS: maxMS},
	}
}

// TestContinuationDelay verifies the fixed 1s continuation delay.
func TestContinuationDelay(t *testing.T) {
	if continuationDelay != 1000*time.Millisecond {
		t.Errorf("want 1000ms, got %v", continuationDelay)
	}
}

// TestBackoffDelay verifies exponential growth and max cap (SPEC §8.4).
func TestBackoffDelay(t *testing.T) {
	cases := []struct {
		attempt int
		maxMS   int
		wantMS  int
	}{
		{1, 60_000, 10_000},    // 10000 * 2^0
		{2, 60_000, 20_000},    // 10000 * 2^1
		{3, 60_000, 40_000},    // 10000 * 2^2
		{4, 60_000, 60_000},    // 10000 * 2^3 = 80000, capped at 60000
		{1, 0, 10_000},         // maxMS=0 means no cap
		{10, 300_000, 300_000}, // large attempt, capped
	}
	for _, tc := range cases {
		got := backoffDelay(tc.attempt, retryCfg(tc.maxMS))
		if got != time.Duration(tc.wantMS)*time.Millisecond {
			t.Errorf("attempt=%d maxMS=%d: want %dms, got %v", tc.attempt, tc.maxMS, tc.wantMS, got)
		}
	}
}

// TestScheduleRetry verifies timer fires and sends to channel after delay.
func TestScheduleRetry_TimerFires(t *testing.T) {
	clk := newFakeClock(time.Now())
	st := NewState()
	fireCh := make(chan retryFireReq, 1)

	entry := RetryEntry{IssueID: "id1", Identifier: "P-1", Attempt: 2, Kind: RetryBackoff}
	scheduleRetry(st, clk, fireCh, context.Background(), entry, 5*time.Second)

	// Enqueued but not fired yet.
	snap := st.Snapshot()
	if _, ok := snap.RetryAttempts["id1"]; !ok {
		t.Fatal("want retry entry enqueued")
	}
	select {
	case <-fireCh:
		t.Fatal("timer fired too early")
	default:
	}

	// Advance past delay.
	clk.Advance(5 * time.Second)
	select {
	case req := <-fireCh:
		if req.IssueID != "id1" || req.Attempt != 2 {
			t.Errorf("unexpected req: %+v", req)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not fire")
	}
}

// TestScheduleRetry_CancelledContextDropped verifies timer callback does not block on cancelled ctx.
func TestScheduleRetry_CancelledContextDropped(t *testing.T) {
	clk := newFakeClock(time.Now())
	st := NewState()
	fireCh := make(chan retryFireReq) // unbuffered; no reader

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entry := RetryEntry{IssueID: "id2", Identifier: "P-2", Attempt: 1, Kind: RetryBackoff}
	scheduleRetry(st, clk, fireCh, ctx, entry, time.Millisecond)
	clk.Advance(time.Millisecond)
	// Should not block; just assert no panic.
}

// TestScheduleRetry_RearmStopsOldTimer verifies that re-arming a retry for the same
// issue stops the previous timer so it does not fire (SPEC §8.4 / Elixir cancel_timer).
func TestScheduleRetry_RearmStopsOldTimer(t *testing.T) {
	clk := newFakeClock(time.Now())
	st := NewState()
	fireCh := make(chan retryFireReq, 4)

	e1 := RetryEntry{IssueID: "id3", Identifier: "P-3", Attempt: 1, Kind: RetryBackoff}
	scheduleRetry(st, clk, fireCh, context.Background(), e1, 10*time.Second)

	// Re-arm before the first timer fires — old timer must be stopped.
	e2 := RetryEntry{IssueID: "id3", Identifier: "P-3", Attempt: 2, Kind: RetryBackoff}
	scheduleRetry(st, clk, fireCh, context.Background(), e2, 20*time.Second)

	// Advance past the first timer's deadline; it should NOT fire (was stopped).
	clk.Advance(10 * time.Second)
	select {
	case req := <-fireCh:
		t.Fatalf("orphan fire from old timer: %+v", req)
	default:
	}

	// Advance to the second timer's deadline; it SHOULD fire.
	clk.Advance(10 * time.Second)
	select {
	case req := <-fireCh:
		if req.IssueID != "id3" || req.Attempt != 2 {
			t.Errorf("unexpected req: %+v", req)
		}
	case <-time.After(time.Second):
		t.Fatal("second timer did not fire")
	}
}

// TestRetryTimer_StopZeroValue verifies RetryTimer.Stop on a zero value does not panic.
func TestRetryTimer_StopZeroValue(t *testing.T) {
	var rt RetryTimer
	if rt.Stop() {
		t.Error("zero-value RetryTimer.Stop() should return false")
	}
}
