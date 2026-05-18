package runtime

import (
	"context"
	"fmt"
	"log/slog"
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
	proxy               *CredProxyRunner // nil when proxy disabled
	dataDir             string
}

// NewDevcontainerLauncher creates an AgentLauncher that runs agents inside devcontainers.
// dataDir is the daemon's data directory (e.g. ~/.roost); it contains the run/ subtree.
// resolveProjectScope returns the raw project-scope SandboxConfig (nil if absent).
func NewDevcontainerLauncher(
	mgr sandbox.Manager[*sandboxdc.ContainerState],
	resolveSandbox func(string) config.SandboxConfig,
	resolveProjectScope func(string) *config.SandboxConfig,
	proxy *CredProxyRunner,
	dataDir string,
) *DevcontainerLauncher {
	return &DevcontainerLauncher{
		mgr:                 mgr,
		resolveSandbox:      resolveSandbox,
		resolveProjectScope: resolveProjectScope,
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

	// Resolve startDir to its container-side path via the pathmap before handing
	// it to BuildLaunchCommand. Pass it through FrameContext so shared containers
	// (which have no canonical project root) get the right `docker exec -w`
	// without the spec needing to know anything about the frame's project.
	workDir := plan.StartDir
	if containerPath, ok := mounts.ToContainer(plan.StartDir); ok {
		workDir = containerPath
	}
	frameCtx, err := l.ResolveFrameContext(ctx, plan.Project, frameID)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: frame ctx: %w", err)
	}
	frameCtx.WorkDir = workDir

	cmd, outEnv, err := l.mgr.BuildLaunchCommand(inst, plan, frameCtx, env)
	if err != nil {
		return WrappedLaunch{}, fmt.Errorf("devcontainer launcher: build command: %w", err)
	}

	return WrappedLaunch{
		Command:          cmd,
		StartDir:         workDir,
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

// RunDirKey returns the run-dir key (instance key) for projectPath, resolving
// shared vs project isolation. Callers outside WrapLaunch use this to align
// the per-frame run directory with the container the launcher will actually
// produce.
func (l *DevcontainerLauncher) RunDirKey(projectPath string) string {
	return l.runDirKey(projectPath, l.resolveStartOptions(projectPath))
}

// StartOptionsFor exposes the resolved StartOptions for projectPath so callers
// that interact with the underlying Manager directly (e.g. stream backend) use
// the same shared/project decision as WrapLaunch.
func (l *DevcontainerLauncher) StartOptionsFor(projectPath string) sandbox.StartOptions {
	return l.resolveStartOptions(projectPath)
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

	return sandbox.StartOptions{
		SharedMode:      true,
		DevcontainerDir: config.ExpandPath(userSandbox.Devcontainer.Path),
	}
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

// BuildContainerOverlay returns the OverlayFunc applied once per container at
// EnsureInstance time. Its output ends up baked into the container-scoped
// DevcontainerSpec, so it must carry only container-scope state: workspace
// bind-mounts, postCreate bridges, container-level env defaults.
//
// Per-frame state (per-project credential / env-script results that vary across
// frames inside a shared container) is resolved later by ResolveFrameContext
// and emitted as docker exec -e at launch time — NOT through this overlay.
//
// effectiveOverlayProject keeps the shared-mode invariant: for SharedContainerKey
// the project is dropped to "" so project-scope sandbox config is not merged.
// For project-mode containers the actual project is used (= existing behavior).
//
// dataDir is the daemon's data directory (e.g. ~/.roost). postCreateSubcmds
// are driver-specific setup commands; the caller injects them so runtime stays
// driver-agnostic. projects supplies the workspace list for shared containers.
func BuildContainerOverlay(resolveSandbox func(string) config.SandboxConfig, projects config.ProjectsConfig, proxy *CredProxyRunner, dataDir string, postCreateSubcmds []string) sandboxdc.OverlayFunc {
	return func(instanceKey, projectPath, dcDir string) (sandboxdc.SpecOverlay, error) {
		// Shared containers run all projects in one image: per-project env,
		// credentials, bridges, and workspace fallback would otherwise be
		// frozen to whichever project triggered the overlay first and leak
		// into every later frame. Resolve everything at user scope instead.
		overlayProject := effectiveOverlayProject(instanceKey, projectPath)
		sb := resolveSandbox(overlayProject)
		dc := sb.Devcontainer

		proxySpec, scriptEnv, err := resolveOverlaySpecs(proxy, overlayProject, dc)
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

// effectiveOverlayProject returns the project context that BuildOverlayFunc
// should hand to env-script, credproxy, and workspace-fallback resolution.
// Project-mode containers use the frame's actual project. Shared containers
// must use "" so the resolved values stay at user scope — otherwise the first
// frame's project pins them onto the shared spec for every later frame.
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

// resolveWorkspaceFallback returns the container-side path to use when
// devcontainer.json doesn't specify workspaceFolder or workspaceMount.
// Empty prefix mirrors the host path as-is; non-empty prefix prepends it.
func resolveWorkspaceFallback(projectPath, prefix string) string {
	if prefix == "" {
		return projectPath
	}
	return path.Join(prefix, projectPath)
}

// ResolveFrameContext computes per-frame env by invoking env-script and
// credproxy for the frame's project at launch time.
//
// In shared mode the project parameter is dropped (user scope only) — project
// scope sandbox config is intentionally not merged when isolation=shared, so
// every shared-container frame sees the same user-scope credential set.
//
// In project mode the actual project path is used so per-project credentials
// (AWS profile, GCP active, hostexec policy, …) resolve correctly. The result
// is fed to BuildLaunchCommand via sandbox.FrameContext.Env and emitted as
// docker exec -e — it never touches the container-scoped DevcontainerSpec.
func (l *DevcontainerLauncher) ResolveFrameContext(ctx context.Context, projectPath string, frameID state.FrameID) (sandbox.FrameContext, error) {
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
		FrameID: frameID,
		Env:     frameScopeEnv(proxySpec.Env),
	}, nil
}

// frameScopeEnv returns the subset of proxy-supplied env that is safe to emit
// as docker exec -e per frame. Container-scope keys (PATH, ROOST_*, anything
// whose value still contains $-placeholders the container resolves at create
// time) MUST be excluded — otherwise docker exec receives both the spec's
// expanded value and the raw placeholder string, and the "last -e wins" rule
// of docker hands the broken raw string to the container (e.g. PATH=...:$PATH
// with $PATH literal, making /bin/bash unreachable).
//
// Per-frame credentials (AWS_PROFILE, GCP_PROJECT, custom OIDC tokens, …)
// flow through unchanged.
func frameScopeEnv(proxyEnv map[string]string) map[string]string {
	out := make(map[string]string, len(proxyEnv))
	for k, v := range proxyEnv {
		if isContainerScopeEnvKey(k) {
			continue
		}
		if strings.Contains(v, "$") {
			// Placeholder is meant for spec.RemoteEnv container-time expansion.
			// Leaving it in frameCtx would emit the raw $… into docker exec -e.
			continue
		}
		out[k] = v
	}
	return out
}

// isContainerScopeEnvKey identifies env keys whose value does not vary across
// frames in the same container. These are owned by BuildContainerOverlay and
// must not be re-emitted from frameCtx (see frameScopeEnv).
func isContainerScopeEnvKey(k string) bool {
	switch k {
	case "PATH", "ROOST_SOCKET", "ROOST_DATA_DIR", "SSH_AUTH_SOCK":
		return true
	}
	return false
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
