package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

const validWorkflow = `---
tracker:
  kind: linear
  api_key: lin_api_test
  project_slug: test-proj
codex:
  command: codex app-server
---
`

func writeWorkflow(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// isolateHome points HOME at a temp dir so run() neither reads the developer's
// real ~/.roost/settings.toml (which may select devcontainer mode and shell out
// to docker) nor writes the real ~/.roost/roost.log. Both logger.Init and the
// sandbox config loader resolve paths via os.UserHomeDir().
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestRunMissingWorkflow(t *testing.T) {
	isolateHome(t)
	ctx := context.Background()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--workflow", filepath.Join(t.TempDir(), "no-such.md")}, &stderr)
	if code == 0 {
		t.Error("want non-zero exit for missing workflow")
	}
	if stderr.Len() == 0 {
		t.Error("want operator-visible error on stderr")
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	isolateHome(t)
	wf := writeWorkflow(t, validWorkflow)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--workflow", wf}, &stderr)
	if code != 0 {
		t.Errorf("want 0 for graceful shutdown, got %d; stderr: %s", code, &stderr)
	}
}

func TestRunInvalidFlag(t *testing.T) {
	isolateHome(t)
	ctx := context.Background()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--no-such-flag"}, &stderr)
	if code == 0 {
		t.Error("want non-zero exit for unknown flag")
	}
}

func TestRunPreflightFailure(t *testing.T) {
	isolateHome(t)
	// Missing project_slug triggers preflight error after config resolve.
	content := `---
tracker:
  kind: linear
  api_key: lin_api_test
codex:
  command: codex app-server
---
`
	wf := writeWorkflow(t, content)
	ctx := context.Background()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--workflow", wf}, &stderr)
	if code == 0 {
		t.Error("want non-zero exit for preflight failure")
	}
	if stderr.Len() == 0 {
		t.Error("want operator-visible error on stderr")
	}
}

func TestRunConfigResolveFailure(t *testing.T) {
	isolateHome(t)
	// polling.interval_ms < 0 fails wfconfig.validate before preflight.
	content := `---
tracker:
  kind: linear
  api_key: lin_api_test
  project_slug: test-proj
polling:
  interval_ms: -1
codex:
  command: codex app-server
---
`
	wf := writeWorkflow(t, content)
	ctx := context.Background()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--workflow", wf}, &stderr)
	if code == 0 {
		t.Error("want non-zero exit for config validation failure")
	}
	if stderr.Len() == 0 {
		t.Error("want operator-visible error on stderr")
	}
}
