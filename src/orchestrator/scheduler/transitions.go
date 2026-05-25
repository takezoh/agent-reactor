package scheduler

import (
	"errors"
	"maps"
	"time"

	"github.com/takezoh/agent-roost/platform/metrics"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// ErrDuplicateDispatch is returned when claim is attempted for an issue that is
// already claimed or running. This enforces the §7.4 single-authority invariant.
var ErrDuplicateDispatch = errors.New("issue already claimed or running")

// All functions below are pure: they take a State value and return a new State,
// never mutating the receiver. Maps are cloned copy-on-write before modification.

// claim reserves an issue slot before spawning (SPEC §16.4).
// Returns ErrDuplicateDispatch if the issue is already claimed or running.
// Removes any existing retry entry for the issue.
func claim(s State, issue tracker.Issue) (State, error) {
	if _, ok := s.Claimed[issue.ID]; ok {
		return s, ErrDuplicateDispatch
	}
	if _, ok := s.Running[issue.ID]; ok {
		return s, ErrDuplicateDispatch
	}
	s.RetryAttempts = withoutKey(s.RetryAttempts, issue.ID)
	s.Claimed = withKey(s.Claimed, issue.ID, struct{}{})
	return s, nil
}

// claimFromRetry promotes a RetryQueued issue back to claimed for re-dispatch (SPEC §7.1).
// The issue must be in RetryAttempts and Claimed (retained by workerExit*) but not Running.
func claimFromRetry(s State, issueID string) (State, error) {
	if _, ok := s.RetryAttempts[issueID]; !ok {
		return s, ErrDuplicateDispatch
	}
	if _, ok := s.Running[issueID]; ok {
		return s, ErrDuplicateDispatch
	}
	if _, ok := s.Claimed[issueID]; !ok {
		// claimed must be retained by workerExit* (§7.1); missing = broken invariant.
		return s, ErrDuplicateDispatch
	}
	s.RetryAttempts = withoutKey(s.RetryAttempts, issueID)
	return s, nil
}

// markRunning promotes an already-claimed issue to running after spawn succeeds (SPEC §16.4).
func markRunning(s State, issue tracker.Issue, attempt int, session LiveSession) State {
	s.Running = withKey(s.Running, issue.ID, RunAttempt{
		Issue:     issue,
		Session:   session,
		Attempt:   attempt,
		Phase:     PhasePreparingWorkspace,
		StartedAt: session.StartedAt,
	})
	return s
}

// updateIssueSnapshot replaces the Issue snapshot for a running attempt (SPEC §8.5 Part B).
func updateIssueSnapshot(s State, issueID string, issue tracker.Issue) State {
	run, ok := s.Running[issueID]
	if !ok {
		return s
	}
	run.Issue = issue
	s.Running = withKey(s.Running, issueID, run)
	return s
}

// workerExitNormal records a clean worker exit and returns a continuation RetryEntry (SPEC §7.3).
// Usage/Runtime accumulators are kept alive across continuation so resumed threads do not
// double-count (§13.5 B”). claimed is retained (§7.1); releaseClaim is the only terminal removal.
func workerExitNormal(s State, issueID string) (State, RetryEntry, bool) {
	run, ok := s.Running[issueID]
	if !ok {
		return s, RetryEntry{}, false
	}
	s.Running = withoutKey(s.Running, issueID)
	return s, RetryEntry{
		IssueID:    issueID,
		Identifier: run.Issue.Identifier,
		Attempt:    run.Attempt + 1,
		Kind:       RetryContinuation,
	}, true
}

// workerExitAbnormal records an abnormal worker exit and returns a backoff RetryEntry (SPEC §7.3).
func workerExitAbnormal(s State, issueID string, err error, attempt int) (State, RetryEntry, bool) {
	run, ok := s.Running[issueID]
	if !ok {
		return s, RetryEntry{}, false
	}
	s.Running = withoutKey(s.Running, issueID)
	return s, RetryEntry{
		IssueID:    issueID,
		Identifier: run.Issue.Identifier,
		Attempt:    attempt + 1,
		Kind:       RetryBackoff,
		Err:        err,
	}, true
}

// releaseClaim removes an issue from all tracking maps, returning it to Unclaimed (SPEC §7.3).
// It rolls up accumulated token/runtime totals into the lifetime counters before deleting
// the per-issue accumulators (§13.5 B”).
func releaseClaim(s State, issueID string) State {
	if acc, ok := s.Usage[issueID]; ok {
		t := acc.Totals()
		s.CodexTotals.Input += t.Input
		s.CodexTotals.Output += t.Output
		s.CodexTotals.Total += t.Total
		s.Usage = withoutKey(s.Usage, issueID)
	}
	if rt, ok := s.Runtime[issueID]; ok {
		s.CodexRuntime += rt
		s.Runtime = withoutKey(s.Runtime, issueID)
	}
	s.Running = withoutKey(s.Running, issueID)
	s.Claimed = withoutKey(s.Claimed, issueID)
	s.RetryAttempts = withoutKey(s.RetryAttempts, issueID)
	return s
}

// enqueueRetry records a retry entry for an issue (SPEC §7.3 / §8.4).
func enqueueRetry(s State, entry RetryEntry) State {
	s.RetryAttempts = withKey(s.RetryAttempts, entry.IssueID, entry)
	return s
}

// incrementTurnCount increments the completed turn counter for a running attempt (SPEC §4.1.6).
func incrementTurnCount(s State, issueID string) State {
	run, ok := s.Running[issueID]
	if !ok {
		return s
	}
	run.TurnCount++
	s.Running = withKey(s.Running, issueID, run)
	return s
}

// updateCodexActivity records the latest codex notification for stall detection (SPEC §8.5 Part A).
// An empty message leaves LastCodexMessage unchanged.
func updateCodexActivity(s State, issueID, event, message string, ts time.Time) State {
	run, ok := s.Running[issueID]
	if !ok {
		return s
	}
	run.LastCodexEvent = event
	run.LastCodexTimestamp = ts
	if message != "" {
		run.LastCodexMessage = message
	}
	s.Running = withKey(s.Running, issueID, run)
	return s
}

// recordUsage applies an absolute cumulative token report via §13.5 (b) bookkeeping.
func recordUsage(s State, issueID string, u metrics.Usage) State {
	run, ok := s.Running[issueID]
	if !ok {
		return s
	}
	acc := s.Usage[issueID].Observe(u)
	s.Usage = withKey(s.Usage, issueID, acc)
	totals := acc.Totals()
	run.TotalInputTokens = totals.Input
	run.TotalOutputTokens = totals.Output
	run.TotalTokens = totals.Total
	s.Running = withKey(s.Running, issueID, run)
	return s
}

// recordRateLimit stores the latest rate-limit snapshot (SPEC §13.5).
func recordRateLimit(s State, issueID string, rl metrics.RateLimitSnapshot) State {
	run, ok := s.Running[issueID]
	if !ok {
		return s
	}
	run.RateLimit = &rl
	s.Running = withKey(s.Running, issueID, run)
	return s
}

// addRuntime accumulates one completed turn's duration for §13.5 runtime aggregation.
func addRuntime(s State, issueID string, d time.Duration) State {
	run, ok := s.Running[issueID]
	if !ok || d <= 0 {
		return s
	}
	run.TotalRuntime += d
	s.Running = withKey(s.Running, issueID, run)
	s.Runtime = withKey(s.Runtime, issueID, s.Runtime[issueID]+d)
	return s
}

// withKey returns a clone of m with key set to v (copy-on-write).
func withKey[K comparable, V any](m map[K]V, key K, v V) map[K]V {
	out := maps.Clone(m)
	if out == nil {
		out = make(map[K]V, 1)
	}
	out[key] = v
	return out
}

// withoutKey returns a clone of m without key (copy-on-write). No-op clone if absent.
func withoutKey[K comparable, V any](m map[K]V, key K) map[K]V {
	if _, ok := m[key]; !ok {
		return m
	}
	out := maps.Clone(m)
	delete(out, key)
	return out
}
