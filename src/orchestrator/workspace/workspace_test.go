package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: t.TempDir()},
		Hooks:     wfconfig.HooksConfig{TimeoutMS: 5000},
	})
}

// Root returns the cleaned workspace root (the per-project container key).
func TestRoot_ReturnsCleanedRoot(t *testing.T) {
	m := New(wfconfig.Config{
		Workspace: wfconfig.WorkspaceConfig{Root: "/foo/bar/"},
	})
	if got := m.Root(); got != "/foo/bar" {
		t.Errorf("Root() = %q, want %q", got, "/foo/bar")
	}
}

// Root must be the parent of every Path so pathmap can translate the per-issue
// StartDir to a subdir of the project mount.
func TestRoot_IsParentOfPath(t *testing.T) {
	m := newTestManager(t)
	p, err := m.Path("ISSUE-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir := filepath.Dir(p); dir != m.Root() {
		t.Errorf("filepath.Dir(Path) = %q, want Root() = %q", dir, m.Root())
	}
}

// §17.2: identifier→path is deterministic.
func TestPath_Deterministic(t *testing.T) {
	m := newTestManager(t)
	p1, err := m.Path("my-issue-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p2, _ := m.Path("my-issue-42")
	if p1 != p2 {
		t.Errorf("Path not deterministic: %q != %q", p1, p2)
	}
}

func TestPath_Sanitize_ReplacesInvalidChars(t *testing.T) {
	m := newTestManager(t)
	p, err := m.Path("issue/2024 #foo@bar!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	base := filepath.Base(p)
	if base != "issue_2024__foo_bar_" {
		t.Errorf("sanitized base = %q, want %q", base, "issue_2024__foo_bar_")
	}
}

func TestPath_Sanitize_PreservesAllowedChars(t *testing.T) {
	m := newTestManager(t)
	p, err := m.Path("Issue-42.Fix_v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(p) != "Issue-42.Fix_v1" {
		t.Errorf("sanitized base = %q, want unchanged %q", filepath.Base(p), "Issue-42.Fix_v1")
	}
}

// §17.2 / §9.5 Inv2+Inv3: root escape and sanitize-bypass must be rejected.
func TestPath_EscapeRoot_DotDot(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Path("..")
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Errorf("Path(%q) err = %v, want ErrPathEscapesRoot", "..", err)
	}
}

func TestPath_EscapeRoot_EmptyIdentifier(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Path("")
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Errorf("Path(%q) err = %v, want ErrPathEscapesRoot", "", err)
	}
}

func TestPath_EscapeRoot_SingleDot(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Path(".")
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Errorf("Path(%q) err = %v, want ErrPathEscapesRoot", ".", err)
	}
}

func TestPath_ValidIdentifier_WithinRoot(t *testing.T) {
	m := newTestManager(t)
	p, err := m.Path("valid-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("Path = %q, want absolute path", p)
	}
}

// §9.5 Inv1: VerifyCWD enforces cwd == workspace_path before agent launch.
func TestVerifyCWD_Match(t *testing.T) {
	m := newTestManager(t)
	expected, _ := m.Path("issue-1")
	if err := os.MkdirAll(expected, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.VerifyCWD("issue-1", expected); err != nil {
		t.Errorf("VerifyCWD with matching path: %v", err)
	}
}

func TestVerifyCWD_Mismatch_OtherIssue(t *testing.T) {
	m := newTestManager(t)
	p1, _ := m.Path("issue-1")
	if err := os.MkdirAll(p1, 0o755); err != nil {
		t.Fatal(err)
	}
	err := m.VerifyCWD("issue-2", p1)
	if !errors.Is(err, ErrCWDMismatch) {
		t.Errorf("VerifyCWD mismatch err = %v, want ErrCWDMismatch", err)
	}
}

func TestVerifyCWD_Mismatch_OutsideRoot(t *testing.T) {
	m := newTestManager(t)
	err := m.VerifyCWD("issue-1", os.TempDir())
	if !errors.Is(err, ErrCWDMismatch) {
		t.Errorf("VerifyCWD outside-root err = %v, want ErrCWDMismatch", err)
	}
}

// §9.5/§15.2: symlink inside root that points outside must be rejected.
func TestVerifyCWD_SymlinkEscape_OutsideRoot(t *testing.T) {
	m := newTestManager(t)
	wsPath, err := m.Path("issue-sym")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	externalDir := t.TempDir()
	if err := os.Symlink(externalDir, wsPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	err = m.VerifyCWD("issue-sym", wsPath)
	if !errors.Is(err, ErrSymlinkEscapesRoot) {
		t.Errorf("VerifyCWD symlink-escape err = %v, want ErrSymlinkEscapesRoot", err)
	}
}

// §9.5/§15.2: symlink inside root that points to another dir inside root must succeed.
func TestVerifyCWD_SymlinkWithinRoot(t *testing.T) {
	m := newTestManager(t)
	target := filepath.Join(m.Root(), "real-issue-dir")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	wsPath, err := m.Path("issue-link")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.Symlink(target, wsPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if err := m.VerifyCWD("issue-link", wsPath); err != nil {
		t.Errorf("VerifyCWD symlink-within-root: unexpected err = %v", err)
	}
}

// §9.5/§15.2: symlink that resolves to the workspace root itself must be rejected.
// Path() rejects rel=="." for lexical paths; the post-symlink check must be equally strict.
func TestVerifyCWD_SymlinkToRoot_IsRejected(t *testing.T) {
	m := newTestManager(t)
	wsPath, err := m.Path("issue-rootlink")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.Symlink(m.Root(), wsPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if err := m.VerifyCWD("issue-rootlink", wsPath); !errors.Is(err, ErrSymlinkEscapesRoot) {
		t.Errorf("VerifyCWD symlink-to-root err = %v, want ErrSymlinkEscapesRoot", err)
	}
}

// §9.5/§15.2: VerifyCWD returns ErrCWDMismatch when cwd does not exist on disk
// (EvalSymlinks cannot resolve a non-existent path).
func TestVerifyCWD_NonExistentPath_ReturnsError(t *testing.T) {
	m := newTestManager(t)
	wsPath, err := m.Path("issue-ghost")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	err = m.VerifyCWD("issue-ghost", wsPath)
	if !errors.Is(err, ErrCWDMismatch) {
		t.Errorf("VerifyCWD non-existent err = %v, want ErrCWDMismatch", err)
	}
}
