package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/config"
)

func TestResolveStartOptions_SharedIsolation(t *testing.T) {
	// Build a temp project_root with two sub-projects.
	root := t.TempDir()
	projA := filepath.Join(root, "project-a")
	projB := filepath.Join(root, "project-b")
	if err := os.MkdirAll(projA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projB, 0o755); err != nil {
		t.Fatal(err)
	}

	l := &DevcontainerLauncher{
		resolveProjectScope: func(p string) *config.SandboxConfig { return nil },
		resolveSandbox: func(p string) config.SandboxConfig {
			return config.SandboxConfig{
				Isolation:    "shared",
				Devcontainer: config.DevcontainerConfig{Path: "/tmp/shared-dc"},
			}
		},
		projectsConfig: config.ProjectsConfig{
			ProjectRoots: []string{root},
		},
	}

	opts := l.resolveStartOptions(projA)

	if !opts.SharedMode {
		t.Fatalf("expected SharedMode=true, got false")
	}

	wantProjects := []string{projA, projB}
	for _, want := range wantProjects {
		found := false
		for _, m := range opts.ExtraMounts {
			if strings.Contains(m, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected mount for %s in %v", want, opts.ExtraMounts)
		}
	}
}
