// Package devcontainer implements sandbox.Manager using direct docker commands.
// @devcontainers/cli is used only for image build (devcontainer build).
// Container lifecycle (create/start/exec/rm) is managed by roost directly.
package devcontainer

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/takezoh/agent-roost/procio"
)

// CLI wraps the @devcontainers/cli tool for image build only.
type CLI struct {
	binPath string
}

// NewCLI returns a CLI using binPath (PATH-resolved name or absolute path).
// Returns error when the binary cannot be found.
func NewCLI(binPath string) (*CLI, error) {
	if binPath == "" {
		binPath = "devcontainer"
	}
	if _, err := exec.LookPath(binPath); err != nil {
		return nil, fmt.Errorf("devcontainer CLI not found (%q): %w\n  install: npm install -g @devcontainers/cli", binPath, err)
	}
	return &CLI{binPath: binPath}, nil
}

// Build runs "devcontainer build" and returns the built image name.
func (c *CLI) Build(ctx context.Context, workspaceFolder, configPath, imageName string, extraArgs []string) (string, error) {
	args := []string{
		"build",
		"--workspace-folder", workspaceFolder,
		"--config", configPath,
		"--image-name", imageName,
	}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, c.binPath, args...)
	cmd.Stdout = procio.Stdout()
	cmd.Stderr = procio.Stderr()

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("devcontainer build: %w", err)
	}
	return imageName, nil
}
