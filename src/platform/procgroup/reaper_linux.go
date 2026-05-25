//go:build linux

package procgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// markerExt suffixes process-group marker files written under the reaper dir.
const markerExt = ".pgid"

// WriteMarker records a live process group under dir so a future daemon boot
// can reap it if this process dies without cleaning up (SIGKILL/crash). The
// marker filename encodes the pgid; its content is the current bootNonce so the
// reaper can distinguish this boot's groups from a prior boot's leftovers.
//
// pgid must be the process-group id (== leader pid when started via Command on
// Linux, i.e. cmd.Process.Pid). Values <= 1 are rejected to avoid ever
// addressing pid 1 / the whole session.
func WriteMarker(dir, bootNonce string, pgid int) error {
	if pgid <= 1 {
		return fmt.Errorf("procgroup: refusing marker for pgid %d", pgid)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("procgroup: marker dir: %w", err)
	}
	path := filepath.Join(dir, strconv.Itoa(pgid)+markerExt)
	if err := os.WriteFile(path, []byte(bootNonce), 0o600); err != nil {
		return fmt.Errorf("procgroup: write marker: %w", err)
	}
	return nil
}

// RemoveMarker deletes the marker for pgid after a normal Wait. Missing files
// are not an error.
func RemoveMarker(dir string, pgid int) error {
	path := filepath.Join(dir, strconv.Itoa(pgid)+markerExt)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("procgroup: remove marker: %w", err)
	}
	return nil
}

// PruneOrphans is called once at daemon startup. It scans dir for markers left
// by a *different* boot (bootNonce mismatch), SIGKILLs any still-alive process
// group, and removes the marker. Markers matching the current bootNonce are
// left untouched (they belong to this boot's live groups).
func PruneOrphans(dir, bootNonce string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("procgroup: read marker dir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, markerExt) {
			continue
		}
		path := filepath.Join(dir, name)
		pgid, perr := strconv.Atoi(strings.TrimSuffix(name, markerExt))
		if perr != nil || pgid <= 1 {
			_ = os.Remove(path)
			continue
		}
		data, rerr := os.ReadFile(path)
		if rerr == nil && string(data) == bootNonce {
			continue // belongs to the current boot
		}
		// Stale group from a prior boot: kill it if still alive, then drop the marker.
		if syscall.Kill(-pgid, 0) == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
		_ = os.Remove(path)
	}
	return nil
}
