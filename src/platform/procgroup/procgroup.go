// Package procgroup builds external-command *exec.Cmd values whose entire
// process group is terminated when the owning context is cancelled.
//
// Go's exec.CommandContext only sends SIGKILL to the immediate child on
// cancellation, so any grandchildren the child spawned (an ssh-agent, a
// language server launched by an MCP shim, codex tool subprocesses, …) are
// reparented to init and survive. By placing each command in its own process
// group (Setpgid) and killing the whole group on cancellation, descendants are
// reaped together with the parent.
//
// The Linux implementation mirrors the pattern established in
// cmd/claude-app-server (Setpgid + Cancel=Kill(-pgid) + WaitDelay). On non-Linux
// platforms the helper degrades to plain exec.CommandContext semantics.
package procgroup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"strconv"
	"time"
)

// DefaultWaitDelay bounds how long Cmd.Wait blocks after context cancellation
// before the os/exec machinery force-closes the process's I/O. It gives a
// killed process group a brief window to exit on its own SIGKILL.
const DefaultWaitDelay = 5 * time.Second

// Spec describes an external command to launch under process-group control.
// The caller still owns Start/Wait and any stdio pipe wiring on the returned
// *exec.Cmd; procgroup only configures cancellation and grouping.
type Spec struct {
	Ctx       context.Context
	Bin       string
	Args      []string
	Dir       string   // working directory; empty = inherit
	Env       []string // full environment; nil = inherit os.Environ()
	WaitDelay time.Duration
}

// Command builds an *exec.Cmd bound to spec.Ctx. On Linux the command runs in
// its own process group and the whole group is SIGKILL'd when the context is
// cancelled; on other platforms it falls back to exec.CommandContext defaults.
func Command(spec Spec) *exec.Cmd {
	ctx := spec.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, spec.Bin, spec.Args...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	delay := spec.WaitDelay
	if delay <= 0 {
		delay = DefaultWaitDelay
	}
	cmd.WaitDelay = delay
	applyProcGroup(cmd)
	return cmd
}

// NewBootNonce returns a random per-process token used to tag this boot's
// process-group markers, so a later boot's PruneOrphans only reaps groups left
// by *earlier* boots. Falls back to a timestamp if the RNG is unavailable.
func NewBootNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

// Tracker records live process groups under Dir tagged with Nonce so a future
// boot can reap them after a crash. A nil Tracker (or one with an empty Dir) is
// a no-op, so callers without crash-reaping configured need no branching. On
// non-Linux platforms the underlying marker operations are no-ops.
type Tracker struct {
	Dir   string
	Nonce string
}

// Track records pgid as a live group of the current boot.
func (t *Tracker) Track(pgid int) {
	if t == nil || t.Dir == "" {
		return
	}
	_ = WriteMarker(t.Dir, t.Nonce, pgid)
}

// Untrack drops pgid's marker after the group has been reaped normally.
func (t *Tracker) Untrack(pgid int) {
	if t == nil || t.Dir == "" {
		return
	}
	_ = RemoveMarker(t.Dir, pgid)
}

// Prune reaps process groups left by earlier boots. Call once at startup.
func (t *Tracker) Prune() {
	if t == nil || t.Dir == "" {
		return
	}
	_ = PruneOrphans(t.Dir, t.Nonce)
}
