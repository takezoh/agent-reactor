//go:build linux

package procgroup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestCommandKillsGrandchild verifies the core invariant: cancelling the context
// reaps not just the immediate child but the whole process group, so a
// grandchild that would otherwise be reparented to init is also killed.
func TestCommandKillsGrandchild(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	ctx, cancel := context.WithCancel(context.Background())

	// bash backgrounds a long sleep (the grandchild), records its pid, then waits.
	// Killing only bash would orphan the sleep; killing the group reaps it.
	cmd := Command(Spec{
		Ctx:  ctx,
		Bin:  "bash",
		Args: []string{"-c", "sleep 60 & echo $! > " + pidFile + "; wait"},
	})
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start: %v", err)
	}

	grandPID := waitForPID(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(grandPID, syscall.SIGKILL) }) // safety net

	if syscall.Kill(grandPID, 0) != nil {
		t.Fatalf("grandchild %d not alive before cancel", grandPID)
	}

	cancel()
	_ = cmd.Wait()

	if !eventually(3*time.Second, func() bool {
		return syscall.Kill(grandPID, 0) == syscall.ESRCH
	}) {
		t.Fatalf("grandchild %d survived context cancellation (orphan leak)", grandPID)
	}
}

func waitForPID(t *testing.T, pidFile string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			if s := strings.TrimSpace(string(data)); s != "" {
				pid, perr := strconv.Atoi(s)
				if perr == nil {
					return pid
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("grandchild pid not written to %s", pidFile)
	return 0
}

func eventually(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}
