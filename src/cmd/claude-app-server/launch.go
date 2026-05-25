package main

import (
	"context"
	"io"
	"os"

	claudecli "github.com/takezoh/agent-roost/platform/lib/claude/cli"
	"github.com/takezoh/agent-roost/platform/procgroup"
)

// claudeLauncher starts a claude process and returns its stdout, a wait func,
// and any startup error.
//
//   - resumeSessionID is empty for a new session; non-empty triggers --resume.
//   - appendSystemPrompt, when non-empty, is passed via --append-system-prompt.
//   - extraEnv is appended to the inherited environment (e.g. TOOL_BRIDGE_SOCKET).
type claudeLauncher func(ctx context.Context, cwd, resumeSessionID, appendSystemPrompt, prompt string, extraEnv []string) (io.ReadCloser, func() error, error)

func realLaunch(ctx context.Context, cwd, resumeSessionID, appendSystemPrompt, prompt string, extraEnv []string) (io.ReadCloser, func() error, error) {
	bin := os.Getenv("CLAUDE_BIN")
	if bin == "" {
		bin = "claude"
	}

	// procgroup.Command runs claude in its own process group and SIGKILLs the
	// whole group on context cancellation, so claude's tool subprocesses are
	// terminated with it rather than orphaned.
	var env []string
	if len(extraEnv) > 0 {
		env = append(os.Environ(), extraEnv...)
	}
	cmd := procgroup.Command(procgroup.Spec{
		Ctx:  ctx,
		Bin:  bin,
		Args: claudecli.AppServerArgs(resumeSessionID, appendSystemPrompt, prompt),
		Dir:  cwd,
		Env:  env,
	})

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdout, cmd.Wait, nil
}
