package devcontainer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// setupUserDC creates a fake home with ~/.devcontainer/devcontainer.json.
func setupUserDC(t *testing.T, content string, extraFiles map[string]string) string {
	t.Helper()
	fakeHome := t.TempDir()
	dcDir := filepath.Join(fakeHome, ".devcontainer")
	os.MkdirAll(dcDir, 0o755)
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range extraFiles {
		if err := os.WriteFile(filepath.Join(dcDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return fakeHome
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

// ── UserBaseDC ────────────────────────────────────────────────────────────────

func TestUserBaseDC_found(t *testing.T) {
	fakeHome := setupUserDC(t, `{"image":"default"}`, nil)
	t.Setenv("HOME", fakeHome)

	got, err := UserBaseDC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, fakeHome) {
		t.Errorf("expected path under %s, got %q", fakeHome, got)
	}
}

func TestUserBaseDC_notFound(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, err := UserBaseDC()
	if !errors.Is(err, ErrNoUserDevcontainer) {
		t.Errorf("expected ErrNoUserDevcontainer, got %v", err)
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
