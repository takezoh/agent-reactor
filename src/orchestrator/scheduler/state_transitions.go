package scheduler

import (
	"errors"
	"maps"
	"time"

	"github.com/takezoh/agent-roost/platform/metrics"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// ErrDuplicateDispatch is returned when Claim or Dispatch is called for an issue
// that is already claimed or running. This enforces the §7.4 single-authority invariant.
var ErrDuplicateDispatch = errors.New("issue already claimed or running")

// Claim reserves an issue slot before spawning (SPEC §16.4).
// Returns ErrDuplicateDispatch if the issue is already claimed or running.
// Removes any existing retry entry for the issue.
func (s *State) Claim(issue tracker.Issue, attempt int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.claimed[issue.ID]; ok {
		return ErrDuplicateDispatch
	}
	if _, ok := s.running[issue.ID]; ok {
		return ErrDuplicateDispatch
	}

	delete(s.retryAttempts, issue.ID)
	s.claimed[issue.ID] = struct{}{}
	return nil
}

// MarkRunning promotes an already-claimed issue to running after spawn succeeds (SPEC §16.4).
// startedAt is recorded for stall detection (§8.5 Part A).
func (s *State) MarkRunning(issueID string, issue tracker.Issue, attempt int, session LiveSession, startedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running[issueID] = RunAttempt{
		Issue:     issue,
		Session:   session,
		Attempt:   attempt,
		Phase:     PhasePreparingWorkspace,
		StartedAt: startedAt,
	}
}

// Dispatch is a convenience wrapper around Claim + MarkRunning for callers that have
// the session ready upfront (SPEC §16.4).
// Returns ErrDuplicateDispatch if the issue is already claimed or running.
func (s *State) Dispatch(issue tracker.Issue, attempt int, session LiveSession, startedAt time.Time) error {
	if err := s.Claim(issue, attempt); err != nil {
		return err
	}
	s.MarkRunning(issue.ID, issue, attempt, session, startedAt)
	return nil
}

// UpdateIssueSnapshot replaces the Issue snapshot for a running attempt (SPEC §8.5 Part B).
// No-op if issueID is not in running.
func (s *State) UpdateIssueSnapshot(issueID string, issue tracker.Issue) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return
	}
	run.Issue = issue
	s.running[issueID] = run
}

// WorkerExitNormal records a clean worker exit and returns a continuation RetryEntry (SPEC §7.3).
// The caller (issue 012) sets DueAtMS to the 1s fixed delay before enqueueing.
// Returns (zero, false) if issueID is not in running.
// usage and runtime accumulators are kept alive across continuation so the resumed thread's
// absolute-cumulative reports do not double-count (§13.5 B”).
func (s *State) WorkerExitNormal(issueID string) (RetryEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return RetryEntry{}, false
	}
	delete(s.running, issueID)
	delete(s.claimed, issueID)
	// usage and runtime intentionally kept for cross-retry accumulation (§13.5 B'').

	return RetryEntry{
		IssueID:    issueID,
		Identifier: run.Issue.Identifier,
		Attempt:    1,
		Kind:       RetryContinuation,
	}, true
}

// WorkerExitAbnormal records an abnormal worker exit and returns a backoff RetryEntry (SPEC §7.3).
// The caller (issue 012) sets DueAtMS to the exponential delay before enqueueing.
// Returns (zero, false) if issueID is not in running.
// usage and runtime accumulators are kept alive so resumed threads (new or continuing) do not
// double-count previously-accumulated totals (§13.5 B”).
func (s *State) WorkerExitAbnormal(issueID string, err error, attempt int) (RetryEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return RetryEntry{}, false
	}
	delete(s.running, issueID)
	delete(s.claimed, issueID)
	// usage and runtime intentionally kept for cross-retry accumulation (§13.5 B'').

	return RetryEntry{
		IssueID:    issueID,
		Identifier: run.Issue.Identifier,
		Attempt:    attempt + 1,
		Kind:       RetryBackoff,
		Err:        err,
	}, true
}

// ReleaseClaim removes an issue from all tracking maps, returning it to Unclaimed (SPEC §7.3).
// It rolls up any accumulated token and runtime totals into the State-level lifetime counters
// before deleting per-issue accumulators (§13.5 B”).
func (s *State) ReleaseClaim(issueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.running, issueID)
	delete(s.claimed, issueID)
	delete(s.retryAttempts, issueID)

	if acc, ok := s.usage[issueID]; ok {
		t := acc.Snapshot()
		s.codexTotals.Input += t.Input
		s.codexTotals.Output += t.Output
		s.codexTotals.Total += t.Total
		delete(s.usage, issueID)
	}
	if rt, ok := s.runtime[issueID]; ok {
		s.codexRuntime += rt
		delete(s.runtime, issueID)
	}
}

// EnqueueRetry registers a retry entry for an issue (SPEC §7.3).
// Callers populate entry.DueAtMS and entry.Timer before calling.
func (s *State) EnqueueRetry(entry RetryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.retryAttempts[entry.IssueID] = entry
}

// UpdateCodexActivity records the latest codex notification for stall detection (SPEC §8.5 Part A).
// An empty message leaves LastCodexMessage unchanged.
// No-op if issueID is not running.
func (s *State) UpdateCodexActivity(issueID, event, message string, ts time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return
	}
	run.LastCodexEvent = event
	run.LastCodexTimestamp = ts
	if message != "" {
		run.LastCodexMessage = message
	}
	s.running[issueID] = run
}

// RecordUsage applies an absolute cumulative token report via §13.5 (b) bookkeeping.
// No-op if issueID is not running.
func (s *State) RecordUsage(issueID string, u metrics.Usage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return
	}
	acc, ok := s.usage[issueID]
	if !ok {
		acc = metrics.NewAccumulator()
		s.usage[issueID] = acc
	}
	totals := acc.Observe(u)
	run.TotalInputTokens = totals.Input
	run.TotalOutputTokens = totals.Output
	run.TotalTokens = totals.Total
	s.running[issueID] = run
}

// RecordRateLimit stores the latest rate-limit snapshot (SPEC §13.5).
// No-op if issueID is not running.
func (s *State) RecordRateLimit(issueID string, rl metrics.RateLimitSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return
	}
	run.RateLimit = &rl
	s.running[issueID] = run
}

// AddRuntime accumulates one completed turn's duration for §13.5 runtime aggregation.
// Updates both the per-run display field and the cross-retry persistent runtime map (B”).
// No-op if issueID is not running or d is non-positive.
func (s *State) AddRuntime(issueID string, d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return
	}
	if d > 0 {
		run.TotalRuntime += d
		s.runtime[issueID] += d
	}
	s.running[issueID] = run
}

// Snapshot returns a deep-copy read-only view of the current state (SPEC §7.3).
// CodexTotals and CodexSecondsRunning reflect lifetime cumulative values:
// ended-session contributions (codexTotals/codexRuntime) plus all live accumulators (§13.5 B”).
func (s *State) Snapshot() StateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := StateSnapshot{
		Running:       make(map[string]RunAttempt, len(s.running)),
		Claimed:       make(map[string]struct{}, len(s.claimed)),
		RetryAttempts: make(map[string]RetryEntry, len(s.retryAttempts)),
	}
	maps.Copy(snap.Running, s.running)
	maps.Copy(snap.Claimed, s.claimed)
	maps.Copy(snap.RetryAttempts, s.retryAttempts)

	totals := s.codexTotals
	for _, acc := range s.usage {
		t := acc.Snapshot()
		totals.Input += t.Input
		totals.Output += t.Output
		totals.Total += t.Total
	}
	snap.CodexTotals = totals

	rt := s.codexRuntime
	for _, d := range s.runtime {
		rt += d
	}
	snap.CodexSecondsRunning = rt.Seconds()

	return snap
}
