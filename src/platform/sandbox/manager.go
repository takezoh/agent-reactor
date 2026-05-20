// Package sandbox defines the backend-agnostic interface for project-level
// sandbox lifecycle management (Docker, Firecracker, …).
//
// Each backend creates one long-lived sandbox instance per project directory.
// Frames (tmux windows / docker exec sessions) join by calling AcquireFrame;
// the sandbox is destroyed when the last frame calls ReleaseFrame.
package sandbox

import (
	"context"
)

// FrameID identifies a tmux frame within a sandbox session.
type FrameID string

// LaunchSpec carries the minimal launch parameters that BuildLaunchCommand
// needs. Callers construct it from their own plan types at the boundary.
type LaunchSpec struct {
	Command  string
	StartDir string
}

// Instance represents a running sandbox for one project directory.
// I is the backend-specific internal state type (e.g. *docker.ContainerState).
type Instance[I any] struct {
	ProjectPath string // canonical absolute path
	Image       string // docker image (or equivalent) used to start this instance
	Internal    I
}

// StartOptions carries optional per-launch parameters for starting a new sandbox
// instance. Options are only applied when the instance is freshly created; a
// cached (running) instance ignores them.
type StartOptions struct {
	Env             map[string]string // fixed env vars to set in the container
	ForwardEnv      []string          // host env var names to pass through if set on the host
	DevcontainerDir string            // devcontainer.json directory override; empty = auto-discover
	SharedMode      bool              // use shared container (isolation=shared) instead of per-project
	// ColdStart asks EnsureInstance to discard any existing container before
	// provisioning a fresh one. Set by the coordinator's cold-start path so
	// that a daemon SIGKILL (no graceful DestroyInstance) cannot leak stale
	// in-container daemons (sockbridge, app-server) into the next launch.
	ColdStart bool
}

// FrameContext carries per-frame values the launcher resolves at launch time.
// Keeping these off the container-scoped DevcontainerSpec is what lets a
// shared container host frames from different projects without leaking the
// first frame's project state into every later one.
type FrameContext struct {
	FrameID FrameID           // identifies the frame this command is for
	WorkDir string            // container-side cwd (pathmap で解決済み); empty falls back to spec
	Env     map[string]string // per-frame -e KEY=VAL; wins over spec.RemoteEnv on conflict
	Mounts  []string          // per-frame -v / --mount (将来用、当面空)
}

// Manager is the backend-neutral lifecycle controller for project sandboxes.
// I is the backend-specific internal state type.
// Implementations must be safe for concurrent use from multiple goroutines.
type Manager[I any] interface {
	// EnsureInstance starts the sandbox for the (projectPath, image) pair if not
	// already running, or returns the existing instance. opts only apply when a
	// new instance is created. Concurrent calls for the same (project, image) must
	// be serialized (e.g. via singleflight).
	EnsureInstance(ctx context.Context, projectPath, image string, opts StartOptions) (*Instance[I], error)

	// BuildLaunchCommand generates the shell command string and environment to
	// run spec inside the sandbox instance. frameCtx carries per-frame values
	// (workDir, env) the launcher resolved at launch time. The returned command
	// is passed to TmuxBackend.SpawnWindow.
	BuildLaunchCommand(inst *Instance[I], spec LaunchSpec, frameCtx FrameContext, env map[string]string) (command string, outEnv map[string]string, err error)

	// AcquireFrame increments the ref-count for the instance.
	// Must be called before the frame is spawned.
	AcquireFrame(inst *Instance[I])

	// ReleaseFrame decrements the ref-count. Returns true when the count
	// drops to zero — the caller should then call DestroyInstance.
	ReleaseFrame(inst *Instance[I]) bool

	// DestroyInstance stops and removes the sandbox. Only called when
	// ReleaseFrame returns true (ref-count == 0).
	DestroyInstance(ctx context.Context, inst *Instance[I]) error
}
