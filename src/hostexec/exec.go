package hostexec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/takezoh/agent-roost/config"
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
	name   string // binary name (first non-env-assignment token of allow patterns)
	policy *Policy
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

func executeRequest(ctx context.Context, e *entry, project string, req Request, fds [3]int) int {
	stdin := os.NewFile(uintptr(fds[0]), "stdin")
	stdout := os.NewFile(uintptr(fds[1]), "stdout")
	stderr := os.NewFile(uintptr(fds[2]), "stderr")
	defer stdin.Close()
	defer stdout.Close()
	defer stderr.Close()

	argv := append([]string{req.Binary}, req.Args...)
	if err := e.policy.Check(argv); err != nil {
		slog.Warn("hostexec: request rejected", "project", project, "binary", req.Binary, "err", err)
		fmt.Fprintf(stderr, "host-exec: %v\n", err)
		return 126
	}

	slog.Info("hostexec: exec", "project", project, "binary", e.name, "args", req.Args)

	cmd := exec.CommandContext(ctx, e.name, req.Args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if req.Cwd != "" {
		if _, err := os.Stat(req.Cwd); err == nil {
			cmd.Dir = req.Cwd
		}
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
