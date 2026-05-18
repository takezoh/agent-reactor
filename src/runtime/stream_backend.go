package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cstream "github.com/takezoh/agent-roost/runtime/subsystem/stream"
	"github.com/takezoh/agent-roost/sandbox"
	"github.com/takezoh/agent-roost/state"
)

// ensureStreamBackend returns the stream backend for the given subsystem ID,
// creating and starting it on first access.
func (r *Runtime) ensureStreamBackend(ctx context.Context, subsystemID state.SubsystemID, project string, cfg cstream.CommandConfig, opts state.StreamLaunchOptions) (*cstream.Backend, error) {
	if existing, ok := r.streamBackends.Load(subsystemID); ok {
		return existing.(*cstream.Backend), nil
	}

	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), project)
	if err != nil {
		return nil, fmt.Errorf("stream backend: run dir: %w", err)
	}
	sockPath := filepath.Join(runDir, cstream.SockName)
	containerSock := sockPath
	if launcher(r.cfg).IsContainer(project) {
		containerSock = ContainerRunDir + "/" + cstream.SockName
	}

	backend := cstream.New(
		r,
		subsystemID,
		project,
		cfg.ServerBin,
		cfg.ServerArgs,
		cfg.Model,
		opts.SandboxPolicy == state.StreamSandboxPolicyExternal,
		opts.ApprovalPolicy == state.StreamApprovalPolicyAutoApprove,
		sockPath,
		containerSock,
		cstream.LoopbackPort,
		func() state.FrameID { return r.activeFrameID },
	)
	actual, loaded := r.streamBackends.LoadOrStore(subsystemID, backend)
	if loaded {
		return actual.(*cstream.Backend), nil
	}
	if err := backend.Start(ctx); err != nil {
		r.streamBackends.Delete(subsystemID)
		return nil, err
	}
	return backend, nil
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
	inst, err := dl.mgr.EnsureInstance(ctx, project, "", sandbox.StartOptions{})
	if err != nil {
		return nil, err
	}
	cs := inst.Internal
	return &cstream.ContainerExecConfig{
		ContainerID: cs.ContainerID(),
		User:        cs.EffectiveUser(),
		WorkDir:     cs.WorkspaceTarget(),
		PreExec:     cs.PreExec(),
	}, nil
}
