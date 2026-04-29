package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallBinaryInRunDir(t *testing.T) {
	runDir := t.TempDir()

	containerPath, err := InstallBinaryInRunDir(runDir)
	if err != nil {
		t.Fatalf("InstallBinaryInRunDir: %v", err)
	}

	if containerPath != ContainerBinaryPath {
		t.Errorf("container path = %q, want %q", containerPath, ContainerBinaryPath)
	}

	dst := filepath.Join(runDir, "roost")
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat %s: %v", dst, err)
	}
	if info.Size() == 0 {
		t.Error("copied binary is empty")
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("copied binary not executable, mode = %v", info.Mode())
	}
}

func TestInstallBinaryInRunDir_Idempotent(t *testing.T) {
	runDir := t.TempDir()

	if _, err := InstallBinaryInRunDir(runDir); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := InstallBinaryInRunDir(runDir); err != nil {
		t.Fatalf("second install: %v", err)
	}

	dst := filepath.Join(runDir, "roost")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("stat after second install: %v", err)
	}
}
