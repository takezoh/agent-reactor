package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/platform/logger"
)

// SPEC §17.1 — explicit --workflow path is used when provided; the cwd default
// is ./WORKFLOW.md when no explicit path argument is given.
func TestSPEC_17_1_WorkflowFilePathPrecedence(t *testing.T) {
	t.Run("explicit path used", func(t *testing.T) {
		isolateHome(t)
		explicit := filepath.Join(t.TempDir(), "custom.md")
		var stderr bytes.Buffer
		code := run(context.Background(), []string{"--workflow", explicit}, &stderr)
		if code == 0 {
			t.Error("want non-zero exit when explicit path does not exist")
		}
		// The error must reference the explicit path, not the cwd default.
		if !bytes.Contains(stderr.Bytes(), []byte("custom.md")) {
			t.Errorf("want stderr to reference the explicit path; stderr: %s", stderr.String())
		}
	})

	t.Run("cwd default is WORKFLOW.md", func(t *testing.T) {
		isolateHome(t)
		dir := t.TempDir()
		t.Chdir(dir)
		// No WORKFLOW.md in dir → run() fails because ./WORKFLOW.md is absent.
		var stderr bytes.Buffer
		code := run(context.Background(), []string{}, &stderr)
		if code == 0 {
			t.Error("want non-zero exit when ./WORKFLOW.md is absent (cwd default)")
		}
		// With a valid ./WORKFLOW.md, run() should start successfully.
		wfPath := filepath.Join(dir, "WORKFLOW.md")
		if err := os.WriteFile(wfPath, []byte(validWorkflow), 0o644); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var stderr2 bytes.Buffer
		code2 := run(ctx, []string{}, &stderr2)
		if code2 != 0 {
			t.Errorf("want 0 when ./WORKFLOW.md exists and ctx is already cancelled, got %d; stderr: %s", code2, &stderr2)
		}
	})
}

// SPEC §17.7 — the resolved tracker.api_key value must never appear in log output
// or stderr (§15.3: secret non-log invariant).
func TestSPEC_17_7_SecretNeverLogged(t *testing.T) {
	isolateHome(t)

	const sentinel = "roost-secret-sentinel-XYZABC"
	t.Setenv("CONF_TEST_SENTINEL_KEY", sentinel)

	wf := writeWorkflow(t, `---
tracker:
  kind: linear
  api_key: $CONF_TEST_SENTINEL_KEY
  project_slugs:
    - test-proj
codex:
  command: codex app-server
---
`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stderr bytes.Buffer
	run(ctx, []string{"--workflow", wf}, &stderr)

	if bytes.Contains(stderr.Bytes(), []byte(sentinel)) {
		t.Errorf("secret appeared in stderr: %s", stderr.String())
	}

	logPath := logger.LogFilePath()
	if logPath != "" {
		data, err := os.ReadFile(logPath)
		if err == nil && bytes.Contains(data, []byte(sentinel)) {
			t.Errorf("secret appeared in log file %s", logPath)
		}
	}
}

// SPEC §17.7 — CLI exits with code 0 when the application starts and shuts down normally.
func TestSPEC_17_7_GracefulShutdownExitsZero(t *testing.T) {
	isolateHome(t)
	wf := writeWorkflow(t, validWorkflow)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--workflow", wf}, &stderr)
	if code != 0 {
		t.Errorf("want exit 0 on graceful shutdown, got %d; stderr: %s", code, &stderr)
	}
}
