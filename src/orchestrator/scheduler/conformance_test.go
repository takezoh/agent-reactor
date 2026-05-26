package scheduler

import (
	"context"
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
	s := New(path, schedCfg(), "", minDeps(tr, spawn.fn, newFakeClock(time.Now())))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Write invalid config (missing project_slug fails Preflight).
	writeFrontMatter(t, path, strings.ReplaceAll(validFrontMatter, "project_slugs: [test-proj]\n", ""))
	s.tick(ctx)

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

// SPEC §7.1 / §7.4 — claimed always contains RetryQueued issues; dispatchOnce must not
// re-dispatch during the backoff window regardless of how many ticks fire.
func TestSPEC_7_1_RetryQueuedStaysClaimed(t *testing.T) {
	path := writeWorkflow(t)
	tr := &fakeTracker{issues: []ptrackerv.Issue{
		{ID: "1", Identifier: "P-1", Title: "issue", State: "In Progress"},
	}}
	spawn := &fakeSpawn{}
	s := New(path, schedCfg(), "", minDeps(tr, spawn.fn, newFakeClock(time.Now())))
	ctx := context.Background()

	// Initial tick dispatches the issue.
	s.tick(ctx)
	if spawn.callCount() != 1 {
		t.Fatalf("want 1 spawn after first tick, got %d", spawn.callCount())
	}

	// Worker exits normally — issue enters RetryQueued, claimed is retained.
	s.step(ctx, EvWorkerExit{IssueID: "1", Err: nil})
	if _, ok := s.Snapshot().Claimed["1"]; !ok {
		t.Fatal("SPEC §7.1: want issue retained in claimed after normal worker exit")
	}

	// Simulate multiple ticks during the retry backoff window.
	for range 3 {
		s.tick(ctx)
	}

	// No additional spawns: the issue is still RetryQueued / claimed.
	if spawn.callCount() != 1 {
		t.Errorf("SPEC §7.4: want 0 additional spawns during retry window, got %d extra", spawn.callCount()-1)
	}
}
