package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestAcquireDaemonLock_RefusesSecond reproduces the multi-daemon bug:
// two coordinators against the same data dir must not both start. The
// first acquirer holds an exclusive lock; the second must fail rather
// than proceed (which is what let two daemons fight over ~/.roost/sessions).
func TestAcquireDaemonLock_RefusesSecond(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roost.pid")

	l1, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer l1.release()

	l2, err := acquireDaemonLock(path)
	if err == nil {
		l2.release()
		t.Fatal("second acquire succeeded — multi-daemon not prevented")
	}
}

// TestAcquireDaemonLock_ReleaseAllowsReacquire ensures a clean shutdown
// frees the lock so the next daemon can start.
func TestAcquireDaemonLock_ReleaseAllowsReacquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roost.pid")

	l1, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	l1.release()

	l2, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("re-acquire after release failed: %v", err)
	}
	l2.release()
}

// TestAcquireDaemonLock_WritesPid verifies the lock file records the
// owner's pid so an operator can identify the running daemon.
func TestAcquireDaemonLock_WritesPid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roost.pid")

	l, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	defer l.release()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if got != strconv.Itoa(os.Getpid()) {
		t.Errorf("pid file = %q, want %d", got, os.Getpid())
	}
}

// TestAcquireDaemonLock_ErrorNamesExistingPid checks the refusal error
// surfaces the holder's pid, so the user knows which process to kill.
func TestAcquireDaemonLock_ErrorNamesExistingPid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roost.pid")

	l1, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer l1.release()

	_, err = acquireDaemonLock(path)
	if err == nil {
		t.Fatal("second acquire unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(os.Getpid())) {
		t.Errorf("error %q should name holder pid %d", err.Error(), os.Getpid())
	}
}
