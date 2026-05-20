package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/state"
)

// WrappedLaunch is the resolved launch specification after the launcher
// has applied any sandboxing. The runtime passes Command/StartDir/Env
// directly to TmuxBackend.SpawnWindow; Cleanup is called when the frame
// is destroyed (nil is safe to ignore).
type WrappedLaunch struct {
	Command   string
	StartDir  string
	Env       map[string]string
	Cleanup   func() error
	Subsystem state.LaunchSubsystem
	Stream    state.StreamLaunchOptions
	// ContainerSockDir is set by devcontainer sandbox launchers to the host-side
	// run directory that is bind-mounted into the container as /opt/roost/run.
	// When non-empty, the runtime starts the container endpoint for this project.
	ContainerSockDir string
	// Mounts is the set of bind mounts for the container instance.
	// Used to translate container-absolute paths to host-absolute paths at
	// the IPC boundary. Empty for non-sandbox (DirectLauncher) launches.
	Mounts pathmap.Mounts
}

// AgentLauncher wraps a state.LaunchPlan before it reaches tmux, allowing
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
	// with the sandbox backend (the agent process is already running in tmux).
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
		Command:   plan.Command,
		StartDir:  plan.StartDir,
		Env:       merged,
		Subsystem: plan.Subsystem,
		Stream:    plan.Stream,
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

// launcher returns cfg.Launcher if set, otherwise a zero-cost DirectLauncher.
func launcher(cfg Config) AgentLauncher {
	if cfg.Launcher != nil {
		return cfg.Launcher
	}
	return DirectLauncher{}
}

// wrapLaunchResult holds the output of wrapLaunchForSpawn.
type wrapLaunchResult struct {
	wrapped WrappedLaunch
	// token is non-empty only for container frames. The token string is
	// generated here so it can be baked into the spawn env. Registration
	// (token↔frame) happens on the event loop via internalSpawnComplete.
	token string
}

// wrapLaunchForSpawn calls WrapLaunch and (for container launchers) generates
// a bearer token. It has no side effects on runtime state — all registry
// writes happen on the event loop after the spawn goroutine completes.
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

// generateToken returns a random 32-byte hex-encoded token.
// Pure computation; safe to call from any goroutine.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
