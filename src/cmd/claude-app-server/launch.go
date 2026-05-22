package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// claudeLauncher starts a claude process and returns its stdout, a wait func,
// and any startup error.
//
//   - resumeSessionID is empty for a new session; non-empty triggers --resume.
//   - appendSystemPrompt, when non-empty, is passed via --append-system-prompt.
//   - extraEnv is appended to the inherited environment (e.g. TOOL_BRIDGE_SOCKET).
type claudeLauncher func(ctx context.Context, cwd, resumeSessionID, appendSystemPrompt, prompt string, extraEnv []string) (io.ReadCloser, func() error, error)

// claudeArgs builds the claude CLI argv. --verbose is mandatory: current claude
// versions reject `-p --output-format stream-json` without it ("requires --verbose").
func claudeArgs(resumeSessionID, appendSystemPrompt, prompt string) []string {
	args := []string{"-p", "--output-format", "stream-json", "--verbose"}
	if appendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", appendSystemPrompt)
	}
	if resumeSessionID != "" {
		args = append(args, "--resume", resumeSessionID)
	}
	return append(args, prompt)
}

func realLaunch(ctx context.Context, cwd, resumeSessionID, appendSystemPrompt, prompt string, extraEnv []string) (io.ReadCloser, func() error, error) {
	bin := os.Getenv("CLAUDE_BIN")
	if bin == "" {
		bin = "claude"
	}

	cmd := exec.CommandContext(ctx, bin, claudeArgs(resumeSessionID, appendSystemPrompt, prompt)...) //nolint:gosec
	cmd.Dir = cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Kill the whole process group on context cancellation so claude's children
	// (tool subprocesses) are also terminated.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second

	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdout, cmd.Wait, nil
}
