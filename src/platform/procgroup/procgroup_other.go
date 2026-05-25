//go:build !linux

package procgroup

import "os/exec"

// applyProcGroup is a no-op on non-Linux platforms. exec.CommandContext still
// SIGKILLs the immediate child on cancellation, but descendant reaping and the
// crash-path reaper (see reaper_other.go) are Linux-only.
func applyProcGroup(_ *exec.Cmd) {}
