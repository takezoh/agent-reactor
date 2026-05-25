package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// Ensure idempotently creates the workspace directory for identifier and runs
// the after_create hook only when newly created per §9.2.
// Returns the absolute workspace path.
func (m *Manager) Ensure(ctx context.Context, identifier string) (string, error) {
	p, err := m.Path(identifier)
	if err != nil {
		return "", err
	}

	createdNow, err := ensureDir(p)
	if err != nil {
		return "", err
	}

	if createdNow {
		if hookErr := m.runHook(ctx, "after_create", m.hooks.AfterCreate, p); hookErr != nil {
			// §9.3: remove the partially-prepared new workspace so a retry
			// re-creates it and re-runs after_create (otherwise the dir would
			// persist and the next Ensure would skip the hook).
			_ = os.RemoveAll(p)
			return "", fmt.Errorf("after_create hook: %w", hookErr)
		}
	}
	return p, nil
}

// ensureDir creates dir if absent. Reports whether it was just created.
// Returns ErrNotDirectory if a non-directory file exists at the path.
func ensureDir(p string) (createdNow bool, err error) {
	info, statErr := os.Stat(p)
	if statErr == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("%w: %q", ErrNotDirectory, p)
		}
		return false, nil
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return false, statErr
	}
	if mkErr := os.MkdirAll(p, 0o755); mkErr != nil {
		return false, mkErr
	}
	return true, nil
}

// Remove runs the before_remove hook (failure logged, not fatal) then deletes
// the workspace directory per §9.4. Deletion happens regardless of hook outcome.
func (m *Manager) Remove(ctx context.Context, identifier string) error {
	p, err := m.Path(identifier)
	if err != nil {
		return err
	}
	_ = m.runHook(ctx, "before_remove", m.hooks.BeforeRemove, p)
	return os.RemoveAll(p)
}
