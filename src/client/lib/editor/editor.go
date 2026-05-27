// Package editor launches an external code editor on a path.
// It is a thin wrapper over exec that detaches the child process so the
// caller is not blocked waiting for the editor to close.
package editor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveTarget returns the best target to hand to the editor for dir.
// If any file with an extension in extensions exists directly inside dir, the
// lexicographically first one is returned. Otherwise dir itself is returned.
//
// os.ReadDir is used instead of filepath.Glob so that directory names
// containing glob metacharacters (e.g. "proj[1]") are never misinterpreted
// as patterns, which would silently scan a different directory.
func ResolveTarget(dir string, extensions []string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir
	}
	var targets []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		for _, ext := range extensions {
			if strings.HasSuffix(e.Name(), ext) {
				targets = append(targets, filepath.Join(dir, e.Name()))
				break
			}
		}
	}
	if len(targets) == 0 {
		return dir
	}
	sort.Strings(targets)
	return targets[0]
}

// Launch starts the editor named by command on target and returns
// without waiting for it to exit. command may include flags
// (e.g. "code --reuse-window"); they are split on whitespace.
func Launch(command, target string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("editor: empty command")
	}
	bin := parts[0]
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("editor: %q not found in PATH: %w", bin, err)
	}
	args := make([]string, 0, len(parts[1:])+1)
	args = append(args, parts[1:]...)
	args = append(args, target)
	slog.Info("editor: launching", "bin", resolved, "target", target)
	cmd := exec.CommandContext(context.Background(), resolved, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("editor: start %s: %w", resolved, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
