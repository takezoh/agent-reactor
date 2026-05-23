package workspace

import (
	"errors"
	"os"
	"testing"
)

// SPEC §17.2 — workspace path sanitization replaces disallowed chars per §9.5 Inv3.
// The sanitized key uses only [A-Za-z0-9._-]; other chars become '_'.
func TestSPEC_17_2_WorkspaceKeySanitized(t *testing.T) {
	m := newTestManager(t)

	// Identifier with spaces and slashes: all become '_'.
	p1, err := m.Path("my issue/123 foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p2, err := m.Path("my_issue_123_foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p1 != p2 {
		t.Errorf("sanitized path mismatch:\n  identifier with special chars: %s\n  pre-sanitized: %s", p1, p2)
	}
}

// SPEC §17.2 — cwd must equal the workspace path; out-of-root paths are rejected.
func TestSPEC_17_2_CwdEqualsWorkspaceRoot(t *testing.T) {
	m := newTestManager(t)

	p, err := m.Path("ISSUE-1")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	// VerifyCWD resolves symlinks so the path must exist on disk.
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := m.VerifyCWD("ISSUE-1", p); err != nil {
		t.Errorf("VerifyCWD with matching path want nil, got %v", err)
	}
	if err := m.VerifyCWD("ISSUE-1", "/tmp/other"); !errors.Is(err, ErrCWDMismatch) {
		t.Errorf("VerifyCWD with wrong path want ErrCWDMismatch, got %v", err)
	}
}

// SPEC §17.7 — root prefix containment invariant: identifiers that resolve outside
// the workspace root are rejected before agent launch (§9.5 Inv2).
// sanitizeKey preserves '.' so ".." and "." are the canonical path-traversal probes.
func TestSPEC_17_7_RootPrefixCheck(t *testing.T) {
	m := newTestManager(t)

	// ".." → filepath.Join(root, "..") = parent of root → escapes root.
	_, err := m.Path("..")
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Errorf("Path(\"..\") want ErrPathEscapesRoot, got %v", err)
	}

	// "" → filepath.Join(root, "") = root itself → rel is "." → rejected.
	_, err = m.Path("")
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Errorf("Path(\"\") want ErrPathEscapesRoot, got %v", err)
	}
}
