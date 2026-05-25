package scheduler

import (
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

// Reduce is the pure functional core of the scheduler. It folds one Event into the
// next State and returns the Effects the shell must perform. It is a pure function:
// no I/O, no goroutines, no mutexes, no wall-clock reads — time enters as the `now`
// parameter. Given the same (s, ev, cfg, now) it always returns the same result, so
// the entire decision surface is table-testable without fakes.
func Reduce(s State, ev Event, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	switch e := ev.(type) {
	case EvTick:
		return reduceTick(s, e, cfg, now)
	case EvTrackerRefreshed:
		return reduceTrackerRefreshed(s, e, cfg, now)
	case EvCandidatesFetched:
		return reduceCandidates(s, e.Issues, cfg)
	case EvRevalidated:
		return reduceRevalidated(s, e, cfg)
	case EvRetryDue:
		return s, []Effect{EffRetryFetch(e)}
	case EvRetryResolved:
		return reduceRetryResolved(s, e, cfg, now)
	case EvSpawned:
		return markRunning(s, e.Issue, e.Attempt, e.Session), nil
	case EvSpawnFailed:
		return reduceSpawnFailed(s, e, cfg, now)
	case EvWorkerExit:
		return reduceWorkerExit(s, e, cfg, now)
	case EvCodexActivity:
		return reduceCodexActivity(s, e), nil
	default:
		return s, nil
	}
}

// reduceTick runs reconcile Part A (stall detection) then schedules reconcile Part B
// (tracker refresh) and, when the workflow is valid, a dispatch candidate fetch (SPEC §8.1).
func reduceTick(s State, e EvTick, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	var effs []Effect
	s, effs = reduceStall(s, cfg, now)

	if len(s.Running) > 0 {
		effs = append(effs, EffRefreshTracker{IDs: sortedRunningIDs(s)})
	}
	if e.ConfigValid {
		effs = append(effs, EffFetchCandidates{})
	}
	return s, effs
}

// reduceWorkerExit processes an agent runner's turn-loop ending (SPEC §16.6).
// A clean exit schedules a continuation retry; an abnormal exit schedules a backoff
// retry. An exit for an issue no longer running is a no-op (idempotent — reconcile may
// have already transitioned it).
func reduceWorkerExit(s State, e EvWorkerExit, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	if e.Err == nil {
		ns, entry, ok := workerExitNormal(s, e.IssueID)
		if !ok {
			return s, nil
		}
		return armRetry(ns, entry, continuationDelay, now)
	}
	ns, entry, ok := workerExitAbnormal(s, e.IssueID, e.Err, e.Attempt)
	if !ok {
		return s, nil
	}
	return armRetry(ns, entry, backoffDelay(entry.Attempt, cfg), now)
}

// reduceSpawnFailed releases the claim and schedules a backoff retry (SPEC §16.4).
func reduceSpawnFailed(s State, e EvSpawnFailed, cfg wfconfig.Config, now time.Time) (State, []Effect) {
	s = releaseClaim(s, e.Issue.ID)
	entry := RetryEntry{
		IssueID:    e.Issue.ID,
		Identifier: e.Issue.Identifier,
		Attempt:    e.Attempt + 1,
		Kind:       RetryBackoff,
		Err:        e.Err,
	}
	return armRetry(s, entry, backoffDelay(entry.Attempt, cfg), now)
}

// reduceCodexActivity folds a codex notification into the running attempt's bookkeeping
// (SPEC §13.5). Emits no effects.
func reduceCodexActivity(s State, e EvCodexActivity) State {
	s = updateCodexActivity(s, e.IssueID, e.Event, e.Message, e.Timestamp)
	if e.Usage != nil {
		s = recordUsage(s, e.IssueID, *e.Usage)
	}
	if e.RateLimit != nil {
		s = recordRateLimit(s, e.IssueID, *e.RateLimit)
	}
	if e.TurnDuration != nil {
		s = addRuntime(s, e.IssueID, *e.TurnDuration)
	}
	if e.TurnCompleted {
		s = incrementTurnCount(s, e.IssueID)
	}
	return s
}

// armRetry records the retry entry (with its due time) and emits the timer-arm effect.
func armRetry(s State, entry RetryEntry, delay time.Duration, now time.Time) (State, []Effect) {
	entry.DueAtMS = now.Add(delay).UnixMilli()
	s = enqueueRetry(s, entry)
	return s, []Effect{EffArmRetryTimer{
		IssueID:    entry.IssueID,
		Identifier: entry.Identifier,
		Attempt:    entry.Attempt,
		Delay:      delay,
	}}
}
