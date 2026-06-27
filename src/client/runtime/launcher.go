package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/pathmap"
)

// WrappedLaunch is the resolved launch specification after the launcher
// has applied any sandboxing. The runtime passes Command/StartDir/Env
// directly to PaneBackend.SpawnWindow; Cleanup is called when the frame
// is destroyed (nil is safe to ignore).
type WrappedLaunch struct {
	Command  string
	StartDir string
	Env      map[string]string
	Cleanup  func() error
	// ContainerSockDir is set by devcontainer sandbox launchers to the host-side
	// run directory that is bind-mounted into the container as /opt/agent-reactor/run.
	// When non-empty, the runtime starts the container endpoint for this project.
	ContainerSockDir string
	// Mounts is the set of bind mounts for the container instance.
	// Used to translate container-absolute paths to host-absolute paths at
	// the IPC boundary. Empty for non-sandbox (DirectLauncher) launches.
	Mounts pathmap.Mounts
}

// AgentLauncher wraps a state.LaunchPlan before it reaches the pane backend, allowing
// sandbox implementations to prepend wrapper commands or spin up isolated
// environments. The runtime calls WrapLaunch once per spawn;
// DirectLauncher is used when no Launcher is configured.
//
// Sandbox cleanup is handled via state.EffReleaseFrameSandboxes, not through
// any Shutdown method. The Launcher is responsible only for per-frame wrap
// and adopt; the runtime interpreter drains frame cleanups on shutdown.
type AgentLauncher interface {
	WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error)

	// AdoptFrame is called during warm start to re-register a pre-existing frame
	// with the sandbox backend (the agent process is already running in a pane).
	// Returns the Cleanup callback and the bind-mount map for the frame (may be
	// nil for non-sandbox backends). Must not start or restart the sandbox.
	AdoptFrame(ctx context.Context, frameID state.FrameID, projectPath string) (func() error, pathmap.Mounts, error)

	// EnsureProject prepares the sandbox environment for a project without
	// allocating a frame. No-op for non-sandbox launchers.
	EnsureProject(ctx context.Context, projectPath string) error

	// IsContainer reports whether the given project will be run inside a
	// container by this launcher. The runtime uses this to decide whether to
	// inject ROOST_SOCKET_TOKEN before calling WrapLaunch.
	IsContainer(project string) bool
}

// ColdStartAware は cold-start 区間中の sandbox 再構築を sandbox-bearing な
// launcher だけが知る optional capability。coordinator.coldStart が
// BeginColdStart / EndColdStart を defer 越しに呼び、その区間内の
// EnsureProject / WrapLaunch は pre-existing container を破棄して新規
// provisioning を行う。capability を持たない launcher (DirectLauncher 等)
// は実装不要 ― 型 assertion 経由でしか呼ばれない。
type ColdStartAware interface {
	BeginColdStart()
	EndColdStart()
}

// DirectLauncher is the no-op implementation: it passes the plan through
// unchanged so behaviour is identical to the pre-launcher code path.
// SockPath, when non-empty, is injected as ROOST_SOCKET so hook subprocesses
// can reach the daemon without relying on baked-in or fallback paths.
type DirectLauncher struct {
	SockPath string
}

func (d DirectLauncher) WrapLaunch(_ state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	merged := stripContainerOnlyEnv(env)
	if d.SockPath != "" {
		merged = cloneAndSet(merged, "ROOST_SOCKET", d.SockPath)
	}
	return WrappedLaunch{
		Command:  plan.Command,
		StartDir: plan.StartDir,
		Env:      merged,
	}, nil
}

func (DirectLauncher) AdoptFrame(_ context.Context, _ state.FrameID, _ string) (func() error, pathmap.Mounts, error) {
	return nil, nil, nil
}

func (DirectLauncher) EnsureProject(_ context.Context, _ string) error { return nil }

func (DirectLauncher) IsContainer(_ string) bool { return false }

func (DirectLauncher) BeginColdStart() {}
func (DirectLauncher) EndColdStart()   {}

// stripContainerOnlyEnv returns a copy of env without ROOST_SOCKET_TOKEN.
// Token injection is only valid inside containers; DirectLauncher drops it
// so host processes are never given a container credential.
func stripContainerOnlyEnv(env map[string]string) map[string]string {
	out := cloneEnvMap(env, 0)
	delete(out, "ROOST_SOCKET_TOKEN")
	return out
}

func cloneAndSet(env map[string]string, key, value string) map[string]string {
	out := cloneEnvMap(env, 1)
	out[key] = value
	return out
}

// dispatcherAdapter translates between AgentLauncher (client state types) and
// agentlaunch.Dispatcher (pure platform types).
type dispatcherAdapter struct {
	d agentlaunch.Dispatcher
}

func (a dispatcherAdapter) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	pp := agentlaunch.LaunchPlan{
		Command:   plan.Command,
		Env:       env,
		StartDir:  plan.StartDir,
		Project:   plan.Project,
		ForceHost: plan.Sandbox == state.SandboxOverrideHost,
	}
	w, err := a.d.Wrap(context.Background(), string(frameID), pp)
	if err != nil {
		return WrappedLaunch{}, err
	}
	return WrappedLaunch{
		Command:          w.Command,
		StartDir:         w.StartDir,
		Env:              w.Env,
		Cleanup:          adaptCleanup(w.Cleanup),
		ContainerSockDir: w.ContainerSockDir,
		Mounts:           toPathmapMounts(w.Mounts),
	}, nil
}

func (a dispatcherAdapter) AdoptFrame(ctx context.Context, frameID state.FrameID, projectPath string) (func() error, pathmap.Mounts, error) {
	cleanupFn, mounts, err := a.d.AdoptFrame(ctx, string(frameID), projectPath)
	if err != nil {
		return nil, nil, err
	}
	return adaptCleanup(cleanupFn), toPathmapMounts(mounts), nil
}

func (a dispatcherAdapter) EnsureProject(ctx context.Context, projectPath string) error {
	return a.d.EnsureProject(ctx, projectPath)
}

func (a dispatcherAdapter) IsContainer(project string) bool {
	return a.d.IsContainer(project)
}

func (a dispatcherAdapter) BeginColdStart() {
	if cs, ok := a.d.(agentlaunch.ColdStartAware); ok {
		cs.BeginColdStart()
	}
}

func (a dispatcherAdapter) EndColdStart() {
	if cs, ok := a.d.(agentlaunch.ColdStartAware); ok {
		cs.EndColdStart()
	}
}

// NewDispatcherAdapter wraps an agentlaunch.Dispatcher for use as AgentLauncher.
func NewDispatcherAdapter(d agentlaunch.Dispatcher) AgentLauncher {
	return dispatcherAdapter{d: d}
}

// adaptCleanup converts func(context.Context) error → func() error by
// supplying a 30-second timeout context.
func adaptCleanup(f func(context.Context) error) func() error {
	if f == nil {
		return nil
	}
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return f(ctx)
	}
}

// toPathmapMounts converts []agentlaunch.Mount → pathmap.Mounts.
func toPathmapMounts(ms []agentlaunch.Mount) pathmap.Mounts {
	if len(ms) == 0 {
		return nil
	}
	out := make(pathmap.Mounts, len(ms))
	for i, m := range ms {
		out[i] = pathmap.Mount{Host: m.Host, Container: m.Container}
	}
	return out
}

// launcher returns cfg.Launcher if set, otherwise a zero-cost DirectLauncher.
func launcher(cfg Config) AgentLauncher {
	if cfg.Launcher != nil {
		return cfg.Launcher
	}
	return DirectLauncher{}
}

// devcontainerLauncherFor extracts the *agentlaunch.DevcontainerLauncher from l,
// handling both a bare dispatcherAdapter (wrapping agentlaunch.SandboxDispatcher)
// and a legacy *SandboxDispatcher or *DevcontainerLauncher.
func devcontainerLauncherFor(l AgentLauncher) *agentlaunch.DevcontainerLauncher {
	if a, ok := l.(dispatcherAdapter); ok {
		return agentlaunch.DevcontainerLauncherFor(a.d)
	}
	return nil
}

// wrapLaunchResult holds the output of wrapLaunchForSpawn.
type wrapLaunchResult struct {
	wrapped WrappedLaunch
	// token is non-empty only for container frames. The token string is
	// generated here so it can be baked into the spawn env. Registration
	// (token↔frame) happens on the event loop via internalSpawnComplete, so
	// no runtime state is mutated from the spawn goroutine.
	token string
}

// wrapLaunchForSpawn calls WrapLaunch and (for container launchers) generates a
// bearer token to inject as ROOST_SOCKET_TOKEN. It has no side effects on
// runtime state — token/mount registration and endpoint startup happen on the
// event loop after the spawn goroutine completes (see registerContainerFrame).
// For non-container launchers the token is never generated.
func wrapLaunchForSpawn(l AgentLauncher, frameID state.FrameID, project string, plan state.LaunchPlan, baseEnv map[string]string) (wrapLaunchResult, error) {
	if !l.IsContainer(project) {
		wrapped, err := l.WrapLaunch(frameID, plan, baseEnv)
		return wrapLaunchResult{wrapped: wrapped}, err
	}

	token, err := generateToken()
	if err != nil {
		return wrapLaunchResult{}, fmt.Errorf("token generate: %w", err)
	}
	env := cloneAndSet(baseEnv, "ROOST_SOCKET_TOKEN", token)

	wrapped, err := l.WrapLaunch(frameID, plan, env)
	if err != nil {
		return wrapLaunchResult{}, fmt.Errorf("launcher wrap: %w", err)
	}
	return wrapLaunchResult{wrapped: wrapped, token: token}, nil
}

// generateToken returns a random 32-byte hex-encoded bearer token.
// Pure computation; safe to call from any goroutine.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
