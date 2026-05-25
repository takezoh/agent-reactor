//go:build linux

package procgroup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// startGroup launches a long-lived `sleep` in its own process group and returns
// its command and pgid (== leader pid). The process is killed on test cleanup.
//
// Note: the sleep is a direct child of the test process, so after a kill it
// becomes a zombie until Wait reaps it (unlike a real prior-boot orphan, which
// init reaps). Tests therefore assert the kill via wait-status, not via
// kill(-pgid, 0), which would still see a zombie as present.
func startGroup(t *testing.T) (*exec.Cmd, int) {
	t.Helper()
	cmd := Command(Spec{Ctx: context.Background(), Bin: "sleep", Args: []string{"120"}})
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pgid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		_ = cmd.Wait()
	})
	return cmd, pgid
}

func killedBySIGKILL(cmd *exec.Cmd) bool {
	if cmd.ProcessState == nil {
		return false
	}
	ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	return ok && ws.Signaled() && ws.Signal() == syscall.SIGKILL
}

func TestPruneOrphansKillsPriorBoot(t *testing.T) {
	dir := t.TempDir()
	cmd, pgid := startGroup(t)
	if err := WriteMarker(dir, "boot-old", pgid); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	if err := PruneOrphans(dir, "boot-new"); err != nil {
		t.Fatalf("PruneOrphans: %v", err)
	}

	// PruneOrphans must have SIGKILL'd the prior-boot group; Wait reaps the zombie.
	_ = cmd.Wait()
	if !killedBySIGKILL(cmd) {
		t.Fatalf("prior-boot group %d not killed by prune (state=%v)", pgid, cmd.ProcessState)
	}
	if _, err := os.Stat(filepath.Join(dir, strconv.Itoa(pgid)+markerExt)); !os.IsNotExist(err) {
		t.Errorf("marker for %d not removed after prune", pgid)
	}
}

func TestPruneOrphansSpareCurrentBoot(t *testing.T) {
	dir := t.TempDir()
	_, pgid := startGroup(t)
	if err := WriteMarker(dir, "boot-cur", pgid); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	if err := PruneOrphans(dir, "boot-cur"); err != nil {
		t.Fatalf("PruneOrphans: %v", err)
	}

	// Same-boot marker: the live group must be left running and the marker kept.
	time.Sleep(100 * time.Millisecond)
	if syscall.Kill(-pgid, 0) != nil {
		t.Errorf("current-boot group %d was killed by prune", pgid)
	}
	if _, err := os.Stat(filepath.Join(dir, strconv.Itoa(pgid)+markerExt)); err != nil {
		t.Errorf("current-boot marker removed unexpectedly: %v", err)
	}
}

func TestRemoveMarkerIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := WriteMarker(dir, "boot", 999999); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}
	if err := RemoveMarker(dir, 999999); err != nil {
		t.Fatalf("RemoveMarker: %v", err)
	}
	if err := RemoveMarker(dir, 999999); err != nil {
		t.Errorf("RemoveMarker on missing file: %v", err)
	}
}

func TestWriteMarkerRejectsLowPGID(t *testing.T) {
	dir := t.TempDir()
	if err := WriteMarker(dir, "boot", 1); err == nil {
		t.Error("WriteMarker(pgid=1) should error")
	}
}
