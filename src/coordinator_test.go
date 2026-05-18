package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/config"
	appruntime "github.com/takezoh/agent-roost/runtime"
)

func TestNewAgentLauncher_direct(t *testing.T) {
	for _, mode := range []string{"", "direct"} {
		resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: mode})
		l, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: mode}, resolver, config.ProjectsConfig{}, t.TempDir(), "")
		if err != nil {
			t.Errorf("mode=%q: unexpected error: %v", mode, err)
			continue
		}
		d, ok := l.(*appruntime.SandboxDispatcher)
		if !ok {
			t.Errorf("mode=%q: expected *SandboxDispatcher, got %T", mode, l)
			continue
		}
		if d.Devcontainer != nil {
			t.Errorf("mode=%q: expected Devcontainer=nil for direct mode, got %T", mode, d.Devcontainer)
		}
	}
}

func TestNewAgentLauncher_devcontainer_missing(t *testing.T) {
	t.Setenv("PATH", "")
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	_, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: "devcontainer"}, resolver, config.ProjectsConfig{}, t.TempDir(), "")
	if err == nil {
		t.Error("expected error when devcontainer CLI is not in PATH, got nil")
	}
}

func TestResolveShellDisplayFromValues(t *testing.T) {
	cases := []struct {
		tmuxDefault string
		envSHELL    string
		want        string
	}{
		{"/usr/bin/zsh", "/bin/bash", "zsh"},
		{"", "/bin/bash", "bash"},
		{"", "/usr/bin/zsh", "zsh"},
		{"", "", "shell"},
		{".", "", "shell"},
		{"", ".", "shell"},
		{".", ".", "shell"},
	}
	for _, c := range cases {
		got := resolveShellDisplayFromValues(c.tmuxDefault, c.envSHELL)
		if got != c.want {
			t.Errorf("resolveShellDisplayFromValues(%q, %q) = %q, want %q",
				c.tmuxDefault, c.envSHELL, got, c.want)
		}
	}
}

func TestRunCoordinatorRejectsInsideTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	err := runCoordinator()
	if err == nil {
		t.Fatal("expected error when $TMUX is set, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to start coordinator") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShouldKeepRuntimeAliveAfterAttach(t *testing.T) {
	errAttach := errors.New("attach failed")
	if !shouldKeepRuntimeAliveAfterAttach(errAttach, true) {
		t.Fatal("want keep-alive when attach failed and session exists")
	}
	if shouldKeepRuntimeAliveAfterAttach(nil, true) {
		t.Fatal("did not expect keep-alive on clean detach")
	}
	if shouldKeepRuntimeAliveAfterAttach(errAttach, false) {
		t.Fatal("did not expect keep-alive when session is gone")
	}
}
