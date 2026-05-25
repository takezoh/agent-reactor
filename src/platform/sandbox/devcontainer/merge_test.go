package devcontainer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// setupProjectDC creates a temp project dir with a .devcontainer/devcontainer.json.
func setupProjectDC(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	dcDir := filepath.Join(dir, ".devcontainer")
	if err := os.MkdirAll(dcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ── ProjectBaseDC ─────────────────────────────────────────────────────────────

func TestProjectBaseDC_found(t *testing.T) {
	project := setupProjectDC(t, `{"image":"ubuntu"}`)
	got, err := ProjectBaseDC(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(project, ".devcontainer", "devcontainer.json")
	if got != want {
		t.Errorf("basePath = %q, want %q", got, want)
	}
}

func TestProjectBaseDC_notFound(t *testing.T) {
	project := t.TempDir()
	_, err := ProjectBaseDC(project)
	if !errors.Is(err, ErrNoProjectDevcontainer) {
		t.Errorf("expected ErrNoProjectDevcontainer, got %v", err)
	}
}

func TestUserBaseDC_found(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dcDir := filepath.Join(home, ".devcontainer")
	if err := os.MkdirAll(dcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(`{"image":"ubuntu"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := UserBaseDC()
	if err != nil {
		t.Fatalf("UserBaseDC: %v", err)
	}
	want := filepath.Join(dcDir, "devcontainer.json")
	if got != want {
		t.Errorf("UserBaseDC = %q, want %q", got, want)
	}
}

func TestUserBaseDC_notFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := UserBaseDC()
	if !errors.Is(err, ErrNoUserDevcontainer) {
		t.Errorf("expected ErrNoUserDevcontainer, got %v", err)
	}
}

// ── FindDevcontainerPath with override ────────────────────────────────────────

func TestFindDevcontainerPath_override_found(t *testing.T) {
	override := setupProjectDC(t, `{"image":"ubuntu"}`)
	got, err := FindDevcontainerPath("/some/project", filepath.Join(override, ".devcontainer"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(override, ".devcontainer", "devcontainer.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFindDevcontainerPath_override_notFound(t *testing.T) {
	_, err := FindDevcontainerPath("/some/project", "/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent override path, got nil")
	}
}

func TestFindDevcontainerPath_no_override_falls_through(t *testing.T) {
	project := setupProjectDC(t, `{"image":"ubuntu"}`)
	got, err := FindDevcontainerPath(project, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(project, ".devcontainer", "devcontainer.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ── TranslateWorkDir ──────────────────────────────────────────────────────────

func TestTranslateWorkDir(t *testing.T) {
	const project = "/home/take/code/myapp"
	const remoteWS = "/workspaces/myapp"

	cases := []struct {
		name    string
		hostDir string
		want    string
	}{
		{"empty → remoteWS", "", remoteWS},
		{"project root → remoteWS", project, remoteWS},
		{"sub-dir", project + "/backend/api", remoteWS + "/backend/api"},
		{"worktree", project + "/.roost/worktree/feat", remoteWS + "/.roost/worktree/feat"},
		{"outside project → remoteWS", "/home/take/other", remoteWS},
		{"dotdot escape → remoteWS", project + "/../other", remoteWS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateWorkDir(tc.hostDir, project, remoteWS)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
