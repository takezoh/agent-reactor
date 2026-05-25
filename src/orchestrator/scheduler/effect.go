package scheduler

import (
	"time"

	"github.com/takezoh/agent-roost/platform/tracker"
)

// Effect is a descriptor of I/O the shell must perform on behalf of the pure core.
// Reduce returns Effects as data; the scheduler loop (the imperative shell) interprets
// them, performs the actual I/O, and feeds the result back as an Event. Effects that
// produce a result name the Event they yield in their doc comment.
type Effect interface{ isSchedulerEffect() }

// EffFetchCandidates fetches the dispatch candidate list → EvCandidatesFetched.
type EffFetchCandidates struct{}

// EffRefreshTracker re-fetches current tracker state for running issues → EvTrackerRefreshed.
type EffRefreshTracker struct{ IDs []string }

// EffRevalidate re-verifies an issue's state immediately before spawn → EvRevalidated (SPEC §16.4).
type EffRevalidate struct {
	Issue   tracker.Issue
	Attempt int
}

// EffRetryFetch fetches candidates when a retry timer fired → EvRetryResolved (SPEC §8.4).
type EffRetryFetch struct {
	IssueID    string
	Identifier string
	Attempt    int
}

// EffSpawn launches an agent worker → EvSpawned / EvSpawnFailed.
type EffSpawn struct {
	Issue   tracker.Issue
	Attempt int
}

// EffKillWorker stops the live worker for an issue (fire-and-forget; the worker's own
// teardown later yields EvWorkerExit, which is idempotent for an already-removed issue).
type EffKillWorker struct {
	IssueID string
	Reason  string
}

// EffRemoveWorkspace removes a per-issue workspace directory (SPEC §8.5 terminal cleanup).
type EffRemoveWorkspace struct{ Identifier string }

// EffArmRetryTimer arms (or re-arms) the retry timer for an issue → EvRetryDue after Delay.
type EffArmRetryTimer struct {
	IssueID    string
	Identifier string
	Attempt    int
	Delay      time.Duration
}

// EffCancelRetryTimer stops and clears any pending retry timer for an issue.
type EffCancelRetryTimer struct{ IssueID string }

func (EffFetchCandidates) isSchedulerEffect()  {}
func (EffRefreshTracker) isSchedulerEffect()   {}
func (EffRevalidate) isSchedulerEffect()       {}
func (EffRetryFetch) isSchedulerEffect()       {}
func (EffSpawn) isSchedulerEffect()            {}
func (EffKillWorker) isSchedulerEffect()       {}
func (EffRemoveWorkspace) isSchedulerEffect()  {}
func (EffArmRetryTimer) isSchedulerEffect()    {}
func (EffCancelRetryTimer) isSchedulerEffect() {}
