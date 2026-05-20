package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// daemonLock is an exclusive advisory lock over the daemon's data
// directory, backed by flock(2) on a pid file. It prevents two
// coordinators from running against the same ~/.roost concurrently:
// two daemons share the sessions directory and fight over persistence —
// one rewrites session files the other has just deleted, so terminated
// sessions resurrect on every cold start.
//
// flock is released automatically when the holding process exits (the
// fd closes), so a crashed daemon never leaves a stale lock that blocks
// the next start. The pid is written into the file purely for humans.
type daemonLock struct {
	f *os.File
}

// acquireDaemonLock takes the exclusive lock at path. It returns an
// error naming the current holder's pid if another daemon already holds
// it. The caller must call release() on clean shutdown.
func acquireDaemonLock(path string) (*daemonLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("daemon lock: open %s: %w", path, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		holder := readPid(f)
		_ = f.Close()
		if holder != "" {
			return nil, fmt.Errorf("daemon lock: another roost daemon is already running (pid %s); refusing to start", holder)
		}
		return nil, fmt.Errorf("daemon lock: another roost daemon is already running; refusing to start")
	}
	if err := f.Truncate(0); err != nil {
		releaseLocked(f)
		return nil, fmt.Errorf("daemon lock: truncate %s: %w", path, err)
	}
	if _, err := f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0); err != nil {
		releaseLocked(f)
		return nil, fmt.Errorf("daemon lock: write pid %s: %w", path, err)
	}
	return &daemonLock{f: f}, nil
}

// release unlocks and closes the lock file. Safe to call on a nil lock.
func (l *daemonLock) release() {
	if l == nil || l.f == nil {
		return
	}
	releaseLocked(l.f)
	l.f = nil
}

func releaseLocked(f *os.File) {
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	_ = f.Close()
}

func readPid(f *os.File) string {
	buf := make([]byte, 32)
	n, _ := f.ReadAt(buf, 0)
	return strings.TrimSpace(string(buf[:n]))
}
