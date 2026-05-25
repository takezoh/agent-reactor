package hostexec

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/takezoh/agent-roost/platform/config"
	"golang.org/x/term"
)

// validBinaryNameRe restricts binary names to safe filesystem and shell tokens.
// Names with metacharacters (;, $, |, etc.) could inject code into the shim script.
var validBinaryNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func validBinaryName(name string) error {
	if !validBinaryNameRe.MatchString(name) {
		return fmt.Errorf("invalid binary name %q: must match [a-zA-Z0-9][a-zA-Z0-9._-]*", name)
	}
	return nil
}

// entry is the compiled form of a config.HostExecConfig: binary name and pre-compiled policy.
type entry struct {
	name     string // binary name used for policy matching and PATH lookup
	execPath string // full path to execute; when non-empty, overrides PATH lookup
	policy   *Policy
}

// compileEntries builds a map of entries keyed by binary name.
// Each distinct first non-env-assignment token across all allow patterns becomes one entry.
// The shared policy (all allow/deny patterns) is applied to every binary.
func compileEntries(cfg config.HostExecConfig) (map[string]*entry, error) {
	if len(cfg.Allow) == 0 {
		return nil, fmt.Errorf("hostexec: allow must not be empty")
	}
	pol, err := CompilePolicy(cfg.Allow, cfg.Deny)
	if err != nil {
		return nil, fmt.Errorf("hostexec: compile policy: %w", err)
	}
	seen := map[string]struct{}{}
	for _, pat := range cfg.Allow {
		if fields := skipEnvAssignments(strings.Fields(pat)); len(fields) > 0 {
			name := fields[0]
			if err := validBinaryName(name); err != nil {
				return nil, fmt.Errorf("hostexec: %w", err)
			}
			seen[name] = struct{}{}
		}
	}
	m := make(map[string]*entry, len(seen))
	for name := range seen {
		m[name] = &entry{name: name, policy: pol}
	}
	return m, nil
}

// OverlayAlias returns a stable, broker-safe alias for a canonicalized container target path.
// Using CRC32 of the path avoids basename collisions between overlay entries.
func OverlayAlias(canonicalDst string) string {
	return fmt.Sprintf("ov%08x", crc32.ChecksumIEEE([]byte(canonicalDst)))
}

// compileOverlayEntry builds an entry for a single overlay target.
// hostBinary is the basename used for policy matching; execPath is the full host path to exec.
// Empty allow = default-deny (same semantics as global host_exec allow).
func compileOverlayEntry(hostBinary, execPath string, allow, deny []string) (*entry, error) {
	if err := validBinaryName(hostBinary); err != nil {
		return nil, fmt.Errorf("hostexec overlay: %w", err)
	}
	pol, err := CompilePolicy(allow, deny)
	if err != nil {
		return nil, fmt.Errorf("hostexec overlay %q: %w", hostBinary, err)
	}
	return &entry{name: hostBinary, execPath: execPath, policy: pol}, nil
}

func executeRequest(ctx context.Context, e *entry, project string, req Request, fds [3]int, callerPID int) int {
	stdin := os.NewFile(uintptr(fds[0]), "stdin")
	stdout := os.NewFile(uintptr(fds[1]), "stdout")
	stderr := os.NewFile(uintptr(fds[2]), "stderr")
	defer stdin.Close()
	defer stdout.Close()
	defer stderr.Close()

	argv := append([]string{e.name}, req.Args...)
	if err := e.policy.Check(argv); err != nil {
		slog.Warn("hostexec: request rejected", "project", project, "binary", req.Binary, "caller_pid", callerPID, "caller", procComm(callerPID), "err", err)
		fmt.Fprintf(stderr, "host-exec: %v\n", err)
		return 126
	}

	execBin := e.name
	if e.execPath != "" {
		execBin = e.execPath
	}
	slog.Info("hostexec: exec", "project", project, "binary", execBin, "args", req.Args)

	cmd := exec.CommandContext(ctx, execBin, req.Args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if req.Cwd != "" {
		if _, err := os.Stat(req.Cwd); err == nil {
			cmd.Dir = req.Cwd
		}
	}
	// Start a new session so the child gets its own process group.
	// We intentionally do NOT set Setctty: the pts fd from the container is already
	// the controlling terminal of the container session; TIOCSCTTY on a pty owned by
	// another session returns EPERM and aborts the exec entirely.
	if term.IsTerminal(fds[0]) {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		slog.Warn("hostexec: exec failed", "project", project, "binary", e.name, "err", err)
		fmt.Fprintf(stderr, "host-exec: exec %s: %v\n", e.name, err)
		return 1
	}
	return 0
}
