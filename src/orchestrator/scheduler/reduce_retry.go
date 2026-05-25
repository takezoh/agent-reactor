package scheduler

import (
	"strings"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// reduceRetryResolved acts on the candidate list fetched after a retry timer fired (SPEC §8.4).
// Transient fetch failure reschedules; a missing or no-longer-active issue is released; a
// still-active issue is re-claimed from retry and spawned when a slot is free, otherwise requeued.
func reduceRetryResolved(s State, e EvRetryResolved, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	if e.FetchErr != nil {
		return armRetry(s, retryEntry(e, e.Attempt+1, e.FetchErr), backoffDelay(e.Attempt+1, cfg), now)
	}

	found := findIssue(e.Issues, e.IssueID)
	if found == nil {
		return releaseClaim(s, e.IssueID), nil
	}

	terminal := normSet(cfg.Tracker.TerminalStates)
	active := normSet(cfg.Tracker.ActiveStates)
	if norm := strings.ToLower(found.State); terminal[norm] || !active[norm] {
		return releaseClaim(s, e.IssueID), nil
	}

	if !retryReclaimHasSlot(s, cfg, e.IssueID, found.State) {
		return armRetry(s, retryEntry(e, e.Attempt+1, errNoSlots), backoffDelay(e.Attempt+1, cfg), now)
	}

	ns, err := claimFromRetry(s, e.IssueID)
	if err != nil {
		return s, nil // claim rejected (state changed underneath); leave as-is.
	}
	return ns, []Effect{EffSpawn{Issue: *found, Attempt: e.Attempt}}
}

// retryEntry builds a backoff RetryEntry for a retry-fire reschedule.
func retryEntry(e EvRetryResolved, attempt int, cause error) RetryEntry {
	return RetryEntry{
		IssueID:    e.IssueID,
		Identifier: e.Identifier,
		Attempt:    attempt,
		Kind:       RetryBackoff,
		Err:        cause,
	}
}

// findIssue returns the issue with the given ID, or nil if absent.
func findIssue(issues []tracker.Issue, id string) *tracker.Issue {
	for i := range issues {
		if issues[i].ID == id {
			return &issues[i]
		}
	}
	return nil
}
