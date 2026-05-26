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
	p, err := m.Ensure(context.Background(), "issue-1", "")
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
	p1, err := m.Ensure(context.Background(), "issue-1", "")
	if err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	p2, err := m.Ensure(context.Background(), "issue-1", "")
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
	_, err := m.Ensure(context.Background(), "issue-file", "")
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
	if _, err := m.Ensure(context.Background(), "issue-1", ""); err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("marker not created by after_create: %v", statErr)
	}

	// Remove marker, then reuse — after_create must not fire again.
	os.Remove(marker)
	if _, err := m.Ensure(context.Background(), "issue-1", ""); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("after_create fired on workspace reuse — must not happen")
	}
}

// after_create receives the per-project branch via ROOST_PROJECT_BRANCH.
func TestEnsure_AfterCreate_ReceivesProjectBranch(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "branch.txt")
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks: wfconfig.HooksConfig{
			TimeoutMS:   5000,
			AfterCreate: fmt.Sprintf("printf '%%s' \"$ROOST_PROJECT_BRANCH\" > %s", out),
		},
	})
	if _, err := m.Ensure(context.Background(), "issue-1", "develop"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read hook output: %v", err)
	}
	if string(got) != "develop" {
		t.Errorf("ROOST_PROJECT_BRANCH = %q, want develop", string(got))
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
	_, err := m.Ensure(context.Background(), "issue-fail", "")
	if !errors.Is(err, ErrHookFailed) {
		t.Errorf("Ensure with failing after_create err = %v, want ErrHookFailed", err)
	}
}

// §9.3: a failed after_create removes the just-created workspace so a later
// Ensure re-creates it and re-runs after_create (no half-initialized dir).
func TestEnsure_AfterCreate_FailureRemovesDir(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "marker")
	// Hook fails on the first run, then succeeds once the marker exists.
	script := fmt.Sprintf("test -f %s || { touch %s; exit 1; }", marker, marker)
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks:     wfconfig.HooksConfig{TimeoutMS: 5000, AfterCreate: script},
	})

	p, _ := m.Path("issue-1")
	if _, err := m.Ensure(context.Background(), "issue-1", ""); !errors.Is(err, ErrHookFailed) {
		t.Fatalf("first Ensure err = %v, want ErrHookFailed", err)
	}
	if _, statErr := os.Stat(p); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("workspace must be removed after after_create failure, stat: %v", statErr)
	}

	// Retry: dir was removed, so createdNow=true again and after_create re-runs (now succeeds).
	if _, err := m.Ensure(context.Background(), "issue-1", ""); err != nil {
		t.Fatalf("retry Ensure should succeed once after_create passes: %v", err)
	}
	if _, statErr := os.Stat(p); statErr != nil {
		t.Errorf("workspace should exist after successful retry, stat: %v", statErr)
	}
}

// §17.2: Remove deletes the workspace directory.
func TestRemove_DeletesDirectory(t *testing.T) {
	m := newTestManager(t)
	p, err := m.Ensure(context.Background(), "issue-1", "")
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
	p, err := m.Ensure(context.Background(), "issue-1", "")
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
