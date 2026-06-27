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
	"syscall"
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

// Launch starts the editor named by command on target and returns without
// waiting for it to exit. command may include flags (e.g. "code
// --reuse-window"); they are split on whitespace. Under WSL the launcher
// (e.g. the `code` script) detects the distro itself and opens the folder via
// Remote-WSL, so no --remote flag is added here.
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
	args := make([]string, 0, len(parts))
	args = append(args, parts[1:]...)
	args = append(args, target)
	slog.Info("editor: launching", "bin", resolved, "args", args)
	cmd := exec.CommandContext(context.Background(), resolved, args...)
	// The palette runs in a short-lived backend popup process; when it exits the
	// popup is torn down and any child sharing its session is killed by SIGHUP
	// before the editor can open. Detach the child into its own session so it
	// outlives the palette, and point its stdout/stderr at a regular file so it
	// keeps a stable, writable target once the popup's terminal is gone (the
	// Remote-WSL launcher relays that stdio to Windows Code.exe and the null
	// device breaks the relay).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// Use a unique temp file (CreateTemp uses O_EXCL + a random name, so an
	// attacker cannot pre-plant a symlink at a predictable path to redirect or
	// truncate another file) and unlink it right after Start: the child keeps
	// the inherited fd, so the name can go away leaving no litter.
	if logf, ferr := os.CreateTemp("", "reactor-editor-*.log"); ferr == nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
		defer func() {
			_ = os.Remove(logf.Name())
			_ = logf.Close()
		}()
	} else {
		slog.Warn("editor: cannot open launch log; output discarded", "err", ferr)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("editor: start %s: %w", resolved, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
