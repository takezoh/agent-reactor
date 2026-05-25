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

// TestArmTimer_Fires verifies the shell's retry timer delivers a retryFireReq after the delay.
func TestArmTimer_Fires(t *testing.T) {
	clk := newFakeClock(time.Now())
	s := New("", schedCfg(), "", Deps{Clock: clk})

	s.armTimer(context.Background(), "id1", "P-1", 2, 5*time.Second)

	select {
	case <-s.retryFire:
		t.Fatal("timer fired too early")
	default:
	}

	clk.Advance(5 * time.Second)
	select {
	case req := <-s.retryFire:
		if req.IssueID != "id1" || req.Attempt != 2 {
			t.Errorf("unexpected req: %+v", req)
		}
	default:
		t.Fatal("timer did not fire")
	}
}

// TestArmTimer_CancelledContextDropped verifies the timer callback does not block on a
// cancelled context (the fire is dropped rather than blocking the goroutine).
func TestArmTimer_CancelledContextDropped(t *testing.T) {
	clk := newFakeClock(time.Now())
	s := New("", schedCfg(), "", Deps{Clock: clk})
	s.retryFire = make(chan retryFireReq) // unbuffered; no reader

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.armTimer(ctx, "id2", "P-2", 1, time.Millisecond)
	clk.Advance(time.Millisecond)
	select {
	case req := <-s.retryFire:
		t.Fatalf("cancelled ctx: unexpected fire: %+v", req)
	default:
	}
}

// TestArmTimer_RearmStopsOldTimer verifies that re-arming a retry for the same issue stops
// the previous timer so it does not fire (SPEC §8.4 / Elixir cancel_timer).
func TestArmTimer_RearmStopsOldTimer(t *testing.T) {
	clk := newFakeClock(time.Now())
	s := New("", schedCfg(), "", Deps{Clock: clk})

	s.armTimer(context.Background(), "id3", "P-3", 1, 10*time.Second)
	// Re-arm before first timer fires.
	s.armTimer(context.Background(), "id3", "P-3", 2, 20*time.Second)

	// Advance past the first timer's deadline; it should NOT fire (was stopped).
	clk.Advance(10 * time.Second)
	select {
	case req := <-s.retryFire:
		t.Fatalf("orphan fire from old timer: %+v", req)
	default:
	}

	// Advance to the second timer's deadline; it SHOULD fire.
	clk.Advance(10 * time.Second)
	select {
	case req := <-s.retryFire:
		if req.IssueID != "id3" || req.Attempt != 2 {
			t.Errorf("unexpected req: %+v", req)
		}
	default:
		t.Fatal("second timer did not fire")
	}
}
