package scheduler

import (
	"errors"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

var (
	// ErrStall is recorded in a RetryEntry when a worker is killed due to stall timeout.
	ErrStall = errors.New("stall timeout exceeded")
	// ErrNotInRefresh is recorded when an issue disappears from the tracker refresh response (SPEC §8.5).
	ErrNotInRefresh = errors.New("issue not in refresh response")
	// ErrLeftActiveStates is recorded when an issue transitions to a non-active, non-terminal state.
	ErrLeftActiveStates = errors.New("issue left active states")
)

// reduceStall implements reconcile Part A (SPEC §8.5): running attempts whose last codex
// activity (or start time) exceeds stall_timeout_ms are killed and queued for backoff retry.
// Pure: emits EffKillWorker + EffArmRetryTimer; the actual kill/timer happens in the shell.
func reduceStall(s State, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	if cfg.Codex.StallTimeoutMS <= 0 {
		return s, nil
	}
	threshold := time.Duration(cfg.Codex.StallTimeoutMS) * time.Millisecond

	var effs []Effect
	for _, id := range sortedRunningIDs(s) {
		run := s.Running[id]
		base := run.LastCodexTimestamp
		if base.IsZero() {
			base = run.StartedAt
		}
		if base.IsZero() || now.Sub(base) <= threshold {
			continue
		}
		ns, entry, ok := workerExitAbnormal(s, id, ErrStall, run.Attempt)
		if !ok {
			continue
		}
		s = ns
		effs = append(effs, EffKillWorker{IssueID: id, Reason: "stall"})
		var armEffs []Effect
		s, armEffs = armRetry(s, entry, backoffDelay(entry.Attempt, cfg), now)
		effs = append(effs, armEffs...)
	}
	return s, effs
}

// reduceTrackerRefreshed implements reconcile Part B (SPEC §8.5): act on the refreshed
// tracker state for each running issue — terminal → release + remove workspace, active →
// update snapshot, intermediate/missing → kill + backoff retry (workspace retained).
func reduceTrackerRefreshed(s State, e EvTrackerRefreshed, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	byID := make(map[string]tracker.Issue, len(e.Issues))
	for _, iss := range e.Issues {
		byID[iss.ID] = iss
	}
	terminal := normSet(cfg.Tracker.TerminalStates)
	active := normSet(cfg.Tracker.ActiveStates)

	var effs []Effect
	for _, id := range sortedIDs(e.RequestedIDs) {
		run, ok := s.Running[id]
		if !ok {
			continue // already transitioned (e.g. by stall) — skip.
		}
		iss, found := byID[id]
		if !found {
			s, effs = reconcileKillRetry(s, effs, id, run.Attempt, ErrNotInRefresh, "not-found", cfg, now)
			continue
		}
		switch norm := strings.ToLower(iss.State); {
		case terminal[norm]:
			s = releaseClaim(s, id)
			effs = append(effs, EffKillWorker{IssueID: id, Reason: "terminal"},
				EffRemoveWorkspace{Identifier: run.Issue.Identifier})
		case active[norm]:
			s = updateIssueSnapshot(s, id, iss)
		default:
			s, effs = reconcileKillRetry(s, effs, id, run.Attempt, ErrLeftActiveStates, "non-active", cfg, now)
		}
	}
	return s, effs
}

// reconcileKillRetry transitions a running issue to a backoff retry and emits the kill +
// timer-arm effects (workspace retained). Shared by the not-found and non-active branches.
func reconcileKillRetry(s State, effs []Effect, id string, attempt int, cause error, reason string, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	ns, entry, ok := workerExitAbnormal(s, id, cause, attempt)
	if !ok {
		return s, effs
	}
	effs = append(effs, EffKillWorker{IssueID: id, Reason: reason})
	var armEffs []Effect
	ns, armEffs = armRetry(ns, entry, backoffDelay(entry.Attempt, cfg), now)
	return ns, append(effs, armEffs...)
}
