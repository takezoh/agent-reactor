package fake

// PTY spawn helper for the FakeCLI. Tests use SpawnCLI to launch the test
// binary in FakeCLI mode inside a pty, then write prompts via the ptmx and
// scan `[EVENT]` / `[READY]` / `[REQUEST]` lines out the other side. This
// exercises the full wire path (stdin → codexclient.StartTurn →
// notification broadcast → codexclient.Notify → pty) without depending on
// the real codex TUI.

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// CLIHandle wraps a pty-attached FakeCLI subprocess. Cleanup is registered
// with t.Cleanup on construction.
type CLIHandle struct {
	Cmd    *exec.Cmd
	Ptmx   *os.File
	Events <-chan string // one line per received [EVENT] / [READY] / [REQUEST]
}

// SpawnCLI launches the current test binary (os.Args[0]) in FakeCLI mode
// under a pty. args are appended after the `fake-cli` dispatcher token, e.g.
// []string{"--remote", "unix://" + sock, "--cd", "/work"}.
func SpawnCLI(t *testing.T, args ...string) *CLIHandle {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"fake-cli"}, args...)...)
	cmd.Env = append(os.Environ(), "TERM=dumb")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}

	// Consume stdout in the background and republish complete lines that carry
	// one of the three well-known markers. Non-marker lines (any incidental
	// stderr routed through the pty) are dropped so tests can scan for
	// specific events without noise interference.
	events := make(chan string, 128)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(ptmx)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "[EVENT]") || strings.HasPrefix(line, "[READY]") || strings.HasPrefix(line, "[REQUEST]") {
				events <- line
			}
		}
		// pty EOF (subprocess exit) or a read error terminates the scanner;
		// either is expected on normal shutdown, so scanner.Err() is not
		// surfaced — Close() reaps the process and reports its exit.
		_ = scanner.Err()
	}()

	h := &CLIHandle{Cmd: cmd, Ptmx: ptmx, Events: events}
	t.Cleanup(h.Close)
	return h
}

// SendPrompt writes a newline-terminated prompt to the pty. Real codex TUI
// receives keystrokes; the FakeCLI reads line-by-line from stdin so a single
// Write with "\n" is a submitted prompt.
func (h *CLIHandle) SendPrompt(t *testing.T, prompt string) {
	t.Helper()
	if _, err := io.WriteString(h.Ptmx, prompt+"\n"); err != nil {
		t.Fatalf("pty write: %v", err)
	}
}

// WaitFor blocks until the observed event stream contains a line matching
// pred, or the timeout elapses.
func (h *CLIHandle) WaitFor(t *testing.T, timeout time.Duration, pred func(line string) bool, msg string) string {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case line, ok := <-h.Events:
			if !ok {
				t.Fatalf("CLI event stream closed before %s", msg)
			}
			if pred(line) {
				return line
			}
		case <-deadline:
			t.Fatalf("timeout (%s) waiting for %s", timeout, msg)
			return ""
		}
	}
}

// Ready blocks until the [READY] line is emitted and returns the threadId
// field. Assertion failure on timeout.
func (h *CLIHandle) Ready(t *testing.T, timeout time.Duration) string {
	t.Helper()
	line := h.WaitFor(t, timeout, func(l string) bool { return strings.HasPrefix(l, "[READY]") }, "[READY]")
	// Parse [READY] threadId=X
	_, rest, ok := strings.Cut(line, "threadId=")
	if !ok {
		t.Fatalf("READY line missing threadId: %q", line)
	}
	return strings.TrimSpace(rest)
}

// Close signals EOF to the CLI (blank line ends the read loop) and reaps the
// process. Safe to call more than once.
func (h *CLIHandle) Close() {
	if h.Ptmx == nil {
		return
	}
	// Empty line ends the FakeCLI read loop.
	_, _ = io.WriteString(h.Ptmx, "\n")
	// Give it a moment; then close pty and kill if still around.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	waited := make(chan error, 1)
	go func() { waited <- h.Cmd.Wait() }()
	select {
	case <-waited:
	case <-ctx.Done():
		_ = h.Cmd.Process.Kill()
		<-waited
	}
	_ = h.Ptmx.Close()
	h.Ptmx = nil
}
