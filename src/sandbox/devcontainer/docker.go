package devcontainer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// ContainerInfo holds basic info from "docker ps -a".
type ContainerInfo struct {
	ID    string
	State string // "running", "exited", "created", etc.
}

// FindSharedContainer returns the roost-shared container, or nil if not found.
func FindSharedContainer(ctx context.Context) (*ContainerInfo, error) {
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", "label=roost-managed=1",
		"--filter", "label=roost-isolation=shared",
		"--format", "{{.ID}}\t{{.State}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps (shared): %w", err)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return nil, nil
	}
	parts := strings.SplitN(strings.SplitN(line, "\n", 2)[0], "\t", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("docker ps (shared): unexpected output %q", line)
	}
	return &ContainerInfo{ID: parts[0], State: parts[1]}, nil
}

// FindContainer returns the first roost-managed container for projectPath, or nil.
func FindContainer(ctx context.Context, projectPath string) (*ContainerInfo, error) {
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", "label=roost-managed=1",
		"--filter", "label=roost-project="+projectPath,
		"--format", "{{.ID}}\t{{.State}}",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return nil, nil
	}
	// take first result only
	parts := strings.SplitN(strings.SplitN(line, "\n", 2)[0], "\t", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("docker ps: unexpected output %q", line)
	}
	return &ContainerInfo{ID: parts[0], State: parts[1]}, nil
}

// ImageEnv returns the image's Config.Env as a key→value map.
func ImageEnv(ctx context.Context, imageName string) (map[string]string, error) {
	out, err := exec.CommandContext(ctx,
		"docker", "image", "inspect",
		"--format", "{{json .Config.Env}}", imageName,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker image inspect env: %w", err)
	}
	var lines []string
	if err := json.Unmarshal(bytes.TrimSpace(out), &lines); err != nil {
		return nil, fmt.Errorf("parse image env: %w", err)
	}
	env := make(map[string]string, len(lines))
	for _, line := range lines {
		if i := strings.IndexByte(line, '='); i > 0 {
			env[line[:i]] = line[i+1:]
		}
	}
	return env, nil
}

// ImageExists reports whether the named image exists locally.
func ImageExists(ctx context.Context, imageName string) (bool, error) {
	err := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.ID}}", imageName).Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("docker image inspect: %w", err)
	}
	return true, nil
}

// CreateContainer runs "docker create <args>" and returns the container ID.
func CreateContainer(ctx context.Context, args []string) (string, error) {
	fullArgs := append([]string{"create"}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		slog.Error("devcontainer: docker create failed",
			"err", err,
			"stderr", strings.TrimSpace(stderr.String()),
			"args", fullArgs)
		return "", fmt.Errorf("docker create: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// StartContainer runs "docker start <containerID>".
func StartContainer(ctx context.Context, containerID string) error {
	out, err := exec.CommandContext(ctx, "docker", "start", containerID).CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			slog.Error("devcontainer: start timed out",
				"id", containerID,
				"recover", "docker rm -f "+containerID)
			return fmt.Errorf("docker start %s: timed out", shortID(containerID))
		}
		return fmt.Errorf("docker start %s: %w\n%s", shortID(containerID), err, string(out))
	}
	return nil
}

// RemoveContainer runs "docker rm -f <containerID>".
func RemoveContainer(ctx context.Context, containerID string) error {
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rm %s: %w\n%s", shortID(containerID), err, string(out))
	}
	return nil
}
