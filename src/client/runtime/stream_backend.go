package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cstream "github.com/takezoh/agent-roost/client/runtime/subsystem/stream"
)

// streamRunDirKey returns the container key the project shares for stream-
// subsystem run-dir bookkeeping. Mirrors DevcontainerLauncher.RunDirKey:
// "__shared__" for shared isolation, the project path for project isolation,
// and the project path for host launches (no container). Empty when the
// devcontainer launcher isn't configured.
func (r *Runtime) streamRunDirKey(project string) string {
	l := launcher(r.cfg)
	if dl := devcontainerLauncherFor(l); dl != nil && l.IsContainer(project) {
		return dl.RunDirKey(project)
	}
	return project
}

// resolveStreamSockPaths returns the host-side and container-side sock paths
// for the given project. The container path equals the host path when the
// project runs directly on the host.
func (r *Runtime) resolveStreamSockPaths(project string) (string, string, error) {
	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), r.streamRunDirKey(project))
	if err != nil {
		return "", "", fmt.Errorf("stream backend: run dir: %w", err)
	}
	hostSock := filepath.Join(runDir, cstream.SockName)
	containerSock := hostSock
	if launcher(r.cfg).IsContainer(project) {
		containerSock = ContainerRunDir + "/" + cstream.SockName
	}
	return hostSock, containerSock, nil
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
