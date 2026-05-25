package scheduler

import (
	"time"

	"github.com/takezoh/agent-roost/platform/metrics"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// Event is an input to the pure Reduce function. Each event is a self-contained
// value produced either by the loop's own clock/tick or by an I/O result the shell
// fed back after executing an Effect. Reduce never performs I/O; it only folds
// events into the next State and emits Effects.
type Event interface{ isSchedulerEvent() }

// EvTick is the periodic poll trigger (SPEC §8.1). ConfigValid is false while the
// WORKFLOW.md is unparseable: reconcile still runs on last-known-good config, but
// dispatch is gated (§5.5).
type EvTick struct{ ConfigValid bool }

// EvCandidatesFetched delivers the dispatch candidate list (result of EffFetchCandidates).
type EvCandidatesFetched struct{ Issues []tracker.Issue }

// EvTrackerRefreshed delivers the reconcile Part-B refresh result (EffRefreshTracker).
// RequestedIDs is the set of running issue IDs that were requested, so the reducer can
// detect issues that disappeared from the response (SPEC §8.5).
type EvTrackerRefreshed struct {
	RequestedIDs []string
	Issues       []tracker.Issue
}

// EvRevalidated delivers the result of a pre-spawn revalidation (EffRevalidate, SPEC §16.4).
// Issue is the originally-claimed issue; Fresh is its re-fetched state (nil = missing/error).
type EvRevalidated struct {
	Issue   tracker.Issue
	Attempt int
	Fresh   *tracker.Issue
}

// EvRetryResolved delivers the candidate list fetched when a retry timer fired (EffRetryFetch).
// FetchErr is non-nil when the candidate fetch failed (transient): the issue is rescheduled
// rather than released, so it is not permanently lost (SPEC §8.4).
type EvRetryResolved struct {
	IssueID    string
	Identifier string
	Attempt    int
	Issues     []tracker.Issue
	FetchErr   error
}

// EvSpawned reports a successful spawn (EffSpawn). Session carries the pure identity;
// the live Worker handle is registered by the shell, not carried in the event.
type EvSpawned struct {
	Issue   tracker.Issue
	Attempt int
	Session LiveSession
}

// EvSpawnFailed reports a failed spawn (EffSpawn). Attempt is the attempt that failed.
type EvSpawnFailed struct {
	Issue   tracker.Issue
	Attempt int
	Err     error
}

// EvWorkerExit reports an agent runner's turn loop ending (SPEC §16.6).
// Err == nil indicates a clean exit; non-nil indicates an abnormal exit.
type EvWorkerExit struct {
	IssueID string
	Err     error
	Attempt int
}

// EvRetryDue reports that a retry timer fired (EffArmRetryTimer → shell timer → here).
type EvRetryDue struct {
	IssueID    string
	Identifier string
	Attempt    int
}

// EvCodexActivity carries a codex protocol notification (SPEC §10 / §13.5).
type EvCodexActivity struct {
	IssueID       string
	Event         string // codex notification method name
	Message       string // non-empty for item/agentMessage/delta events
	Timestamp     time.Time
	Usage         *metrics.Usage             // non-nil for thread/tokenUsage/updated
	RateLimit     *metrics.RateLimitSnapshot // non-nil for account/rateLimits/updated
	TurnDuration  *time.Duration             // non-nil for turn/completed (elapsed turn time)
	TurnCompleted bool                       // true when a turn/completed notification was received (SPEC §4.1.6)
}

func (EvTick) isSchedulerEvent()              {}
func (EvCandidatesFetched) isSchedulerEvent() {}
func (EvTrackerRefreshed) isSchedulerEvent()  {}
func (EvRevalidated) isSchedulerEvent()       {}
func (EvRetryResolved) isSchedulerEvent()     {}
func (EvSpawned) isSchedulerEvent()           {}
func (EvSpawnFailed) isSchedulerEvent()       {}
func (EvWorkerExit) isSchedulerEvent()        {}
func (EvRetryDue) isSchedulerEvent()          {}
func (EvCodexActivity) isSchedulerEvent()     {}
