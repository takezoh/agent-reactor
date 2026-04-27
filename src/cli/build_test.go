package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	sandboxdc "github.com/takezoh/agent-roost/sandbox/devcontainer"
)

func TestRunBuild_noArgs(t *testing.T) {
	err := runBuild([]string{})
	if err == nil {
		t.Fatal("expected error with no args")
	}
}

func TestRunBuild_userAndProjectMutuallyExclusive(t *testing.T) {
	err := runBuild([]string{"--user", "/some/project"})
	if err == nil {
		t.Fatal("expected error when --user and project path both given")
	}
}

func TestRunProjectBuild_noDevcontainer(t *testing.T) {
	project := t.TempDir() // no .devcontainer
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	err := runProjectBuild([]string{project})
	if err == nil {
		t.Fatal("expected error when no .devcontainer found")
	}
	if !errors.Is(unwrapAll(err), sandboxdc.ErrNoProjectDevcontainer) {
		t.Errorf("expected ErrNoProjectDevcontainer in chain, got: %v", err)
	}
}

func TestRunUserBuild_noDevcontainer(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	err := runUserBuild()
	if err == nil {
		t.Fatal("expected error when no ~/.devcontainer found")
	}
	if !errors.Is(unwrapAll(err), sandboxdc.ErrNoUserDevcontainer) {
		t.Errorf("expected ErrNoUserDevcontainer in chain, got: %v", err)
	}
}

func TestRunProjectBuild_noDevcontainerCLI(t *testing.T) {
	project := t.TempDir()
	dcDir := filepath.Join(project, ".devcontainer")
	os.MkdirAll(dcDir, 0o755)
	os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(`{"image":"ubuntu"}`), 0o644)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("PATH", t.TempDir())

	err := runProjectBuild([]string{project})
	if err == nil {
		t.Fatal("expected error (devcontainer CLI not found)")
	}
}

// unwrapAll unwraps errors to find a target in the chain.
func unwrapAll(err error) error {
	for {
		u := errors.Unwrap(err)
		if u == nil {
			return err
		}
		err = u
	}
}
