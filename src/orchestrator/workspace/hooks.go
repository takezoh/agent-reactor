package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

// projectBranchEnvVar is the name of the hook environment variable carrying the
// per-project base branch (empty when the project specifies no branch).
const projectBranchEnvVar = "ROOST_PROJECT_BRANCH"

// branchEnv builds the per-project hook environment. The variable is always
// defined so hooks can use ${ROOST_PROJECT_BRANCH:-<default>} for the fallback.
func branchEnv(branch string) []string {
	return []string{projectBranchEnvVar + "=" + branch}
}

// BeforeRun runs the before_run hook before each agent attempt.
// Failure or timeout is fatal: the returned error signals the attempt must be aborted per §9.4.
func (m *Manager) BeforeRun(ctx context.Context, identifier, branch string) error {
	p, err := m.Path(identifier)
	if err != nil {
		return err
	}
	return m.runHook(ctx, "before_run", m.hooks.BeforeRun, p, branchEnv(branch))
}

// AfterRun runs the after_run hook after each agent attempt.
// Failure and timeout are logged and ignored per §9.4.
func (m *Manager) AfterRun(ctx context.Context, identifier, branch string) {
	p, err := m.Path(identifier)
	if err != nil {
		slog.Error("workspace: AfterRun path error", "identifier", identifier, "err", err)
		return
	}
	_ = m.runHook(ctx, "after_run", m.hooks.AfterRun, p, branchEnv(branch))
}

// hookOutputMaxBytes is the maximum number of bytes logged from hook stdout/stderr (SPEC §15.4).
const hookOutputMaxBytes = 2048

// runHook executes script via "sh -lc <script>" with cwd set to the workspace path.
// The hook runs under min(caller deadline, hooks.TimeoutMS).
// extraEnv is appended to the inherited process environment.
// stdout/stderr are captured, truncated to hookOutputMaxBytes, and logged per §15.4.
// Returns ErrHookFailed on non-zero exit or timeout. An empty script is a no-op.
func (m *Manager) runHook(ctx context.Context, name, script, cwd string, extraEnv []string) error {
	if script == "" {
		return nil
	}

	timeout := time.Duration(m.hooks.TimeoutMS) * time.Millisecond
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	slog.Info("workspace: hook start", "hook", name, "cwd", cwd)

	cmd := exec.CommandContext(hctx, "sh", "-lc", script)
	cmd.Dir = cwd
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	out, err := cmd.CombinedOutput()
	output := string(truncateOutput(out, hookOutputMaxBytes))

	if err != nil {
		if errors.Is(hctx.Err(), context.DeadlineExceeded) {
			slog.Error("workspace: hook timeout", "hook", name, "timeout_ms", m.hooks.TimeoutMS, "output", output)
			return fmt.Errorf("%w: %s: timeout after %dms", ErrHookFailed, name, m.hooks.TimeoutMS)
		}
		slog.Error("workspace: hook failed", "hook", name, "err", err, "output", output)
		return fmt.Errorf("%w: %s: %v", ErrHookFailed, name, err)
	}

	slog.Debug("workspace: hook output", "hook", name, "output", output)
	return nil
}

func truncateOutput(b []byte, max int) []byte {
	if len(b) > max {
		return b[:max]
	}
	return b
}
