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
	ID        string
	State     string // "running", "exited", "created", etc.
	MountHash string // <prefix>-mount-hash label; "" for containers created before the label existed
}

// effectivePrefix returns prefix when non-empty or DefaultNamePrefix otherwise.
// Keeps docker.go independent of the Manager's NamePrefix-resolution path while
// preserving the legacy "reactor" prefix as the default.
func effectivePrefix(prefix string) string {
	if prefix == "" {
		return DefaultNamePrefix
	}
	return prefix
}

// psFormatFor returns the docker ps --format string for the given prefix.
// The mount-hash label key is prefix-scoped so containers created by a peer
// daemon under a different prefix report MountHash="" here and never match
// this daemon's --filter, keeping the two name-spaces fully separate.
func psFormatFor(prefix string) string {
	return "{{.ID}}\t{{.State}}\t{{.Label \"" + effectivePrefix(prefix) + "-mount-hash\"}}"
}

// parsePsLine parses one line of "docker ps" output produced by psFormatFor.
func parsePsLine(line string) (*ContainerInfo, error) {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("unexpected output %q", line)
	}
	info := &ContainerInfo{ID: parts[0], State: parts[1]}
	if len(parts) >= 3 {
		info.MountHash = parts[2]
	}
	return info, nil
}

// FindSharedContainer returns the shared container owned by `prefix`, or nil.
// Containers carrying a different prefix's labels are invisible.
func FindSharedContainer(ctx context.Context, prefix string) (*ContainerInfo, error) {
	p := effectivePrefix(prefix)
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", "label="+p+"-managed=1",
		"--filter", "label="+p+"-isolation=shared",
		"--format", psFormatFor(p),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps (shared): %w", err)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return nil, nil
	}
	info, err := parsePsLine(strings.SplitN(line, "\n", 2)[0])
	if err != nil {
		return nil, fmt.Errorf("docker ps (shared): %w", err)
	}
	return info, nil
}

// FindContainer returns the first container for projectPath owned by `prefix`,
// or nil. Containers carrying a different prefix's labels are invisible.
func FindContainer(ctx context.Context, prefix, projectPath string) (*ContainerInfo, error) {
	p := effectivePrefix(prefix)
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", "label="+p+"-managed=1",
		"--filter", "label="+p+"-project="+projectPath,
		"--format", psFormatFor(p),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return nil, nil
	}
	info, err := parsePsLine(strings.SplitN(line, "\n", 2)[0])
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	return info, nil
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
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
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

// StopContainer runs "docker stop -t 5 <containerID>".
// The container is preserved (no rm) so a later StartContainer can resume it.
func StopContainer(ctx context.Context, containerID string) error {
	out, err := exec.CommandContext(ctx, "docker", "stop", "-t", "5", containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop %s: %w\n%s", shortID(containerID), err, string(out))
	}
	return nil
}
