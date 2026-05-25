package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// exec interprets one Effect into real I/O. Effects that yield a result feed it back as an
// Event via step (synchronously), so a tick drains depth-first to completion before the loop
// selects the next external event — preserving the pre-refactor reconcile-then-dispatch order.
func (s *Scheduler) exec(ctx context.Context, eff Effect) {
	switch e := eff.(type) {
	case EffFetchCandidates:
		s.execFetchCandidates(ctx)
	case EffRefreshTracker:
		s.execRefreshTracker(ctx, e)
	case EffRevalidate:
		s.execRevalidate(ctx, e)
	case EffRetryFetch:
		s.execRetryFetch(ctx, e)
	case EffSpawn:
		s.execSpawn(ctx, e)
	case EffKillWorker:
		s.execKillWorker(e)
	case EffRemoveWorkspace:
		s.execRemoveWorkspace(ctx, e)
	case EffArmRetryTimer:
		s.armTimer(ctx, e.IssueID, e.Identifier, e.Attempt, e.Delay)
	case EffCancelRetryTimer:
		s.cancelTimer(e.IssueID)
	}
}

func (s *Scheduler) execFetchCandidates(ctx context.Context) {
	if s.deps.Tracker == nil || s.deps.Spawn == nil {
		return // dispatch not wired
	}
	cands, err := s.deps.Tracker.Candidates(ctx)
	if err != nil {
		slog.Error("tick: candidates fetch failed", "err", err)
		return
	}
	s.step(ctx, EvCandidatesFetched{Issues: cands})
}

func (s *Scheduler) execRefreshTracker(ctx context.Context, e EffRefreshTracker) {
	if s.tracker == nil || s.workspace == nil {
		return // reconcile Part B not wired
	}
	issues, err := s.tracker.RefreshStates(ctx, e.IDs)
	if err != nil {
		slog.Warn("reconcile: RefreshStates failed, skipping refresh", "err", err)
		return
	}
	s.step(ctx, EvTrackerRefreshed{RequestedIDs: e.IDs, Issues: issues})
}

func (s *Scheduler) execRevalidate(ctx context.Context, e EffRevalidate) {
	if s.tracker == nil {
		s.step(ctx, EvRevalidated{Issue: e.Issue, Attempt: e.Attempt, Fresh: &e.Issue})
		return
	}
	fresh, err := revalidateIssue(ctx, s.tracker, e.Issue.ID)
	if err != nil {
		slog.Warn("dispatch: revalidation error, skipping", "issue_id", e.Issue.ID, "err", err)
		fresh = nil
	}
	s.step(ctx, EvRevalidated{Issue: e.Issue, Attempt: e.Attempt, Fresh: fresh})
}

func (s *Scheduler) execRetryFetch(ctx context.Context, e EffRetryFetch) {
	if s.deps.Tracker == nil || s.deps.Spawn == nil {
		return
	}
	cands, err := s.deps.Tracker.Candidates(ctx)
	s.step(ctx, EvRetryResolved{
		IssueID:    e.IssueID,
		Identifier: e.Identifier,
		Attempt:    e.Attempt,
		Issues:     cands,
		FetchErr:   err,
	})
}

func (s *Scheduler) execSpawn(ctx context.Context, e EffSpawn) {
	if s.deps.Spawn == nil {
		return
	}
	res, err := s.deps.Spawn(ctx, e.Issue, e.Attempt)
	if err != nil {
		slog.Error("spawn failed", "issue_id", e.Issue.ID, "issue_identifier", e.Issue.Identifier, "err", err)
		s.step(ctx, EvSpawnFailed{Issue: e.Issue, Attempt: e.Attempt, Err: err})
		return
	}
	if res.Worker != nil {
		s.workers[e.Issue.ID] = res.Worker
	}
	slog.Info("dispatched", "issue_id", e.Issue.ID, "issue_identifier", e.Issue.Identifier, "attempt", e.Attempt)
	s.step(ctx, EvSpawned{Issue: e.Issue, Attempt: e.Attempt, Session: res.Session})
}

// execKillWorker stops a live worker fire-and-forget. Kill blocks until the subprocess exits,
// so it runs in a goroutine to avoid stalling the loop; the worker's own teardown later sends
// a WorkerExit (idempotent for the already-transitioned issue).
func (s *Scheduler) execKillWorker(e EffKillWorker) {
	w, ok := s.workers[e.IssueID]
	if !ok {
		return
	}
	delete(s.workers, e.IssueID)
	go func() {
		if err := w.Kill(e.Reason); err != nil {
			slog.Warn("reconcile: kill failed", "issue_id", e.IssueID, "reason", e.Reason, "err", err)
		}
	}()
}

func (s *Scheduler) execRemoveWorkspace(ctx context.Context, e EffRemoveWorkspace) {
	if s.workspace == nil {
		return
	}
	if err := s.workspace.Remove(ctx, e.Identifier); err != nil {
		slog.Warn("reconcile: workspace remove failed", "identifier", e.Identifier, "err", err)
	}
}

// armTimer arms (or re-arms) the retry timer for an issue; on fire it delivers a retryFireReq
// to the Run loop, which folds it as EvRetryDue.
func (s *Scheduler) armTimer(ctx context.Context, issueID, identifier string, attempt int, delay time.Duration) {
	if old, ok := s.timers[issueID]; ok {
		old.Stop()
	}
	req := retryFireReq{IssueID: issueID, Identifier: identifier, Attempt: attempt}
	s.timers[issueID] = s.clock.NewTimer(delay, func() {
		select {
		case s.retryFire <- req:
		case <-ctx.Done():
		}
	})
	slog.Info("retry scheduled", "issue_id", issueID, "attempt", attempt, "delay_ms", delay.Milliseconds())
}

func (s *Scheduler) cancelTimer(issueID string) {
	if t, ok := s.timers[issueID]; ok {
		t.Stop()
		delete(s.timers, issueID)
	}
}
