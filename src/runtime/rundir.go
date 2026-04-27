package runtime

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectRunDir returns the per-project ephemeral run directory path.
// The directory is bind-mounted into the devcontainer at /opt/roost/run.
// Its inode is stable across daemon restarts; only the files inside change.
func ProjectRunDir(runBase, projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return filepath.Join(runBase, fmt.Sprintf("%x", h[:6]))
}

// EnsureProjectRunDir creates the per-project run directory.
// Returns the run dir path.
func EnsureProjectRunDir(runBase, projectPath string) (string, error) {
	dir := ProjectRunDir(runBase, projectPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("rundir: mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// ContainerSockPath returns the Unix socket path for the container endpoint
// inside the given run directory. This socket is bind-mounted into the
// devcontainer at /opt/roost/run/roost.sock.
func ContainerSockPath(runDir string) string {
	return filepath.Join(runDir, "roost.sock")
}
