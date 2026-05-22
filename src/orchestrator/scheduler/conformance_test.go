package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

// SPEC §17.1 — invalid workflow reload keeps last-known-good config;
// new dispatch is gated until a valid config is restored (§6.2).
func TestSPEC_17_1_LastKnownGoodOnInvalidReload(t *testing.T) {
	path := writeWorkflow(t)
	tr := &fakeTracker{issues: []ptrackerv.Issue{
		{ID: "1", Identifier: "P-1", Title: "issue", State: "In Progress"},
	}}
	spawn := &fakeSpawn{}
	s := New(path, schedCfg(), minDeps(tr, spawn.fn, newFakeClock(time.Now())))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Write invalid config (missing project_slug fails Preflight).
	writeFrontMatter(t, path, strings.ReplaceAll(validFrontMatter, "project_slug: test-proj\n", ""))
	s.tickOnce(ctx)

	if spawn.callCount() != 0 {
		t.Errorf("want 0 spawn calls after invalid reload (dispatch gated), got %d", spawn.callCount())
	}
	if !s.degraded {
		t.Error("want s.degraded == true while config is invalid")
	}
}

// SPEC §17.4 — failure retry backoff is capped by agent.max_retry_backoff_ms (§8.4).
func TestSPEC_17_4_RetryBackoffCapHonored(t *testing.T) {
	cfg := schedCfg()
	cfg.Agent.MaxRetryBackoffMS = 30_000

	// Attempt 10 would produce 10000 * 2^9 = 5120000ms uncapped.
	got := backoffDelay(10, cfg)
	if got.Milliseconds() > int64(cfg.Agent.MaxRetryBackoffMS) {
		t.Errorf("backoffDelay(10) = %v, want <= %dms cap", got, cfg.Agent.MaxRetryBackoffMS)
	}
	if got != 30_000*time.Millisecond {
		t.Errorf("backoffDelay(10) = %v, want exactly cap 30s", got)
	}
}

// SPEC §17.4 — per-state concurrency cap is applied independently from the global cap (§8.3).
func TestSPEC_17_4_PerStateConcurrency(t *testing.T) {
	// Per-state cap of 1 for "In Progress" with global cap 5.
	cfg := agentCfg(5, map[string]int{"in progress": 1})

	emptySnap := snapWithRunning()
	if got := availablePerStateSlots("In Progress", emptySnap, cfg); got != 1 {
		t.Errorf("no running: want 1 per-state slot, got %d", got)
	}

	// One already running in "In Progress": per-state slot exhausted.
	oneRunning := snapWithRunning("In Progress")
	if got := availablePerStateSlots("In Progress", oneRunning, cfg); got != 0 {
		t.Errorf("one running: want 0 per-state slots, got %d", got)
	}
}

// SPEC §17.4 — normal worker exit schedules a continuation with a fixed 1s delay (§8.4).
func TestSPEC_17_4_ContinuationFixed1s(t *testing.T) {
	if continuationDelay != 1000*time.Millisecond {
		t.Errorf("continuationDelay = %v, want 1s", continuationDelay)
	}
}

// SPEC §4.1.5 — attempt is null/0 on the first run; >=1 for retries/continuations.
// Verifies that dispatchOnce passes attempt=0 to SpawnFunc on the initial dispatch.
func TestSPEC_4_1_5_FirstRunAttemptZero(t *testing.T) {
	st := NewState()
	spawn := &fakeSpawn{}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []ptrackerv.Issue{{ID: "1", Identifier: "P-1", Title: "t", State: "In Progress"}}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, schedCfg())

	if spawn.callCount() != 1 {
		t.Fatalf("want 1 spawn, got %d", spawn.callCount())
	}
	if got := spawn.calls[0].Attempt; got != 0 {
		t.Errorf("SPEC §4.1.5: first run attempt must be 0 (null), got %d", got)
	}
}

// SPEC §8.4 — failure backoff is 10s-based exponential; first failure must use 10s.
// Verifies that an initial-run spawn failure schedules a retry with a 10s backoff.
func TestSPEC_8_4_FirstFailureBackoff10s(t *testing.T) {
	cfg := schedCfg()
	st := NewState()
	spawn := &fakeSpawn{err: errors.New("agent error")}
	clk := newFakeClock(time.Now())
	fireCh := make(chan retryFireReq, 4)

	cands := []ptrackerv.Issue{{ID: "1", Identifier: "P-1", Title: "t", State: "In Progress"}}
	dispatchOnce(context.Background(), cands, st, clk, fireCh, spawn.fn, cfg)

	entry, ok := st.Snapshot().RetryAttempts["1"]
	if !ok {
		t.Fatal("want retry entry after initial spawn fail")
	}
	wantDelay := backoffDelay(1, cfg) // 10s
	gotDelayMS := entry.DueAtMS - clk.Now().UnixMilli()
	if gotDelayMS != wantDelay.Milliseconds() {
		t.Errorf("SPEC §8.4: first failure backoff want %dms (10s), got %dms", wantDelay.Milliseconds(), gotDelayMS)
	}
}
