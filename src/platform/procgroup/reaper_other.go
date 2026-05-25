//go:build !linux

package procgroup

// The crash-path reaper relies on POSIX process groups and is Linux-only.
// On other platforms these are no-ops so callers need no build-tag branching.

func WriteMarker(_, _ string, _ int) error { return nil }

func RemoveMarker(_ string, _ int) error { return nil }

func PruneOrphans(_, _ string) error { return nil }
