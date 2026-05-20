// Package workspace manages per-issue workspace directories and hooks per SPEC §9.
package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

var (
	// ErrPathEscapesRoot is returned when an identifier resolves outside the workspace root (§9.5 Inv2).
	ErrPathEscapesRoot = errors.New("workspace: path escapes workspace root")
	// ErrNotDirectory is returned when the workspace path exists but is not a directory.
	ErrNotDirectory = errors.New("workspace: path exists but is not a directory")
	// ErrCWDMismatch is returned when cwd does not equal the expected workspace path (§9.5 Inv1).
	ErrCWDMismatch = errors.New("workspace: cwd does not match workspace path")
	// ErrHookFailed is returned when a fatal hook exits non-zero or times out (§9.4).
	ErrHookFailed = errors.New("workspace: hook failed")
)

// Manager manages per-issue workspace directories and lifecycle hooks per SPEC §9.
type Manager struct {
	root  string
	hooks wfconfig.HooksConfig
}

// New constructs a Manager from cfg.Workspace and cfg.Hooks.
func New(cfg wfconfig.Config) *Manager {
	return &Manager{
		root:  filepath.Clean(cfg.Workspace.Root),
		hooks: cfg.Hooks,
	}
}

// Path computes the absolute workspace path for identifier.
// Enforces §9.5 Inv2 (root containment) and Inv3 (sanitized key).
func (m *Manager) Path(identifier string) (string, error) {
	key := sanitizeKey(identifier)
	p := filepath.Clean(filepath.Join(m.root, key))
	rel, err := filepath.Rel(m.root, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrPathEscapesRoot, identifier)
	}
	return p, nil
}

// sanitizeKey replaces any character outside [A-Za-z0-9._-] with '_' per §9.5 Inv3.
func sanitizeKey(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, s)
}

// VerifyCWD checks that cwd equals the workspace path for identifier per §9.5 Inv1.
// Call this before launching the agent subprocess.
func (m *Manager) VerifyCWD(identifier, cwd string) error {
	expected, err := m.Path(identifier)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCWDMismatch, err)
	}
	if filepath.Clean(abs) != expected {
		return fmt.Errorf("%w: got %q, want %q", ErrCWDMismatch, abs, expected)
	}
	return nil
}
