package agentlaunch

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/takezoh/agent-roost/platform/procgroup"
)

// SpawnResult holds the stdio handles and lifecycle hooks for a spawned agent process.
type SpawnResult struct {
	Stdout io.ReadCloser
	Stdin  io.WriteCloser
	// Wait reaps the process. It should be called after Stdout is fully drained.
	Wait func() error
	// PID is the OS process ID; 0 if unavailable (e.g. process failed to start).
	PID int
}

// SpawnOptions controls optional spawn behaviour.
type SpawnOptions struct {
	// WaitDelay overrides the procgroup kill-grace period. 0 uses the default.
	WaitDelay time.Duration
	// Stderr captures the agent's stderr output (e.g. a prefix-capped buffer).
	// nil discards stderr.
	Stderr io.Writer
	// InheritEnv merges w.Env into os.Environ() when true.
	// When false, the process receives only the keys in w.Env.
	InheritEnv bool
}

// Spawn launches w.Argv[0] with w.Argv[1:] directly (no host-side shell).
// Working directory is w.StartDir; environment is built from w.Env per opts.InheritEnv.
// The process runs in its own process group via procgroup.Command.
func Spawn(ctx context.Context, w WrappedLaunch, opts SpawnOptions) (SpawnResult, error) {
	if len(w.Argv) == 0 {
		return SpawnResult{}, fmt.Errorf("agentlaunch: Spawn called with empty Argv")
	}

	env := buildEnv(w.Env, opts.InheritEnv)
	cmd := procgroup.Command(procgroup.Spec{
		Ctx:       ctx,
		Bin:       w.Argv[0],
		Args:      w.Argv[1:],
		Dir:       w.StartDir,
		Env:       env,
		WaitDelay: opts.WaitDelay,
	})
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SpawnResult{}, fmt.Errorf("agentlaunch: stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return SpawnResult{}, fmt.Errorf("agentlaunch: stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return SpawnResult{}, fmt.Errorf("agentlaunch: start process %s: %w", w.Argv[0], err)
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	return SpawnResult{
		Stdout: stdout,
		Stdin:  stdin,
		Wait:   cmd.Wait,
		PID:    pid,
	}, nil
}

func buildEnv(env map[string]string, inherit bool) []string {
	var base []string
	if inherit {
		base = os.Environ()
	}
	result := make([]string, 0, len(base)+len(env))
	result = append(result, base...)
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	if len(result) == 0 {
		return nil // nil = inherit os.Environ() via procgroup/exec
	}
	return result
}
