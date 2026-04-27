package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/sandbox"
	sandboxdc "github.com/takezoh/agent-roost/sandbox/devcontainer"
	"github.com/takezoh/agent-roost/state"
)

// DevcontainerLauncher wraps launches inside per-project devcontainers.
// It implements AgentLauncher by delegating to a sandbox.Manager[*devcontainer.ContainerState].
type DevcontainerLauncher struct {
	mgr            sandbox.Manager[*sandboxdc.ContainerState]
	resolveSandbox func(projectPath string) config.SandboxConfig
	proxy          *CredProxyRunner // nil when proxy disabled
}

// NewDevcontainerLauncher creates an AgentLauncher that runs agents inside devcontainers.
func NewDevcontainerLauncher(
	mgr sandbox.Manager[*sandboxdc.ContainerState],
	resolveSandbox func(string) config.SandboxConfig,
	proxy *CredProxyRunner,
) *DevcontainerLauncher {
	return &DevcontainerLauncher{
		mgr:            mgr,
		resolveSandbox: resolveSandbox,
		proxy:          proxy,
	}
}

// WrapLaunch ensures the project devcontainer is running and returns a launch
// spec that runs the agent via "docker exec".
// The image must already be built ("roost build <project>").
func (l *DevcontainerLauncher) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	if plan.Project == "" {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: plan.Project is empty for frame %s", frameID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	inst, err := l.mgr.EnsureInstance(ctx, plan.Project, "", sandbox.StartOptions{})
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: ensure instance: %w", err)
	}

	cmd, outEnv, err := l.mgr.BuildLaunchCommand(inst, plan, env)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: build command: %w", err)
	}

	l.mgr.AcquireFrame(inst)
	slog.Debug("devcontainer launcher: frame acquired", "frame", frameID, "project", plan.Project)

	return WrappedLaunch{
		Command:  cmd,
		StartDir: plan.StartDir,
		Env:      outEnv,
		Cleanup:  l.makeCleanup(frameID, inst),
	}, nil
}

// AdoptFrame reclaims an existing devcontainer for a pre-running frame.
func (l *DevcontainerLauncher) AdoptFrame(ctx context.Context, frameID state.FrameID, projectPath string) (func() error, error) {
	if projectPath == "" {
		return nil, nil
	}
	inst, err := l.mgr.EnsureInstance(ctx, projectPath, "", sandbox.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("devcontainer launcher: adopt frame %s: %w", frameID, err)
	}
	l.mgr.AcquireFrame(inst)
	slog.Debug("devcontainer launcher: frame adopted (warm start)", "frame", frameID, "project", projectPath)
	return l.makeCleanup(frameID, inst), nil
}

func (l *DevcontainerLauncher) makeCleanup(frameID state.FrameID, inst *sandbox.Instance[*sandboxdc.ContainerState]) func() error {
	return func() error {
		shouldDestroy := l.mgr.ReleaseFrame(inst)
		slog.Debug("devcontainer launcher: frame released",
			"frame", frameID, "project", inst.ProjectPath, "destroy", shouldDestroy)
		if !shouldDestroy {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return l.mgr.DestroyInstance(ctx, inst)
	}
}

// BuildOverlayFunc returns the OverlayFunc for the given sandbox resolver and proxy runner.
// dataDir is the daemon's data directory (e.g. ~/.roost); it contains roost.sock
// and the run/ directory tree.
// The returned function is called per-EnsureInstance to compute the roost-specific
// env/mounts overlay without triggering any image build.
func BuildOverlayFunc(resolveSandbox func(string) config.SandboxConfig, proxy *CredProxyRunner, dataDir string) sandboxdc.OverlayFunc {
	return func(projectPath, dcDir string) (sandboxdc.SpecOverlay, error) {
		sb := resolveSandbox(projectPath)
		dc := sb.Devcontainer

		allow := isProjectEnvScriptAllowed(projectPath, dc.AllowProjectEnvScript)
		scriptEnv := sandboxdc.RunEnvScript(context.Background(), dc.EnvScript, projectPath, allow)

		var proxySpec credproxy.Spec
		if proxy != nil {
			specCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var err error
			proxySpec, err = proxy.ContainerSpec(specCtx, projectPath, sb)
			if err != nil {
				slog.Warn("devcontainer: credproxy spec failed", "project", projectPath, "err", err)
			}
		}

		centralSock := filepath.Join(dataDir, "roost.sock")
		runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), projectPath, centralSock)
		if err != nil {
			return sandboxdc.SpecOverlay{}, fmt.Errorf("devcontainer: ensure run dir: %w", err)
		}

		env := make(map[string]string)
		for k, v := range scriptEnv {
			env[k] = v
		}
		for k, v := range proxySpec.Env {
			env[k] = v
		}
		env["ROOST_SOCKET"] = "/opt/roost/run/roost.sock"

		mounts := []string{
			fmt.Sprintf("type=bind,source=%s,target=/opt/roost/run", runDir),
			fmt.Sprintf("type=bind,source=%s,target=/opt/roost/devcontainer,readonly", dcDir),
		}

		return sandboxdc.SpecOverlay{Env: env, Mounts: mounts}, nil
	}
}

// isProjectEnvScriptAllowed checks whether projectPath is in the allowlist.
func isProjectEnvScriptAllowed(projectPath string, allowlist []string) bool {
	for _, p := range allowlist {
		if config.ExpandPath(p) == projectPath {
			return true
		}
	}
	return false
}
