//go:build linux

package procgroup

import (
	"os/exec"
	"syscall"
)

// applyProcGroup places the command in its own process group and, on context
// cancellation, SIGKILLs the entire group (negative PID) so descendants are
// reaped with the parent rather than orphaned to init.
func applyProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// The child is the process-group leader (pgid == pid because of
		// Setpgid), so -pid addresses the whole group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
