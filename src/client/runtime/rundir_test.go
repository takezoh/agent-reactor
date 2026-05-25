package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeBridge creates a dummy roost-bridge executable in dir and returns its path.
func fakeBridge(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "roost-bridge")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho bridge\n"), 0o755); err != nil {
		t.Fatalf("fakeBridge: %v", err)
	}
	return p
}

func TestInstallBinaryInRunDir(t *testing.T) {
	srcDir := t.TempDir()
	runDir := t.TempDir()

	src := fakeBridge(t, srcDir)
	containerPath, err := installBridgeInRunDir(src, runDir)
	if err != nil {
		t.Fatalf("installBridgeInRunDir: %v", err)
	}

	if containerPath != ContainerBinaryPath {
		t.Errorf("container path = %q, want %q", containerPath, ContainerBinaryPath)
	}

	dst := filepath.Join(runDir, "roost-bridge")
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
	srcDir := t.TempDir()
	runDir := t.TempDir()

	src := fakeBridge(t, srcDir)
	if _, err := installBridgeInRunDir(src, runDir); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := installBridgeInRunDir(src, runDir); err != nil {
		t.Fatalf("second install: %v", err)
	}

	dst := filepath.Join(runDir, "roost-bridge")
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("stat after second install: %v", err)
	}
}
