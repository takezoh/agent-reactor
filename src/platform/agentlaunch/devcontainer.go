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

	"github.com/takezoh/agent-reactor/platform/config"
	"github.com/takezoh/agent-reactor/platform/credproxy"
	"github.com/takezoh/agent-reactor/platform/mcpproxy"
	"github.com/takezoh/agent-reactor/platform/pathmap"
	"github.com/takezoh/agent-reactor/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-reactor/platform/sandbox/devcontainer"
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
// tty selects `docker exec -it` (true, for interactive backend panes) vs `-i`
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
	slog.Debug("devcontainer launcher: frame acquired", "frame", frameID, "project", plan.Project, "shared", opts.Isolation.IsShared())

	wsHost, wsContainer := opts.Isolation.FrameWorkspaceMount(plan.Project, inst.Internal.WorkspaceTarget())
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

	// cmd is a shell command string: BuildLaunchCommand shell-quotes the user,
	// workdir and env, and (for PreExec / shell launches) wraps the agent command
	// in `sh -c 'exec <login-shell> -lc ...'`, whose login shell is an in-container
	// $(...) substitution. Only a real shell can parse that — SplitArgs is not a
	// shell lexer: it cannot reverse the '\'' single-quote escaping nor evaluate the
	// substitution, and silently splits the agent command off into stray tokens that
	// `sh -c` then ignores (the app-server never launches). Run cmd through `sh -c`
	// so the same shell parsing the interactive (backend pane) consumer relies on applies.
	return WrappedLaunch{
		Command:          cmd,
		Argv:             []string{"sh", "-c", cmd},
		StartDir:         workDir,
		Env:              outEnv,
		Cleanup:          l.makeCleanup(frameID, inst),
		ContainerSockDir: runDir,
		Mounts:           toMounts(pm),
	}, nil
}

// runDirKey returns the key for the per-container run directory.
func (l *DevcontainerLauncher) runDirKey(projectPath string, opts sandbox.StartOptions) string {
	return opts.Isolation.ContainerKey(projectPath)
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

// startOptionsForIsolation gathers the I/O-bound inputs (project-own devcontainer
// stat, scope resolution, ~-expansion) and delegates the precedence decision to
// the pure DecideIsolation. The shell/decision split keeps the decision ladder
// table-testable without a filesystem.
func (l *DevcontainerLauncher) startOptionsForIsolation(projectPath string) sandbox.StartOptions {
	var in IsolationInputs
	// A project's own .devcontainer wins outright, so the scope inputs are only
	// gathered (and their I/O paid) when there is no project-own devcontainer.
	if _, err := sandboxdc.ProjectBaseDC(projectPath); err == nil {
		in.HasOwnDevcontainer = true
	} else {
		in.UserScope = l.resolveSandbox("")
		in.UserDevcontainerDir = config.ExpandPath(in.UserScope.Devcontainer.Path)
		in.ProjectScope = l.resolveProjectScope(projectPath)
		if in.ProjectScope != nil {
			in.ProjectDevcontainerDir = config.ExpandPath(in.ProjectScope.Devcontainer.Path)
		}
	}
	return sandbox.StartOptions{Isolation: DecideIsolation(in)}
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
	slog.Debug("devcontainer launcher: frame adopted (warm start)", "frame", frameID, "project", projectPath, "shared", opts.Isolation.IsShared())

	runDir := ProjectRunDir(filepath.Join(l.dataDir, "run"), l.runDirKey(projectPath, opts))
	wsHost, wsContainer := opts.Isolation.FrameWorkspaceMount(projectPath, inst.Internal.WorkspaceTarget())
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
	return func(plan sandbox.IsolationPlan, projectPath, dcDir string) (sandboxdc.SpecOverlay, error) {
		overlayProject := plan.OverlayProject(projectPath)
		sb := resolveSandbox(overlayProject)
		dc := sb.Devcontainer

		proxySpec, scriptEnv, err := resolveOverlaySpecs(proxy, overlayProject, dc)
		if err != nil {
			return sandboxdc.SpecOverlay{}, err
		}

		runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), plan.ContainerKey(projectPath))
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

		postCreate := buildPostCreate(binPath, postCreateSubcmds, proxySpec.BridgeSpecs)

		var extraWorkspaces []sandboxdc.BindMount
		if plan.IsShared() {
			extraWorkspaces = sharedWorkspaceBindMounts(projects, dc.HostPathMountPrefix)
		}

		return sandboxdc.SpecOverlay{
			Env:                     env,
			Mounts:                  mounts,
			ExtraWorkspaces:         extraWorkspaces,
			ExtraCreateArgs:         dc.ExtraCreateArgs,
			PostCreate:              postCreate,
			WorkspaceFolderFallback: resolveWorkspaceFallback(plan.WorkspaceFallbackProject(projectPath), dc.HostPathMountPrefix),
		}, nil
	}
}

// Overlay project, container key, and workspace-fallback project are derived
// from IsolationPlan — see platform/sandbox/isolation.go.

func sharedWorkspaceBindMounts(projects config.ProjectsConfig, prefix string) []sandboxdc.BindMount {
	// Dedup on the resolved container target, not the host source: docker rejects
	// duplicate mount targets, and two host paths that differ only by a trailing
	// slash (or otherwise collapse under path.Join with a prefix) resolve to the
	// same target. Keying on the target also keeps the derived MCP overlay files
	// (mcp-<hash(target)>.json) from colliding. First host path wins.
	seen := map[string]struct{}{}
	var out []sandboxdc.BindMount
	for _, hostPath := range projects.ListProjects() {
		target := resolveWorkspaceFallback(hostPath, prefix)
		if _, dup := seen[target]; dup {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, sandboxdc.BindMount{Source: hostPath, Target: target})
	}
	return out
}

func resolveWorkspaceFallback(projectPath, prefix string) string {
	if prefix == "" {
		return projectPath
	}
	return path.Join(prefix, projectPath)
}

// BuildProviderHooks returns the credproxy.ProviderHooks that resolve a project
// key — a real project path, or the shared-container sentinel — to the container
// workspace paths the hostexec and MCP providers overlay into. This is the
// devcontainer orchestration knowledge (shared vs project, the project list,
// HostPathMountPrefix) that credproxy must not own; credproxy only threads these
// functions into each provider's Config.
func BuildProviderHooks(resolveSandbox func(string) config.SandboxConfig, projects config.ProjectsConfig) credproxy.ProviderHooks {
	return credproxy.ProviderHooks{
		HostExecWorkspaceFolder: func(projectKey string) string {
			return resolveWorkspaceFallback(projectKey, resolveSandbox(projectKey).Devcontainer.HostPathMountPrefix)
		},
		MCPWorkspaceTargets: func(projectKey string) []mcpproxy.WorkspaceTarget {
			prefix := resolveSandbox(projectKey).Devcontainer.HostPathMountPrefix
			if !sandbox.IsSharedKey(projectKey) {
				return []mcpproxy.WorkspaceTarget{{HostRoot: projectKey, ContainerWS: resolveWorkspaceFallback(projectKey, prefix)}}
			}
			return sharedMCPTargets(projects, prefix)
		},
	}
}

// sharedMCPTargets maps every bound project's workspace bind mount to an MCP
// overlay target, so each project in a shared container gets its own .mcp.json
// at its container workspace root. Reusing sharedWorkspaceBindMounts keeps the
// overlay targets identical to the workspace bind mounts (and shares its dedup).
func sharedMCPTargets(projects config.ProjectsConfig, prefix string) []mcpproxy.WorkspaceTarget {
	binds := sharedWorkspaceBindMounts(projects, prefix)
	out := make([]mcpproxy.WorkspaceTarget, 0, len(binds))
	for _, b := range binds {
		out = append(out, mcpproxy.WorkspaceTarget{HostRoot: b.Source, ContainerWS: b.Target})
	}
	return out
}

// ResolveFrameContext computes per-frame env by invoking env-script and credproxy.
// frameID is plain string; callers that hold a typed FrameID should convert with string().
func (l *DevcontainerLauncher) ResolveFrameContext(ctx context.Context, projectPath string, frameID string) (sandbox.FrameContext, error) {
	opts := l.resolveStartOptions(projectPath)
	effectiveProject := opts.Isolation.OverlayProject(projectPath)
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
// Each BridgeSpec from credproxy providers is started as a background process
// via "reactor-bridge sockbridge" in fixed-socket mode. postCreateSubcmds are run
// via the installed reactor-bridge binary (setup hooks etc.).
//
// `set -e` makes a foreground command failure abort the script with a
// non-zero exit so devcontainer up surfaces the error instead of silently
// reporting success. Background `&` commands (sockbridge) are unaffected:
// bash detaches them and does not propagate their exit status through
// set -e. Without this, e.g. a failing `claude-setup-hooks` (read-only
// ~/.claude, malformed pre-existing JSON) was masked by the subsequent
// `gemini-setup-hooks` succeeding — Claude hooks went silently missing
// while the container provisioning reported OK.
func buildPostCreate(binPath string, postCreateSubcmds []string, bridges []container.BridgeSpec) []string {
	parts := []string{"set -e"}
	for _, bs := range bridges {
		parts = append(parts, fmt.Sprintf("%s sockbridge -listen %s -socket %s &",
			ContainerBinaryPath, bs.ListenAddr, bs.ContainerSocketPath))
	}
	for _, sub := range postCreateSubcmds {
		parts = append(parts, binPath+" "+sub)
	}
	if len(parts) == 1 { // only "set -e" → nothing to do
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
