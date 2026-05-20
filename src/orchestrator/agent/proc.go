package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// realProc launches "bash -lc command" with Dir=dir and the process environment
// set to os.Environ() extended by env. Returns stdout, stdin, and a wait func
// that reaps the process after stdout has been fully drained.
func realProc(ctx context.Context, dir string, env map[string]string, command string) (io.ReadCloser, io.WriteCloser, func(), error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", command) //nolint:gosec
	cmd.Dir = dir

	merged := os.Environ()
	for k, v := range env {
		merged = append(merged, k+"="+v)
	}
	cmd.Env = merged

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("agent: stdout pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("agent: stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("agent: start process: %w", err)
	}
	return stdout, stdin, func() { _ = cmd.Wait() }, nil
}
