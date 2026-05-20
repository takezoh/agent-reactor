package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// BeforeRun runs the before_run hook before each agent attempt.
// Failure or timeout is fatal: the returned error signals the attempt must be aborted per §9.4.
func (m *Manager) BeforeRun(ctx context.Context, identifier string) error {
	p, err := m.Path(identifier)
	if err != nil {
		return err
	}
	return m.runHook(ctx, "before_run", m.hooks.BeforeRun, p)
}

// AfterRun runs the after_run hook after each agent attempt.
// Failure and timeout are logged and ignored per §9.4.
func (m *Manager) AfterRun(ctx context.Context, identifier string) {
	p, err := m.Path(identifier)
	if err != nil {
		slog.Error("workspace: AfterRun path error", "identifier", identifier, "err", err)
		return
	}
	_ = m.runHook(ctx, "after_run", m.hooks.AfterRun, p)
}

// runHook executes script via "sh -lc <script>" with cwd set to the workspace path.
// The context deadline is extended by hooks.TimeoutMS regardless of the caller's deadline.
// Logs start, failure, and timeout. Returns ErrHookFailed on non-zero exit or timeout.
// An empty script is a no-op.
func (m *Manager) runHook(ctx context.Context, name, script, cwd string) error {
	if script == "" {
		return nil
	}

	timeout := time.Duration(m.hooks.TimeoutMS) * time.Millisecond
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	slog.Info("workspace: hook start", "hook", name, "cwd", cwd)

	cmd := exec.CommandContext(hctx, "sh", "-lc", script)
	cmd.Dir = cwd

	if err := cmd.Run(); err != nil {
		if errors.Is(hctx.Err(), context.DeadlineExceeded) {
			slog.Error("workspace: hook timeout", "hook", name, "timeout_ms", m.hooks.TimeoutMS)
			return fmt.Errorf("%w: %s: timeout after %dms", ErrHookFailed, name, m.hooks.TimeoutMS)
		}
		slog.Error("workspace: hook failed", "hook", name, "err", err)
		return fmt.Errorf("%w: %s: %v", ErrHookFailed, name, err)
	}
	return nil
}
