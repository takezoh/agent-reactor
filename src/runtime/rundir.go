package runtime

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Container-side paths for files bind-mounted from the per-project run dir.
// These are the canonical sources; callers must not hard-code these literals.
const (
	ContainerRunDir       = "/opt/roost/run"
	ContainerBinaryPath   = ContainerRunDir + "/roost"
	ContainerSockFileName = "roost.sock"
	ContainerSockFilePath = ContainerRunDir + "/" + ContainerSockFileName
)

// ProjectRunDir returns the per-project ephemeral run directory path.
// The directory is bind-mounted into the devcontainer at ContainerRunDir.
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

// ContainerSockPath returns the host-side Unix socket path for the container
// endpoint inside the given run directory. The same socket appears inside the
// container at ContainerSockFilePath via the run-dir bind mount.
func ContainerSockPath(runDir string) string {
	return filepath.Join(runDir, ContainerSockFileName)
}

// InstallBinaryInRunDir copies the current roost executable into runDir as
// "roost" (mode 0o755). The file is bind-mounted into the devcontainer at
// ContainerRunDir, so the copy is accessible inside the container as
// ContainerBinaryPath. The copy is skipped when an existing dst already
// matches the source size+mtime. Returns the container-internal path.
func InstallBinaryInRunDir(runDir string) (string, error) {
	src, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("rundir: executable: %w", err)
	}
	if resolved, e := filepath.EvalSymlinks(src); e == nil {
		src = resolved
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("rundir: stat src: %w", err)
	}

	dst := filepath.Join(runDir, "roost")
	if dstInfo, err := os.Stat(dst); err == nil &&
		dstInfo.Size() == srcInfo.Size() &&
		dstInfo.ModTime().Equal(srcInfo.ModTime()) {
		return ContainerBinaryPath, nil
	}

	if err := copyFile(src, dst, 0o755); err != nil {
		return "", err
	}
	// Mirror src mtime so the next call can short-circuit via the size+mtime check.
	_ = os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	return ContainerBinaryPath, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("rundir: open binary: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("rundir: create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("rundir: copy binary: %w", err)
	}
	return nil
}
