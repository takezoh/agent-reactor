package devcontainer

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
)

// RunPostCreate executes the postCreateCommand inside the container.
// Must be called only once, immediately after container creation.
// argv is the exec-form command (e.g. ["bash", "-lc", "..."]).
// user is the effective user (remoteUser/containerUser); empty means default.
// A non-zero exit from the command is logged as a warning but does not fail
// session launch — matching devcontainer CLI behaviour.
func RunPostCreate(ctx context.Context, containerID, user string, argv []string) {
	if len(argv) == 0 {
		return
	}
	slog.Info("devcontainer: running postCreateCommand", "id", shortID(containerID), "user", user, "cmd", argv)

	execArgs := buildPostCreateArgs(containerID, user, argv)
	out, err := exec.CommandContext(ctx, "docker", execArgs...).CombinedOutput()
	if err != nil {
		slog.Warn("devcontainer: postCreateCommand failed (non-fatal)",
			"id", shortID(containerID), "err", err, "out", string(out))
		return
	}
	if s := string(out); s != "" {
		slog.Debug("devcontainer: postCreateCommand output", "id", shortID(containerID), "out", s)
	}
	slog.Info("devcontainer: postCreateCommand done", "id", shortID(containerID))
}

func buildPostCreateArgs(containerID, user string, argv []string) []string {
	out := make([]string, 0, 4+len(argv))
	out = append(out, "exec")
	if user != "" {
		out = append(out, "-u", user)
	}
	out = append(out, containerID)
	out = append(out, argv...)
	return out
}

// waitForContainer polls until the container responds to "docker exec true" or ctx expires.
// Used after docker start to confirm the container is accepting exec calls before
// running postCreateCommand or handing the container back to callers.
func waitForContainer(ctx context.Context, containerID string) error {
	for {
		err := exec.CommandContext(ctx, "docker", "exec", containerID, "true").Run()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("container %s did not become ready: %w", shortID(containerID), ctx.Err())
		default:
		}
	}
}
