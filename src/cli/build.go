package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/takezoh/agent-roost/config"
	sandboxdc "github.com/takezoh/agent-roost/sandbox/devcontainer"
)

func init() {
	Register("build", "Build the devcontainer image for a project (or --user for shared image)", runBuild)
}

// runBuild implements `roost build [--user] [<project>]`.
func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	userScope := fs.Bool("user", false, "build user-scope devcontainer image (~/.devcontainer)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()

	if *userScope {
		if len(rest) > 0 {
			return fmt.Errorf("build: --user and project path are mutually exclusive")
		}
		return runUserBuild()
	}
	return runProjectBuild(rest)
}

func runProjectBuild(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("build: specify a project path or use --user for user-scope build")
	}

	projectPath, err := resolveProject(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("build: load config: %w", err)
	}
	dc := cfg.Sandbox.Devcontainer

	cli, err := sandboxdc.NewCLI(dc.CLIPath)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	configPath, err := sandboxdc.ProjectBaseDC(projectPath)
	if err != nil {
		if errors.Is(err, sandboxdc.ErrNoProjectDevcontainer) {
			return fmt.Errorf("build: no .devcontainer/devcontainer.json found in %s; use 'roost build --user' for user-scope: %w", projectPath, sandboxdc.ErrNoProjectDevcontainer)
		}
		return fmt.Errorf("build: %w", err)
	}

	imageName := sandboxdc.ProjectScopeImageForPath(projectPath)
	fmt.Fprintf(os.Stdout, "roost build: building project image for %s...\n", projectPath)
	image, err := cli.Build(context.Background(), projectPath, configPath, imageName, dc.ExtraBuildArgs)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	fmt.Fprintf(os.Stdout, "roost build: done — image: %s\n", image)
	return nil
}

func runUserBuild() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("build: load config: %w", err)
	}
	dc := cfg.Sandbox.Devcontainer

	cli, err := sandboxdc.NewCLI(dc.CLIPath)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	configPath, err := sandboxdc.UserBaseDC()
	if err != nil {
		if errors.Is(err, sandboxdc.ErrNoUserDevcontainer) {
			return fmt.Errorf("build: no ~/.devcontainer/devcontainer.json found; create one to use user-scope image: %w", sandboxdc.ErrNoUserDevcontainer)
		}
		return fmt.Errorf("build: %w", err)
	}

	home, _ := os.UserHomeDir()
	imageName := sandboxdc.UserScopeImage()
	fmt.Fprintf(os.Stdout, "roost build: building user-scope image...\n")
	image, err := cli.Build(context.Background(), home, configPath, imageName, dc.ExtraBuildArgs)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	fmt.Fprintf(os.Stdout, "roost build: done — image: %s\n", image)
	return nil
}

func resolveProject(args []string) (string, error) {
	abs, err := filepath.Abs(args[0])
	if err != nil {
		return "", fmt.Errorf("build: resolve project path: %w", err)
	}
	return abs, nil
}
