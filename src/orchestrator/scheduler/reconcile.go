package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

var (
	// ErrStall is recorded in a RetryEntry when a worker is killed due to stall timeout.
	ErrStall = errors.New("stall timeout exceeded")
	// ErrNotInRefresh is recorded when an issue disappears from the tracker refresh response (SPEC §8.5).
	ErrNotInRefresh = errors.New("issue not in refresh response")
	// ErrLeftActiveStates is recorded when an issue transitions to a non-active, non-terminal state.
	ErrLeftActiveStates = errors.New("issue left active states")
)

// reconcile runs Part A (stall detection) and Part B (tracker state refresh) per SPEC §8.5.
// Called at the start of each tick, regardless of preflight status.
func (s *Scheduler) reconcile(ctx context.Context, cfg wfconfig.Config) {
	snap := s.state.Snapshot()
	if len(snap.Running) == 0 {
		return
	}

	s.reconcileStall(ctx, snap, cfg)
	if s.tracker != nil && s.workspace != nil {
		// Re-snapshot after stall processing: workers killed by stall must not be double-processed.
		s.reconcileRefresh(ctx, s.state.Snapshot(), cfg)
	}
}

// reconcileStall kills workers that have exceeded stall_timeout_ms (§8.5 Part A).
func (s *Scheduler) reconcileStall(ctx context.Context, snap StateSnapshot, cfg wfconfig.Config) {
	if cfg.Codex.StallTimeoutMS <= 0 {
		return
	}
	threshold := time.Duration(cfg.Codex.StallTimeoutMS) * time.Millisecond
	now := s.clock.Now()

	for id, run := range snap.Running {
		base := run.LastCodexTimestamp
		if base.IsZero() {
			base = run.StartedAt
		}
		if base.IsZero() || now.Sub(base) <= threshold {
			continue
		}

		slog.Warn("reconcile: stall detected, killing worker",
			"issue_id", id, "issue_identifier", run.Issue.Identifier, "elapsed_ms", now.Sub(base).Milliseconds())

		// Kill blocks until the subprocess exits; run in a goroutine to avoid
		// stalling the scheduler event loop while waiting for N processes.
		if run.Session.Worker != nil {
			w := run.Session.Worker
			issueID := id
			go func() {
				if err := w.Kill("stall"); err != nil {
					slog.Warn("reconcile: stall kill failed", "issue_id", issueID, "err", err)
				}
			}()
		}

		if entry, ok := s.state.WorkerExitAbnormal(id, ErrStall, run.Attempt); ok {
			scheduleRetry(s.state, s.clock, s.retryFire, ctx, entry, backoffDelay(entry.Attempt, cfg))
		}
	}
}

// reconcileRefresh fetches current tracker states and acts on transitions (§8.5 Part B).
func (s *Scheduler) reconcileRefresh(ctx context.Context, snap StateSnapshot, cfg wfconfig.Config) {
	ids := make([]string, 0, len(snap.Running))
	for id := range snap.Running {
		ids = append(ids, id)
	}

	issues, err := s.tracker.RefreshStates(ctx, ids)
	if err != nil {
		slog.Warn("reconcile: RefreshStates failed, skipping refresh", "err", err)
		return
	}

	byID := make(map[string]ptrackerv.Issue, len(issues))
	for _, iss := range issues {
		byID[iss.ID] = iss
	}

	// State matching is case-insensitive, consistent with dispatch eligibility (§8.2).
	terminal := normSet(cfg.Tracker.TerminalStates)
	active := normSet(cfg.Tracker.ActiveStates)

	for id, run := range snap.Running {
		iss, found := byID[id]
		if !found {
			// Issue disappeared from tracker response: stop worker but keep workspace (SPEC §8.5).
			if run.Session.Worker != nil {
				w := run.Session.Worker
				issueID := id
				go func() {
					if err := w.Kill("not-found"); err != nil {
						slog.Warn("reconcile: not-found kill failed", "issue_id", issueID, "err", err)
					}
				}()
			}
			if entry, ok := s.state.WorkerExitAbnormal(id, ErrNotInRefresh, run.Attempt); ok {
				scheduleRetry(s.state, s.clock, s.retryFire, ctx, entry, backoffDelay(entry.Attempt, cfg))
			}
			slog.Info("reconcile: issue not in refresh response, worker stopped",
				"issue_id", id, "identifier", run.Issue.Identifier)
			continue
		}

		norm := strings.ToLower(iss.State)
		switch {
		case terminal[norm]:
			if run.Session.Worker != nil {
				w := run.Session.Worker
				issueID := id
				go func() {
					if err := w.Kill("terminal"); err != nil {
						slog.Warn("reconcile: terminal kill failed", "issue_id", issueID, "err", err)
					}
				}()
			}
			s.state.ReleaseClaim(id)
			if err := s.workspace.Remove(ctx, run.Issue.Identifier); err != nil {
				slog.Warn("reconcile: workspace remove failed", "identifier", run.Issue.Identifier, "err", err)
			}
			slog.Info("reconcile: terminal issue cleaned up",
				"issue_id", id, "issue_identifier", run.Issue.Identifier, "state", iss.State)

		case active[norm]:
			s.state.UpdateIssueSnapshot(id, iss)

		default:
			// Intermediate state: stop worker but keep workspace.
			if run.Session.Worker != nil {
				w := run.Session.Worker
				issueID := id
				go func() {
					if err := w.Kill("non-active"); err != nil {
						slog.Warn("reconcile: non-active kill failed", "issue_id", issueID, "err", err)
					}
				}()
			}
			if entry, ok := s.state.WorkerExitAbnormal(id, ErrLeftActiveStates, run.Attempt); ok {
				scheduleRetry(s.state, s.clock, s.retryFire, ctx, entry, backoffDelay(entry.Attempt, cfg))
			}
			slog.Info("reconcile: non-active issue, worker stopped",
				"issue_id", id, "issue_identifier", run.Issue.Identifier, "state", iss.State)
		}
	}
}
