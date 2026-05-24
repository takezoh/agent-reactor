package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cstream "github.com/takezoh/agent-roost/client/runtime/subsystem/stream"
	"github.com/takezoh/agent-roost/client/state"
)

// resolveStreamSockPaths returns the host-side and container-side sock paths
// for the given session and project. Each session gets a unique sock file so
// multiple concurrent app-server processes do not collide.
// project is provided by the caller (from the spawn effect) so that this
// function can be called safely from a goroutine without accessing r.state.
func (r *Runtime) resolveStreamSockPaths(sessionID state.SessionID, project string) (string, string, error) {
	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	// All host-mode session sockets share a single directory. The routing
	// sockbridge watches this directory and routes by session ID.
	runDir, err := ensureStreamRunDir(filepath.Join(dataDir, "run", cstream.RunDirName))
	if err != nil {
		return "", "", fmt.Errorf("stream backend: run dir: %w", err)
	}
	sockName := cstream.SockPrefix + string(sessionID) + cstream.SockSuffix
	hostSock := filepath.Join(runDir, sockName)
	containerSock := hostSock
	if launcher(r.cfg).IsContainer(project) {
		// Container sockets use the fixed container run dir so the in-container
		// routing bridge can find them by session ID.
		containerSock = ContainerRunDir + "/" + sockName
	}
	return hostSock, containerSock, nil
}

// ensureStreamRunDir creates the stream run directory if it does not exist.
func ensureStreamRunDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ContainerExecConfig implements stream.RuntimeHook: returns docker exec
// parameters for the project's devcontainer, or nil for host projects.
func (r *Runtime) ContainerExecConfig(ctx context.Context, project string) (*cstream.ContainerExecConfig, error) {
	if !launcher(r.cfg).IsContainer(project) {
		return nil, nil
	}
	dl := devcontainerLauncherFor(launcher(r.cfg))
	if dl == nil {
		return nil, fmt.Errorf("runtime: unsupported container launcher for stream backend")
	}
	info, err := dl.GetContainerExecInfo(ctx, project)
	if err != nil {
		return nil, err
	}
	return &cstream.ContainerExecConfig{
		ContainerID: info.ContainerID,
		User:        info.User,
		WorkDir:     info.WorkDir,
		PreExec:     info.PreExec,
	}, nil
}
