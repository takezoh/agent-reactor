package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunMissingWorkflow(t *testing.T) {
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
	wf := filepath.Join(t.TempDir(), "WORKFLOW.md")
	if err := os.WriteFile(wf, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--workflow", wf}, &stderr)
	if code != 0 {
		t.Errorf("want 0 for graceful shutdown, got %d; stderr: %s", code, &stderr)
	}
}

func TestRunInvalidFlag(t *testing.T) {
	ctx := context.Background()
	var stderr bytes.Buffer
	code := run(ctx, []string{"--no-such-flag"}, &stderr)
	if code == 0 {
		t.Error("want non-zero exit for unknown flag")
	}
}
