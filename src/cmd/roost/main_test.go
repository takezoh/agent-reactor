package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/client/config"
)

func TestRunMainRoostSuccess(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	runCoordinatorFn = func() error { return nil }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if got := stdout.String(); got != "roost: exited\n" {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunMainRoostErrorLogsAndPrints(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	wantErr := errors.New("boom")
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	runCoordinatorFn = func() error { return wantErr }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got != "roost: boom\n" {
		t.Fatalf("stderr = %q", got)
	}

	data, err := os.ReadFile(filepath.Join(dir, "roost.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "main failed") {
		t.Fatalf("log = %q, want main failed entry", string(data))
	}
}

func TestRunMainTUIDoesNotPrint(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	runMainTUIFn = func() error { return errors.New("tui failed") }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain([]string{"--tui", "main"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunMainUnknownCommandDoesNotRunCoordinator(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	dir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = dir
		return cfg, nil
	}
	coordinatorCalled := false
	runCoordinatorFn = func() error {
		coordinatorCalled = true
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runMain([]string{"totally-unknown-command"}, &stdout, &stderr)
	if coordinatorCalled {
		t.Fatal("runCoordinatorFn must not be called for unknown subcommands")
	}
	if code == 0 {
		t.Fatal("unknown command should exit non-zero")
	}
}

func TestRunMainUnknownCommandClassifiedAsCLI(t *testing.T) {
	if got := classifyCommand([]string{"bogus-command"}); got != commandKindCLI {
		t.Fatalf("classifyCommand(['bogus-command']) = %v, want commandKindCLI", got)
	}
}

func stubMainDeps(t *testing.T) func() {
	t.Helper()
	t.Setenv("ROOST_DATA_DIR", "")
	prevLoadBootstrapConfig := loadBootstrapConfig
	prevInitLogger := initLoggerWithDataDir
	prevCloseLogger := closeLogger
	prevRedirectStderr := redirectStderr
	prevRunCoordinator := runCoordinatorFn
	prevRunMainTUI := runMainTUIFn
	prevRunSessionList := runSessionListFn
	prevRunLogViewer := runLogViewerFn
	prevRunPalette := runPaletteFn

	return func() {
		loadBootstrapConfig = prevLoadBootstrapConfig
		initLoggerWithDataDir = prevInitLogger
		closeLogger = prevCloseLogger
		redirectStderr = prevRedirectStderr
		runCoordinatorFn = prevRunCoordinator
		runMainTUIFn = prevRunMainTUI
		runSessionListFn = prevRunSessionList
		runLogViewerFn = prevRunLogViewer
		runPaletteFn = prevRunPalette
	}
}
