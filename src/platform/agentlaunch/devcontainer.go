package agentlaunch

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-roost/platform/config"
	"github.com/takezoh/agent-roost/platform/credproxy"
	"github.com/takezoh/agent-roost/platform/pathmap"
	"github.com/takezoh/agent-roost/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-roost/platform/sandbox/devcontainer"
	"github.com/takezoh/credproxy/container"
)

// DevcontainerLauncher wraps launches inside per-project devcontainers.
// It implements Dispatcher by delegating to a sandbox.Manager[*devcontainer.ContainerState].
type DevcontainerLauncher struct {
	mgr                 sandbox.Manager[*sandboxdc.ContainerState]
	resolveSandbox      func(projectPath string) config.SandboxConfig
	resolveProjectScope func(projectPath string) *config.SandboxConfig
	proxy               *credproxy.Runner // nil when proxy disabled
	dataDir             string
	tty                 bool // allocate a pseudo-TTY (docker exec -it); false for piped/headless consumers
	coldStart           atomic.Bool
}

// BeginColdStart marks the launcher as being in cold-start mode.
func (l *DevcontainerLauncher) BeginColdStart() { l.coldStart.Store(true) }

// EndColdStart clears cold-start mode.
func (l *DevcontainerLauncher) EndColdStart() { l.coldStart.Store(false) }

// NewDevcontainerLauncher creates a Dispatcher that runs agents inside devcontainers.
// tty selects `docker exec -it` (true, for interactive tmux panes) vs `-i`
// (false, for headless consumers that pipe JSON-RPC stdio like the orchestrator).
func NewDevcontainerLauncher(
	mgr sandbox.Manager[*sandboxdc.ContainerState],
	resolveSandbox func(string) config.SandboxConfig,
	resolveProjectScope func(string) *config.SandboxConfig,
	proxy *credproxy.Runner,
	dataDir string,
	tty bool,
) *DevcontainerLauncher {
	return &DevcontainerLauncher{
		mgr:                 mgr,
		resolveSandbox:      resolveSandbox,
		resolveProjectScope: resolveProjectScope,
		proxy:               proxy,
		dataDir:             dataDir,
		tty:                 tty,
	}
}

const containerEnsureTimeout = 120 * time.Second

// Wrap ensures the project devcontainer is running and returns a launch spec
// that runs the agent via "docker exec".
func (l *DevcontainerLauncher) Wrap(ctx context.Context, frameID string, plan LaunchPlan) (WrappedLaunch, error) {
	if plan.Project == "" {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: plan.Project is empty for frame %s", frameID)
	}

	ensureCtx, cancel := context.WithTimeout(ctx, containerEnsureTimeout)
	defer cancel()

	opts := l.resolveStartOptions(plan.Project)
	inst, err := l.mgr.EnsureInstance(ensureCtx, plan.Project, "", opts)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: ensure instance: %w", err)
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
	pm := buildMounts(wsHost, wsContainer, runDir, inst.Internal.BindMounts())

	workDir := plan.StartDir
	if containerPath, ok := pm.ToContainer(plan.StartDir); ok {
		workDir = containerPath
	}
	frameCtx, err := l.ResolveFrameContext(ctx, plan.Project, frameID)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: frame ctx: %w", err)
	}
	frameCtx.WorkDir = workDir

	cmd, outEnv, err := l.mgr.BuildLaunchCommand(inst, sandbox.LaunchSpec{Command: plan.Command, StartDir: plan.StartDir, TTY: l.tty}, frameCtx, plan.Env)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: build command: %w", err)
	}

	// Tokenize the generated docker exec command string into argv for Spawn.
	// BuildLaunchCommand produces well-formed single-quoted tokens that SplitArgs handles correctly.
	argv, err := SplitArgs(cmd)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: tokenize command: %w", err)
	}

	return WrappedLaunch{
		Command:          cmd,
		Argv:             argv,
		StartDir:         workDir,
		Env:              outEnv,
		Cleanup:          l.makeCleanup(frameID, inst),
		ContainerSockDir: runDir,
		Mounts:           toMounts(pm),
	}, nil
}

// runDirKey returns the key for the per-container run directory.
func (l *DevcontainerLauncher) runDirKey(projectPath string, opts sandbox.StartOptions) string {
	if opts.SharedMode {
		return sandboxdc.SharedContainerKey
	}
	return projectPath
}

// RunDirKey returns the run-dir key for projectPath (shared vs project isolation).
func (l *DevcontainerLauncher) RunDirKey(projectPath string) string {
	return l.runDirKey(projectPath, l.resolveStartOptions(projectPath))
}

// StartOptionsFor exposes the resolved StartOptions so callers that interact
// with the underlying Manager use the same shared/project decision as Wrap.
func (l *DevcontainerLauncher) StartOptionsFor(projectPath string) sandbox.StartOptions {
	return l.resolveStartOptions(projectPath)
}

func (l *DevcontainerLauncher) resolveStartOptions(projectPath string) sandbox.StartOptions {
	opts := l.startOptionsForIsolation(projectPath)
	opts.ColdStart = l.coldStart.Load()
	return opts
}

func (l *DevcontainerLauncher) startOptionsForIsolation(projectPath string) sandbox.StartOptions {
	if _, err := sandboxdc.ProjectBaseDC(projectPath); err == nil {
		return sandbox.StartOptions{}
	}

	projScope := l.resolveProjectScope(projectPath)
	if projScope != nil && (projScope.Isolation == "project" || projScope.Devcontainer.Path != "") {
		return sandbox.StartOptions{DevcontainerDir: config.ExpandPath(projScope.Devcontainer.Path)}
	}

	userSandbox := l.resolveSandbox("")
	if userSandbox.Isolation != "shared" {
		return sandbox.StartOptions{}
	}

	return sandbox.StartOptions{
		SharedMode:      true,
		DevcontainerDir: config.ExpandPath(userSandbox.Devcontainer.Path),
	}
}

func (l *DevcontainerLauncher) IsContainer(_ string) bool { return true }

// buildMounts constructs the pathmap.Mounts for a devcontainer instance.
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

// toMounts converts pathmap.Mounts to the pure []Mount type.
func toMounts(pm pathmap.Mounts) []Mount {
	if len(pm) == 0 {
		return nil
	}
	out := make([]Mount, len(pm))
	for i, m := range pm {
		out[i] = Mount{Host: m.Host, Container: m.Container}
	}
	return out
}

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
func (l *DevcontainerLauncher) AdoptFrame(ctx context.Context, frameID, projectPath string) (func(context.Context) error, []Mount, error) {
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
	pm := buildMounts(wsHost, wsContainer, runDir, inst.Internal.BindMounts())
	return l.makeCleanup(frameID, inst), toMounts(pm), nil
}

func (l *DevcontainerLauncher) makeCleanup(frameID string, inst *sandbox.Instance[*sandboxdc.ContainerState]) func(context.Context) error {
	return func(ctx context.Context) error {
		shouldDestroy := l.mgr.ReleaseFrame(inst)
		slog.Debug("devcontainer launcher: frame released",
			"frame", frameID, "project", inst.ProjectPath, "destroy", shouldDestroy)
		if !shouldDestroy {
			return nil
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return l.mgr.DestroyInstance(ctx, inst)
	}
}

// BuildContainerOverlay returns the OverlayFunc applied once per container at
// EnsureInstance time. It bakes container-scope state into the DevcontainerSpec:
// workspace bind-mounts, postCreate bridges, and container-level env defaults.
func BuildContainerOverlay(
	resolveSandbox func(string) config.SandboxConfig,
	projects config.ProjectsConfig,
	proxy *credproxy.Runner,
	dataDir string,
	postCreateSubcmds []string,
) sandboxdc.OverlayFunc {
	return func(instanceKey, projectPath, dcDir string) (sandboxdc.SpecOverlay, error) {
		overlayProject := effectiveOverlayProject(instanceKey, projectPath)
		sb := resolveSandbox(overlayProject)
		dc := sb.Devcontainer

		proxySpec, scriptEnv, err := resolveOverlaySpecs(proxy, overlayProject, dc)
		if err != nil {
			return sandboxdc.SpecOverlay{}, err
		}

		runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), instanceKey)
		if err != nil {
			return sandboxdc.SpecOverlay{}, fmt.Errorf("devcontainer: ensure run dir: %w", err)
		}

		binPath, err := InstallBinaryInRunDir(runDir)
		if err != nil {
			return sandboxdc.SpecOverlay{}, fmt.Errorf("devcontainer: install binary: %w", err)
		}
		env := buildOverlayEnv(scriptEnv, proxySpec)
		mounts := append([]string{
			fmt.Sprintf("type=bind,source=%s,target=%s", runDir, ContainerRunDir),
		}, proxySpec.Mounts...)

		postCreate := buildPostCreate(binPath, postCreateSubcmds, proxySpec.BridgeSpecs, ContainerStreamBridgeCmd(ContainerRunDir))

		var extraWorkspaces []sandboxdc.BindMount
		if instanceKey == sandboxdc.SharedContainerKey {
			extraWorkspaces = sharedWorkspaceBindMounts(projects, dc.HostPathMountPrefix)
		}

		return sandboxdc.SpecOverlay{
			Env:                     env,
			Mounts:                  mounts,
			ExtraWorkspaces:         extraWorkspaces,
			ExtraCreateArgs:         dc.ExtraCreateArgs,
			PostCreate:              postCreate,
			WorkspaceFolderFallback: resolveWorkspaceFallback(overlayProject, dc.HostPathMountPrefix),
		}, nil
	}
}

func effectiveOverlayProject(instanceKey, projectPath string) string {
	if instanceKey == sandboxdc.SharedContainerKey {
		return ""
	}
	return projectPath
}

func sharedWorkspaceBindMounts(projects config.ProjectsConfig, prefix string) []sandboxdc.BindMount {
	seen := map[string]struct{}{}
	var out []sandboxdc.BindMount
	for _, hostPath := range projects.ListProjects() {
		if _, dup := seen[hostPath]; dup {
			continue
		}
		seen[hostPath] = struct{}{}
		out = append(out, sandboxdc.BindMount{
			Source: hostPath,
			Target: resolveWorkspaceFallback(hostPath, prefix),
		})
	}
	return out
}

func resolveWorkspaceFallback(projectPath, prefix string) string {
	if prefix == "" {
		return projectPath
	}
	return path.Join(prefix, projectPath)
}

// ResolveFrameContext computes per-frame env by invoking env-script and credproxy.
// frameID is plain string; callers that hold a typed FrameID should convert with string().
func (l *DevcontainerLauncher) ResolveFrameContext(ctx context.Context, projectPath string, frameID string) (sandbox.FrameContext, error) {
	opts := l.resolveStartOptions(projectPath)
	effectiveProject := projectPath
	if opts.SharedMode {
		effectiveProject = ""
	}
	dc := l.resolveSandbox(effectiveProject).Devcontainer

	proxySpec, _, err := resolveOverlaySpecs(l.proxy, effectiveProject, dc)
	if err != nil {
		return sandbox.FrameContext{}, err
	}
	return sandbox.FrameContext{
		FrameID: sandbox.FrameID(frameID),
		Env:     frameScopeEnv(proxySpec.Env),
	}, nil
}

func frameScopeEnv(proxyEnv map[string]string) map[string]string {
	out := make(map[string]string, len(proxyEnv))
	for k, v := range proxyEnv {
		if isContainerScopeEnvKey(k) {
			continue
		}
		if strings.Contains(v, "$") {
			continue
		}
		out[k] = v
	}
	return out
}

func isContainerScopeEnvKey(k string) bool {
	switch k {
	case "PATH", "ROOST_SOCKET", "ROOST_DATA_DIR", "SSH_AUTH_SOCK":
		return true
	}
	return false
}

func resolveOverlaySpecs(proxy *credproxy.Runner, projectPath string, dc config.DevcontainerConfig) (container.Spec, map[string]string, error) {
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

// buildPostCreate assembles the postCreate shell script for the devcontainer.
// extraBgCmds are run first as background processes (e.g. the stream routing
// bridge). Each BridgeSpec from credproxy providers is started via
// "roost-bridge sockbridge" in fixed-socket mode. postCreateSubcmds are run
// via the installed roost-bridge binary (setup hooks etc.).
func buildPostCreate(binPath string, postCreateSubcmds []string, bridges []container.BridgeSpec, extraBgCmds ...string) []string {
	var parts []string
	parts = append(parts, extraBgCmds...)
	for _, bs := range bridges {
		parts = append(parts, fmt.Sprintf("%s sockbridge -listen %s -socket %s &",
			ContainerBinaryPath, bs.ListenAddr, bs.ContainerSocketPath))
	}
	for _, sub := range postCreateSubcmds {
		parts = append(parts, binPath+" "+sub)
	}
	if len(parts) == 0 {
		return nil
	}
	return []string{"bash", "-lc", strings.Join(parts, "\n")}
}

func isProjectEnvScriptAllowed(projectPath string, allowlist []string) bool {
	for _, p := range allowlist {
		if config.ExpandPath(p) == projectPath {
			return true
		}
	}
	return false
}

// ContainerExecInfo holds the docker-exec parameters needed to run a command
// inside the project's devcontainer. See GetContainerExecInfo method.
type ContainerExecInfo struct {
	ContainerID string
	User        string
	WorkDir     string
	PreExec     string
}

// GetContainerExecInfo ensures the container for projectPath is running and
// returns the parameters needed for docker exec. Used by the stream backend.
func (l *DevcontainerLauncher) GetContainerExecInfo(ctx context.Context, projectPath string) (*ContainerExecInfo, error) {
	opts := l.resolveStartOptions(projectPath)
	inst, err := l.mgr.EnsureInstance(ctx, projectPath, "", opts)
	if err != nil {
		return nil, err
	}
	cs := inst.Internal
	return &ContainerExecInfo{
		ContainerID: cs.ContainerID(),
		User:        cs.EffectiveUser(),
		WorkDir:     cs.WorkspaceTarget(),
		PreExec:     cs.PreExec(),
	}, nil
}
