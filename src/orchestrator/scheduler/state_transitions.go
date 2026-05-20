package scheduler

import (
	"errors"
	"maps"

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
func (s *State) MarkRunning(issueID string, issue tracker.Issue, attempt int, session LiveSession) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running[issueID] = RunAttempt{
		Issue:   issue,
		Session: session,
		Attempt: attempt,
		Phase:   PhasePreparingWorkspace,
	}
}

// Dispatch records a new running attempt for issue (SPEC §16.4). It is a convenience
// wrapper around Claim + MarkRunning for callers that have the session ready upfront.
// Returns ErrDuplicateDispatch if the issue is already claimed or running.
func (s *State) Dispatch(issue tracker.Issue, attempt int, session LiveSession) error {
	if err := s.Claim(issue, attempt); err != nil {
		return err
	}
	s.MarkRunning(issue.ID, issue, attempt, session)
	return nil
}

// WorkerExitNormal records a clean worker exit and returns a continuation RetryEntry (SPEC §7.3).
// The caller (issue 012) sets DueAtMS to the 1s fixed delay before enqueueing.
// Returns (zero, false) if issueID is not in running.
func (s *State) WorkerExitNormal(issueID string) (RetryEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return RetryEntry{}, false
	}
	delete(s.running, issueID)

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
func (s *State) WorkerExitAbnormal(issueID string, err error, attempt int) (RetryEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.running[issueID]
	if !ok {
		return RetryEntry{}, false
	}
	delete(s.running, issueID)

	return RetryEntry{
		IssueID:    issueID,
		Identifier: run.Issue.Identifier,
		Attempt:    attempt + 1,
		Kind:       RetryBackoff,
		Err:        err,
	}, true
}

// ReleaseClaim removes an issue from all tracking maps, returning it to Unclaimed (SPEC §7.3).
func (s *State) ReleaseClaim(issueID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.running, issueID)
	delete(s.claimed, issueID)
	delete(s.retryAttempts, issueID)
}

// EnqueueRetry registers a retry entry for an issue (SPEC §7.3).
// Callers populate entry.DueAtMS and entry.Timer before calling.
func (s *State) EnqueueRetry(entry RetryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.retryAttempts[entry.IssueID] = entry
}

// Snapshot returns a deep-copy read-only view of the current state (SPEC §7.3).
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
	return snap
}
