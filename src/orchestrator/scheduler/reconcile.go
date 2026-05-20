package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

// ErrStall is recorded in a RetryEntry when a worker is killed due to stall timeout.
var ErrStall = errors.New("stall timeout exceeded")

// reconcile runs Part A (stall detection) and Part B (tracker state refresh) per SPEC §8.5.
// Called at the start of each tick, regardless of preflight status.
func (s *Scheduler) reconcile(ctx context.Context, cfg wfconfig.Config) {
	snap := s.state.Snapshot()
	if len(snap.Running) == 0 {
		return
	}

	s.reconcileStall(snap, cfg)
	if s.tracker != nil && s.workspace != nil {
		s.reconcileRefresh(ctx, snap, cfg)
	}
}

// reconcileStall kills workers that have exceeded stall_timeout_ms (§8.5 Part A).
func (s *Scheduler) reconcileStall(snap StateSnapshot, cfg wfconfig.Config) {
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
			"issue_id", id, "identifier", run.Issue.Identifier, "elapsed_ms", now.Sub(base).Milliseconds())

		if run.Session.Worker != nil {
			if err := run.Session.Worker.Kill("stall"); err != nil {
				slog.Warn("reconcile: stall kill failed", "issue_id", id, "err", err)
			}
		}

		if entry, ok := s.state.WorkerExitAbnormal(id, ErrStall, run.Attempt); ok {
			s.state.EnqueueRetry(entry)
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

	for id, run := range snap.Running {
		iss, found := byID[id]
		if !found {
			continue
		}

		switch {
		case slices.Contains(cfg.Tracker.TerminalStates, iss.State):
			if run.Session.Worker != nil {
				if err := run.Session.Worker.Kill("terminal"); err != nil {
					slog.Warn("reconcile: terminal kill failed", "issue_id", id, "err", err)
				}
			}
			s.state.ReleaseClaim(id)
			if err := s.workspace.Remove(ctx, run.Issue.Identifier); err != nil {
				slog.Warn("reconcile: workspace remove failed", "identifier", run.Issue.Identifier, "err", err)
			}
			slog.Info("reconcile: terminal issue cleaned up",
				"issue_id", id, "identifier", run.Issue.Identifier, "state", iss.State)

		case slices.Contains(cfg.Tracker.ActiveStates, iss.State):
			s.state.UpdateIssueSnapshot(id, iss)

		default:
			// Intermediate state: stop worker but keep workspace.
			if run.Session.Worker != nil {
				if err := run.Session.Worker.Kill("non-active"); err != nil {
					slog.Warn("reconcile: non-active kill failed", "issue_id", id, "err", err)
				}
			}
			if entry, ok := s.state.WorkerExitAbnormal(id, errors.New("issue left active states"), run.Attempt); ok {
				s.state.EnqueueRetry(entry)
			}
			slog.Info("reconcile: non-active issue, worker stopped",
				"issue_id", id, "identifier", run.Issue.Identifier, "state", iss.State)
		}
	}
}
