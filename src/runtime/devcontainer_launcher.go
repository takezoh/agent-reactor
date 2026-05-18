package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/lib/pathmap"
	cstream "github.com/takezoh/agent-roost/runtime/subsystem/stream"
	"github.com/takezoh/agent-roost/sandbox"
	sandboxdc "github.com/takezoh/agent-roost/sandbox/devcontainer"
	"github.com/takezoh/agent-roost/state"
	"github.com/takezoh/credproxy/container"
)

// DevcontainerLauncher wraps launches inside per-project devcontainers.
// It implements AgentLauncher by delegating to a sandbox.Manager[*devcontainer.ContainerState].
type DevcontainerLauncher struct {
	mgr                 sandbox.Manager[*sandboxdc.ContainerState]
	resolveSandbox      func(projectPath string) config.SandboxConfig
	resolveProjectScope func(projectPath string) *config.SandboxConfig
	projectsConfig      config.ProjectsConfig
	proxy               *CredProxyRunner // nil when proxy disabled
	dataDir             string
}

// NewDevcontainerLauncher creates an AgentLauncher that runs agents inside devcontainers.
// dataDir is the daemon's data directory (e.g. ~/.roost); it contains the run/ subtree.
// resolveProjectScope returns the raw project-scope SandboxConfig (nil if absent).
// projectsConfig is used to enumerate workspace dirs for shared-mode containers.
func NewDevcontainerLauncher(
	mgr sandbox.Manager[*sandboxdc.ContainerState],
	resolveSandbox func(string) config.SandboxConfig,
	resolveProjectScope func(string) *config.SandboxConfig,
	projectsConfig config.ProjectsConfig,
	proxy *CredProxyRunner,
	dataDir string,
) *DevcontainerLauncher {
	return &DevcontainerLauncher{
		mgr:                 mgr,
		resolveSandbox:      resolveSandbox,
		resolveProjectScope: resolveProjectScope,
		projectsConfig:      projectsConfig,
		proxy:               proxy,
		dataDir:             dataDir,
	}
}

const containerEnsureTimeout = 120 * time.Second

// WrapLaunch ensures the project devcontainer is running and returns a launch
// spec that runs the agent via "docker exec".
// The image must already be built ("roost build <project>").
func (l *DevcontainerLauncher) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	if plan.Project == "" {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: plan.Project is empty for frame %s", frameID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), containerEnsureTimeout)
	defer cancel()

	opts := l.resolveStartOptions(plan.Project)
	inst, err := l.mgr.EnsureInstance(ctx, plan.Project, "", opts)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: ensure instance: %w", err)
	}

	cmd, outEnv, err := l.mgr.BuildLaunchCommand(inst, plan, env)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: build command: %w", err)
	}

	runDir, err := EnsureProjectRunDir(filepath.Join(l.dataDir, "run"), l.runDirKey(plan.Project, opts))
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: ensure run dir: %w", err)
	}

	l.mgr.AcquireFrame(inst)
	slog.Debug("devcontainer launcher: frame acquired", "frame", frameID, "project", plan.Project, "shared", opts.SharedMode)

	wsHost, wsContainer := plan.Project, inst.Internal.WorkspaceTarget()
	if inst.Internal.IsShared() {
		wsHost, wsContainer = "", ""
	}
	mounts := buildMounts(wsHost, wsContainer, runDir, inst.Internal.BindMounts())

	startDir := plan.StartDir
	if containerPath, ok := mounts.ToContainer(startDir); ok {
		startDir = containerPath
	}

	return WrappedLaunch{
		Command:          cmd,
		StartDir:         startDir,
		Env:              outEnv,
		Cleanup:          l.makeCleanup(frameID, inst),
		Subsystem:        plan.Subsystem,
		Stream:           plan.Stream,
		ContainerSockDir: runDir,
		Mounts:           mounts,
	}, nil
}

// runDirKey returns the key to use for the per-container run directory.
func (l *DevcontainerLauncher) runDirKey(projectPath string, opts sandbox.StartOptions) string {
	if opts.SharedMode {
		return sandboxdc.SharedContainerKey
	}
	return projectPath
}

// resolveStartOptions determines whether to use shared or project isolation for
// projectPath, and builds the corresponding sandbox.StartOptions.
func (l *DevcontainerLauncher) resolveStartOptions(projectPath string) sandbox.StartOptions {
	// Project has its own .devcontainer — always project isolation (cheap check first).
	if _, err := sandboxdc.ProjectBaseDC(projectPath); err == nil {
		return sandbox.StartOptions{}
	}

	projScope := l.resolveProjectScope(projectPath)
	// Project opts out of shared mode or specifies its own devcontainer path.
	if projScope != nil && (projScope.Isolation == "project" || projScope.Devcontainer.Path != "") {
		return sandbox.StartOptions{DevcontainerDir: config.ExpandPath(projScope.Devcontainer.Path)}
	}

	userSandbox := l.resolveSandbox("")
	if userSandbox.Isolation != "shared" {
		return sandbox.StartOptions{}
	}

	dcDir := config.ExpandPath(userSandbox.Devcontainer.Path)
	prefix := userSandbox.Devcontainer.HostPathMountPrefix
	return sandbox.StartOptions{
		SharedMode:      true,
		DevcontainerDir: dcDir,
		ExtraMounts:     l.sharedWorkspaceMounts(prefix),
	}
}

// sharedWorkspaceMounts builds "type=bind,source=X,target=Y" mount specs for all
// configured project roots and paths. prefix is HostPathMountPrefix (may be "").
func (l *DevcontainerLauncher) sharedWorkspaceMounts(prefix string) []string {
	seen := map[string]struct{}{}
	var mounts []string
	add := func(hostPath string) {
		if _, dup := seen[hostPath]; dup {
			return
		}
		seen[hostPath] = struct{}{}
		containerPath := resolveWorkspaceFallback(hostPath, prefix)
		mounts = append(mounts, fmt.Sprintf("type=bind,source=%s,target=%s,consistency=cached", hostPath, containerPath))
	}
	for _, root := range l.projectsConfig.ProjectRoots {
		root = config.ExpandPath(root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	for _, p := range l.projectsConfig.ProjectPaths {
		p = config.ExpandPath(p)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			add(p)
		}
	}
	return mounts
}

// buildMounts constructs the pathmap.Mounts for a devcontainer instance.
// Without the user-declared bind mounts here, driver-specific data paths that
// are bind-mounted into the container get cleared at the IPC boundary and
// driver tab views silently empty.
func buildMounts(hostProject, containerWS, hostRunDir string, userBinds []sandboxdc.BindMount) pathmap.Mounts {
	type key = [2]string
	seen := map[key]bool{}
	add := func(ms *pathmap.Mounts, host, container string) {
		k := key{host, container}
		if !seen[k] {
			seen[k] = true
			*ms = append(*ms, pathmap.Mount{Host: host, Container: container})
		}
	}
	var ms pathmap.Mounts
	if hostProject != "" && containerWS != "" {
		add(&ms, hostProject, containerWS)
	}
	if hostRunDir != "" {
		add(&ms, hostRunDir, ContainerRunDir)
	}
	for _, b := range userBinds {
		add(&ms, b.Source, b.Target)
	}
	return ms
}

func (l *DevcontainerLauncher) IsContainer(_ string) bool { return true }

// EnsureProject ensures the devcontainer for projectPath is running.
func (l *DevcontainerLauncher) EnsureProject(ctx context.Context, projectPath string) error {
	ctx, cancel := context.WithTimeout(ctx, containerEnsureTimeout)
	defer cancel()
	opts := l.resolveStartOptions(projectPath)
	_, err := l.mgr.EnsureInstance(ctx, projectPath, "", opts)
	if err != nil {
		return fmt.Errorf("devcontainer launcher: ensure project %s: %w", projectPath, err)
	}
	return nil
}

// AdoptFrame reclaims an existing devcontainer for a pre-running frame.
func (l *DevcontainerLauncher) AdoptFrame(ctx context.Context, frameID state.FrameID, projectPath string) (func() error, pathmap.Mounts, error) {
	if projectPath == "" {
		return nil, nil, nil
	}
	opts := l.resolveStartOptions(projectPath)
	inst, err := l.mgr.EnsureInstance(ctx, projectPath, "", opts)
	if err != nil {
		return nil, nil, fmt.Errorf("devcontainer launcher: adopt frame %s: %w", frameID, err)
	}
	l.mgr.AcquireFrame(inst)
	slog.Debug("devcontainer launcher: frame adopted (warm start)", "frame", frameID, "project", projectPath, "shared", opts.SharedMode)

	runDir := ProjectRunDir(filepath.Join(l.dataDir, "run"), l.runDirKey(projectPath, opts))
	wsHost, wsContainer := projectPath, inst.Internal.WorkspaceTarget()
	if inst.Internal.IsShared() {
		wsHost, wsContainer = "", ""
	}
	mounts := buildMounts(wsHost, wsContainer, runDir, inst.Internal.BindMounts())
	return l.makeCleanup(frameID, inst), mounts, nil
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
// dataDir is the daemon's data directory (e.g. ~/.roost); it contains the run/ directory tree.
// postCreateSubcmds lists driver-specific setup commands; the caller supplies these to enforce
// driver/runtime isolation — runtime itself has no knowledge of driver names.
// The returned function is called per-EnsureInstance to compute the roost-specific
// env/mounts overlay without triggering any image build.
func BuildOverlayFunc(resolveSandbox func(string) config.SandboxConfig, proxy *CredProxyRunner, dataDir string, postCreateSubcmds []string) sandboxdc.OverlayFunc {
	return func(instanceKey, projectPath, dcDir string) (sandboxdc.SpecOverlay, error) {
		// For shared containers use user-scope config only (no project-scope settings).
		configKey := projectPath
		if instanceKey == sandboxdc.SharedContainerKey {
			configKey = ""
		}
		sb := resolveSandbox(configKey)
		dc := sb.Devcontainer

		proxySpec, scriptEnv, err := resolveOverlaySpecs(proxy, projectPath, dc)
		if err != nil {
			return sandboxdc.SpecOverlay{}, err
		}

		// Use instanceKey for run dir so all projects in shared mode share one run dir.
		runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), instanceKey)
		if err != nil {
			return sandboxdc.SpecOverlay{}, fmt.Errorf("devcontainer: ensure run dir: %w", err)
		}

		binPath, err := InstallBinaryInRunDir(runDir)
		if err != nil {
			return sandboxdc.SpecOverlay{}, fmt.Errorf("devcontainer: install binary: %w", err)
		}
		if err := InstallSockBridgeInRunDir(runDir); err != nil {
			slog.Warn("devcontainer: sockbridge install failed", "err", err)
		}

		env := buildOverlayEnv(scriptEnv, proxySpec)
		mounts := append([]string{
			fmt.Sprintf("type=bind,source=%s,target=%s", runDir, ContainerRunDir),
		}, proxySpec.Mounts...)

		// codex backend's sockbridge is registered alongside provider
		// bridges so postCreate starts them all in one place.
		bridges := make([]container.BridgeSpec, 0, len(proxySpec.BridgeSpecs)+1)
		bridges = append(bridges, proxySpec.BridgeSpecs...)
		bridges = append(bridges, cstream.ContainerBridgeSpec(ContainerRunDir))
		postCreate := buildPostCreate(binPath, postCreateSubcmds, bridges)

		return sandboxdc.SpecOverlay{
			Env:                     env,
			Mounts:                  mounts,
			ExtraCreateArgs:         dc.ExtraCreateArgs,
			PostCreate:              postCreate,
			WorkspaceFolderFallback: resolveWorkspaceFallback(projectPath, dc.HostPathMountPrefix),
		}, nil
	}
}

// resolveWorkspaceFallback returns the container-side path to use when
// devcontainer.json doesn't specify workspaceFolder or workspaceMount.
// Empty prefix mirrors the host path as-is; non-empty prefix prepends it.
func resolveWorkspaceFallback(projectPath, prefix string) string {
	if prefix == "" {
		return projectPath
	}
	return path.Join(prefix, projectPath)
}

func resolveOverlaySpecs(proxy *CredProxyRunner, projectPath string, dc config.DevcontainerConfig) (container.Spec, map[string]string, error) {
	allow := isProjectEnvScriptAllowed(projectPath, dc.AllowProjectEnvScript)
	scriptEnv := sandboxdc.RunEnvScript(context.Background(), dc.EnvScript, projectPath, allow)

	var proxySpec container.Spec
	if proxy != nil {
		specCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		proxySpec, err = proxy.ContainerSpec(specCtx, projectPath)
		if err != nil {
			slog.Warn("devcontainer: credproxy spec failed", "project", projectPath, "err", err)
		}
	}
	return proxySpec, scriptEnv, nil
}

func buildOverlayEnv(scriptEnv map[string]string, proxySpec container.Spec) map[string]string {
	env := make(map[string]string)
	for k, v := range scriptEnv {
		env[k] = v
	}
	for k, v := range proxySpec.Env {
		env[k] = v
	}
	env["ROOST_SOCKET"] = ContainerSockFilePath
	env["ROOST_DATA_DIR"] = ContainerRunDir
	return env
}

// buildPostCreate assembles a bash -lc postCreateCommand that:
//  1. starts each bridge as a background daemon, and
//  2. runs each postCreateSubcmd in sequence in the foreground.
func buildPostCreate(binPath string, postCreateSubcmds []string, bridges []container.BridgeSpec) []string {
	var parts []string
	for _, bs := range bridges {
		parts = append(parts, fmt.Sprintf("%s -listen %s -socket %s &",
			ContainerSockBridgePath, bs.ListenAddr, bs.ContainerSocketPath))
	}
	for _, sub := range postCreateSubcmds {
		parts = append(parts, binPath+" "+sub)
	}
	if len(parts) == 0 {
		return nil
	}
	return []string{"bash", "-lc", strings.Join(parts, "\n")}
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
