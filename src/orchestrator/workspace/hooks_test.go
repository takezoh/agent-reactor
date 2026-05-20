package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

// §17.2: before_run failure aborts the attempt.
func TestBeforeRun_FailureReturnsError(t *testing.T) {
	root := t.TempDir()
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks:     wfconfig.HooksConfig{TimeoutMS: 5000, BeforeRun: "exit 1"},
	})
	if _, err := m.Ensure(context.Background(), "issue-1"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	err := m.BeforeRun(context.Background(), "issue-1")
	if !errors.Is(err, ErrHookFailed) {
		t.Errorf("BeforeRun failure err = %v, want ErrHookFailed", err)
	}
}

// §17.2: after_run failure is ignored (no return value, no panic).
func TestAfterRun_FailureIgnored(t *testing.T) {
	root := t.TempDir()
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks:     wfconfig.HooksConfig{TimeoutMS: 5000, AfterRun: "exit 1"},
	})
	if _, err := m.Ensure(context.Background(), "issue-1"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	// Must not panic or block.
	m.AfterRun(context.Background(), "issue-1")
}

// §17.2: fatal hook timeout is an error.
func TestBeforeRun_Timeout(t *testing.T) {
	root := t.TempDir()
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks:     wfconfig.HooksConfig{TimeoutMS: 50, BeforeRun: "sleep 5"},
	})
	if _, err := m.Ensure(context.Background(), "issue-1"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	err := m.BeforeRun(context.Background(), "issue-1")
	if !errors.Is(err, ErrHookFailed) {
		t.Errorf("BeforeRun timeout err = %v, want ErrHookFailed", err)
	}
}

// §17.2: ignore hook timeout is logged, not returned.
func TestAfterRun_TimeoutIgnored(t *testing.T) {
	root := t.TempDir()
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: root},
		Hooks:     wfconfig.HooksConfig{TimeoutMS: 50, AfterRun: "sleep 5"},
	})
	if _, err := m.Ensure(context.Background(), "issue-1"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	m.AfterRun(context.Background(), "issue-1")
}

// §9.4: empty hook script is a no-op.
func TestBeforeRun_EmptyScript_Noop(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Ensure(context.Background(), "issue-1"); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if err := m.BeforeRun(context.Background(), "issue-1"); err != nil {
		t.Errorf("BeforeRun with empty script: %v", err)
	}
}
