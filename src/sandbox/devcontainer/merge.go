package devcontainer

import (
	"errors"
	"os"
	"path/filepath"
)

const devcontainerSubdir = ".devcontainer"

// ErrNoProjectDevcontainer is returned when <project>/.devcontainer/devcontainer.json is not found.
var ErrNoProjectDevcontainer = errors.New("devcontainer: <project>/.devcontainer/devcontainer.json not found")

// ErrNoUserDevcontainer is returned when ~/.devcontainer/devcontainer.json is not found.
var ErrNoUserDevcontainer = errors.New("devcontainer: ~/.devcontainer/devcontainer.json not found")

// OverlayFunc computes per-project roost overlay (env + mounts) to apply at container
// creation time. dcDir is the resolved devcontainer config directory.
// Must not trigger image builds.
type OverlayFunc func(projectPath, dcDir string) (SpecOverlay, error)

// FindDevcontainerPath returns the devcontainer.json path for projectPath.
// Tries <project>/.devcontainer first; falls back to ~/.devcontainer.
func FindDevcontainerPath(projectPath string) (string, error) {
	dcPath, err := ProjectBaseDC(projectPath)
	if errors.Is(err, ErrNoProjectDevcontainer) {
		dcPath, err = UserBaseDC()
	}
	return dcPath, err
}

// ProjectBaseDC returns the path to <project>/.devcontainer/devcontainer.json.
// Returns ErrNoProjectDevcontainer if not found.
func ProjectBaseDC(projectPath string) (string, error) {
	return findDC(projectPath, ErrNoProjectDevcontainer)
}

// UserBaseDC returns the path to ~/.devcontainer/devcontainer.json.
// Returns ErrNoUserDevcontainer if not found.
func UserBaseDC() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoUserDevcontainer
	}
	return findDC(home, ErrNoUserDevcontainer)
}

func findDC(basePath string, notFoundErr error) (string, error) {
	p := filepath.Join(basePath, devcontainerSubdir, "devcontainer.json")
	if _, err := os.Stat(p); err != nil {
		return "", notFoundErr
	}
	return p, nil
}
