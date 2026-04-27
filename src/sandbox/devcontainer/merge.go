package devcontainer

import (
	"errors"
	"os"
	"path/filepath"
)

// devcontainerSubdir is the conventional directory name for devcontainer config.
const devcontainerSubdir = ".devcontainer"

// ErrNoProjectDevcontainer is returned when <project>/.devcontainer/devcontainer.json is not found.
var ErrNoProjectDevcontainer = errors.New("devcontainer: <project>/.devcontainer/devcontainer.json not found")

// ErrNoUserDevcontainer is returned when ~/.devcontainer/devcontainer.json is not found.
var ErrNoUserDevcontainer = errors.New("devcontainer: ~/.devcontainer/devcontainer.json not found")

// OverlayFunc computes per-project roost overlay (env + mounts) to apply at container
// creation time. dcDir is the resolved devcontainer config directory.
// Must not trigger image builds.
type OverlayFunc func(projectPath, dcDir string) (SpecOverlay, error)

// ProjectBaseDC returns the path to <project>/.devcontainer/devcontainer.json.
// Returns ErrNoProjectDevcontainer if not found.
func ProjectBaseDC(projectPath string) (string, error) {
	p := filepath.Join(projectPath, devcontainerSubdir, "devcontainer.json")
	if _, err := os.Stat(p); err != nil {
		return "", ErrNoProjectDevcontainer
	}
	return p, nil
}

// UserBaseDC returns the path to ~/.devcontainer/devcontainer.json.
// Returns ErrNoUserDevcontainer if not found.
func UserBaseDC() (string, error) {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, devcontainerSubdir, "devcontainer.json")
	if _, err := os.Stat(p); err != nil {
		return "", ErrNoUserDevcontainer
	}
	return p, nil
}
