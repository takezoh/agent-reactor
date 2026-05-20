package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

// §17.2: missing directory is created; path returned.
func TestEnsure_CreatesDirectory(t *testing.T) {
	m := newTestManager(t)
	p, err := m.Ensure(context.Background(), "issue-1")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	info, statErr := os.Stat(p)
	if statErr != nil || !info.IsDir() {
		t.Errorf("expected directory at %q, stat: %v", p, statErr)
	}
}

// §17.2: existing directory is reused without error.
func TestEnsure_ReusesExistingDirectory(t *testing.T) {
	m := newTestManager(t)
	p1, err := m.Ensure(context.Background(), "issue-1")
	if err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	p2, err := m.Ensure(context.Background(), "issue-1")
	if err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if p1 != p2 {
		t.Errorf("paths differ: %q vs %q", p1, p2)
	}
}

// §17.2: non-directory file at the workspace path must fail safely.
func TestEnsure_NonDirectoryFails(t *testing.T) {
	m := newTestManager(t)
	p, _ := m.Path("issue-file")
	if err := os.WriteFile(p, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := m.Ensure(context.Background(), "issue-file")
	if !errors.Is(err, ErrNotDirectory) {
		t.Errorf("Ensure on file err = %v, want ErrNotDirectory", err)
	}
}

// §17.2: after_create runs only when newly created, not on reuse.
func TestEnsure_AfterCreate_NewOnly(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks: wfconfig.HooksConfig{
			TimeoutMS:   5000,
			AfterCreate: fmt.Sprintf("touch %s", marker),
		},
	})

	// First call: new workspace — after_create should fire.
	if _, err := m.Ensure(context.Background(), "issue-1"); err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("marker not created by after_create: %v", statErr)
	}

	// Remove marker, then reuse — after_create must not fire again.
	os.Remove(marker)
	if _, err := m.Ensure(context.Background(), "issue-1"); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("after_create fired on workspace reuse — must not happen")
	}
}

// §9.4: after_create failure makes Ensure fatal.
func TestEnsure_AfterCreate_FailureFatal(t *testing.T) {
	root := t.TempDir()
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks: wfconfig.HooksConfig{
			TimeoutMS:   5000,
			AfterCreate: "exit 1",
		},
	})
	_, err := m.Ensure(context.Background(), "issue-fail")
	if !errors.Is(err, ErrHookFailed) {
		t.Errorf("Ensure with failing after_create err = %v, want ErrHookFailed", err)
	}
}

// §17.2: Remove deletes the workspace directory.
func TestRemove_DeletesDirectory(t *testing.T) {
	m := newTestManager(t)
	p, err := m.Ensure(context.Background(), "issue-1")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if err := m.Remove(context.Background(), "issue-1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, statErr := os.Stat(p); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("directory should be gone after Remove, stat: %v", statErr)
	}
}

// §9.4: before_remove failure is ignored; deletion still proceeds.
func TestRemove_BeforeRemoveFailureIgnored(t *testing.T) {
	root := t.TempDir()
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks: wfconfig.HooksConfig{
			TimeoutMS:    5000,
			BeforeRemove: "exit 1",
		},
	})
	p, err := m.Ensure(context.Background(), "issue-1")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if err := m.Remove(context.Background(), "issue-1"); err != nil {
		t.Errorf("Remove must succeed despite before_remove failure: %v", err)
	}
	if _, statErr := os.Stat(p); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("directory should be deleted despite hook failure")
	}
}
