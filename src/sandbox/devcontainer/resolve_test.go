package devcontainer

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestResolveImage_noImages(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found")
	}

	// Use a unique temp path so no roost images can exist for it.
	projectPath := t.TempDir()
	_, _, err := ResolveImage(context.Background(), projectPath)
	if err == nil {
		t.Fatal("expected error when no images exist")
	}
	if !strings.Contains(err.Error(), "roost build") {
		t.Errorf("error should mention 'roost build', got: %v", err)
	}
}

func TestProjectScopeImage_format(t *testing.T) {
	img := ProjectScopeImage("abc123")
	if img != "roost-proj-abc123:latest" {
		t.Errorf("got %q, want %q", img, "roost-proj-abc123:latest")
	}
}

func TestUserScopeImage_format(t *testing.T) {
	img := UserScopeImage()
	if img != "roost-user:latest" {
		t.Errorf("got %q, want %q", img, "roost-user:latest")
	}
}
