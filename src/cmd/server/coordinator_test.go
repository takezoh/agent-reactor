package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/config"
)

func TestNewAgentLauncher_direct(t *testing.T) {
	for _, mode := range []string{"", "direct"} {
		resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: mode})
		l, _, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: mode}, resolver, config.ProjectsConfig{}, t.TempDir(), "", "")
		if err != nil {
			t.Errorf("mode=%q: unexpected error: %v", mode, err)
			continue
		}
		if l.IsContainer("/any/project") {
			t.Errorf("mode=%q: expected IsContainer=false for direct mode", mode)
		}
	}
}

func TestNewAgentLauncher_devcontainer_missing(t *testing.T) {
	t.Setenv("PATH", "")
	resolver := config.NewSandboxResolver(config.SandboxConfig{Mode: "devcontainer"})
	_, _, err := newAgentLauncher(context.Background(), config.SandboxConfig{Mode: "devcontainer"}, resolver, config.ProjectsConfig{}, t.TempDir(), "", "")
	if err == nil {
		t.Error("expected error when devcontainer CLI is not in PATH, got nil")
	}
}

func TestShellDisplayName(t *testing.T) {
	cases := []struct {
		shell string
		want  string
	}{
		{"/usr/bin/zsh", "zsh"},
		{"/bin/bash", "bash"},
		{"zsh", "zsh"},
		{"", "shell"},
		{".", "shell"},
		{"/", "shell"},
	}
	for _, c := range cases {
		if got := shellDisplayName(c.shell); got != c.want {
			t.Errorf("shellDisplayName(%q) = %q, want %q", c.shell, got, c.want)
		}
	}
}

// SIGHUP must not kill the daemon. Regression test for the failure mode
// where the daemon process vanished after `attaching to the backend session`,
// leaving every TUI pane dead and the user staring at a broken session.
//
// Historically a backend `attach-session` ran as a child of the daemon; once it
// took the TTY the parent terminal could deliver a spurious SIGHUP (pane
// closed in WSL/Windows Terminal, controlling-tty races, etc.). The default
// action for SIGHUP is process termination, which would kill the daemon
// while the backend session itself stays up — exactly the "all 4 TUI panes
// EOFed simultaneously, daemon gone, no shutdown log" pattern.
//
// installSignalHandlers must log the signal and ignore it, leaving the
// context live so the daemon keeps serving the backend session.
func TestInstallSignalHandlers_SIGHUP_IgnoredKeepsContextAlive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := installSignalHandlers(nil, cancel)
	defer stop()

	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}
	// Allow the goroutine to consume the signal.
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case <-deadline:
			if err := ctx.Err(); err != nil {
				t.Fatalf("SIGHUP cancelled the context: %v", err)
			}
			return
		default:
			if ctx.Err() != nil {
				t.Fatalf("SIGHUP cancelled the context: %v", ctx.Err())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// SIGTERM must cancel the context for graceful shutdown.
func TestInstallSignalHandlers_SIGTERM_CancelsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stop := installSignalHandlers(nil, cancel)
	defer stop()

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("SIGTERM did not cancel context within 1s")
	}
}

// stop() must unblock even when no signals arrived. Guards against a
// goroutine leak in long-lived tests / repeated start-stop cycles.
func TestInstallSignalHandlers_StopUnblocksWithNoSignals(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := installSignalHandlers(nil, cancel)
	doneCh := make(chan struct{})
	go func() {
		stop()
		close(doneCh)
	}()
	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("stop() did not return within 1s with no signals delivered")
	}
}

// A panic inside the runtime goroutine must not kill the daemon process.
// It must (a) land on errCh as an error, (b) cancel the supervisor
// context so the rest of the daemon shuts down cleanly. Before this
// guard a state.Reduce panic took the entire daemon down without any
// log line beyond "proto: read loop ended" from the orphaned TUI
// subprocesses — the symptom users kept reporting as "TUI suddenly
// broken, daemon vanished".
func TestSuperviseRun_PanicSurfacesErrorAndCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		superviseRun(cancel, errCh, func() error {
			panic("synthetic reducer panic")
		})
		close(done)
	}()
	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "synthetic reducer panic") {
			t.Errorf("want panic surfaced via errCh, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not push panic to errCh within 1s")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not cancel the parent context after panic")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not return after panic")
	}
}

// Errors returned by fn (not panics) surface on errCh but do not cancel
// the supervisor context — that path is reserved for unrecoverable
// goroutine panics.
func TestSuperviseRun_ErrorPropagatesWithoutCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	want := errors.New("ordinary runtime error")
	done := make(chan struct{})
	go func() {
		superviseRun(cancel, errCh, func() error { return want })
		close(done)
	}()
	select {
	case got := <-errCh:
		if !errors.Is(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("error did not arrive on errCh within 1s")
	}
	if ctx.Err() != nil {
		t.Errorf("ordinary error must not cancel ctx, got %v", ctx.Err())
	}
	<-done
}

// context.Canceled is the cooperative shutdown signal and must never be
// reported as an error. superviseRun closes errCh on return (so callers can
// distinguish "no error reported" from "channel still open"), so the receive
// here uses comma-ok: ok=false means the goroutine returned without sending,
// which is the swallow path we want to assert.
func TestSuperviseRun_ContextCanceledSwallowed(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		superviseRun(cancel, errCh, func() error { return context.Canceled })
		close(done)
	}()
	select {
	case got, ok := <-errCh:
		if ok {
			t.Fatalf("context.Canceled must not be reported, got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("superviseRun did not close errCh within 1s")
	}
	<-done
}
