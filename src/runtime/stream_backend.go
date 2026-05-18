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

// prepareStreamLaunch resolves a stream-subsystem LaunchPlan into the pane
// command that attaches the codex TUI to the shared app-server via sockbridge.
func (r *Runtime) prepareStreamLaunch(frameID state.FrameID, subsystemID state.SubsystemID, plan state.LaunchPlan) (state.LaunchPlan, error) {
	if plan.Subsystem != state.LaunchSubsystemStream {
		return plan, nil
	}
	cfg, err := cstream.ParseCommand(plan.Command)
	if err != nil {
		return plan, err
	}
	backend, err := r.ensureStreamBackend(subsystemID, plan.Project, cfg, plan.Stream)
	if err != nil {
		return plan, err
	}
	threadID, err := backend.BindFrame(frameID, plan.StartDir, plan.Stream, plan.Stdin)
	if err != nil {
		return plan, err
	}
	plan.Command = cstream.BuildRemoteCommand(backend.BridgePort(), threadID, plan.StartDir)
	plan.Stdin = nil
	plan.Stream.ResumeThreadID = threadID
	return plan, nil
}

func (r *Runtime) ensureStreamBackend(subsystemID state.SubsystemID, project string, cfg cstream.CommandConfig, opts state.StreamLaunchOptions) (*cstream.Backend, error) {
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
	if err := backend.Start(); err != nil {
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
