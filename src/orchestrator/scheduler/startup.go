package scheduler

import (
	"context"
	"log/slog"
)

// StartupCleanup removes workspaces for terminal issues at scheduler boot (SPEC §8.6 / §16.3).
// Fetch failure is non-fatal: a warning is logged and startup continues.
// No-op if tracker or workspace deps are not wired.
func (s *Scheduler) StartupCleanup(ctx context.Context) {
	if s.tracker == nil || s.workspace == nil {
		return
	}
	issues, err := s.tracker.TerminalIssues(ctx)
	if err != nil {
		slog.Warn("startup cleanup: terminal issues fetch failed, skipping", "err", err)
		return
	}

	for _, iss := range issues {
		if err := s.workspace.Remove(ctx, iss.Identifier); err != nil {
			slog.Warn("startup cleanup: workspace remove failed", "identifier", iss.Identifier, "err", err)
		} else {
			slog.Info("startup cleanup: removed terminal workspace", "identifier", iss.Identifier)
		}
	}
}
