// Package subsystem defines the Subsystem interface that every runtime
// execution backend (cli, stream) implements. Subsystems own goroutines,
// I/O, and per-frame lifecycle — the runtime calls into them via this
// interface only.
package subsystem

import (
	"context"

	"github.com/takezoh/agent-reactor/client/state"
)

// BindRequest carries the frame-level launch context to Subsystem.BindFrame.
type BindRequest struct {
	FrameID state.FrameID
	Plan    state.LaunchPlan // command/startDir/options/stream/...
	Stdin   []byte
	Project string
}

// BindResult is the resolved frame binding returned by BindFrame.
type BindResult struct {
	// Plan is the updated LaunchPlan; command/startDir may be rewritten
	// (e.g. worktree path substituted, stream resume command built).
	Plan state.LaunchPlan
	// ExtraEnv is merged into the frame's environment before sandbox wrap.
	ExtraEnv map[string]string
	// WorktreeStartDir is non-empty when the subsystem created a managed
	// worktree. The runtime enqueues DEvWorktreeResolved via EvPaneSpawned
	// so the driver persists the path for cold-start reconstruction.
	WorktreeStartDir string
	// WorktreeName is the petname chosen for the managed worktree.
	WorktreeName string
}

// Subsystem is the execution backend interface. Each subsystem kind
// (cli, stream) provides one Backend per (project × sandbox) pair.
// Implementations are in runtime/subsystem/cli and runtime/subsystem/stream.
type Subsystem interface {
	// Kind returns the subsystem kind name.
	Kind() state.LaunchSubsystem

	// Start prepares shared backend resources (e.g. stream subsystem starts
	// the app-server and dials the WebSocket; cli subsystem is a no-op).
	// Called once before the first BindFrame.
	Start(ctx context.Context) error

	// BindFrame is called synchronously inside spawnPaneWindowAsync
	// (already a goroutine) before the pane spawn. It resolves the
	// LaunchPlan (worktree creation, stream thread binding, command
	// rewrite) and registers the frame for cleanup tracking.
	// BindFrame must complete before pane spawn happens.
	BindFrame(ctx context.Context, req BindRequest) (BindResult, error)

	// ReleaseFrame is called when a pane dies or a frame is explicitly
	// stopped. The subsystem removes the frame from its registry and
	// fires any frame-specific cleanup (worktree removal, thread teardown).
	ReleaseFrame(frameID state.FrameID)

	// Stop is called at daemon shutdown. It waits for all in-flight
	// cleanup to finish and terminates backend processes (if any).
	Stop(ctx context.Context)
}

// Factory creates or returns the Subsystem instance for a frame's execution
// context. Implementations encapsulate environment resolution (host /
// container) — Runtime, Driver, and Frame remain environment-agnostic.
// Each LaunchSubsystem kind is backed by one Factory registered at Runtime
// construction time.
type Factory interface {
	// Ensure returns the Subsystem and its SubsystemID for (sessionID, project, plan).
	// Idempotent: same sessionID returns the same Subsystem instance so that all
	// frames in one session (root + peers) share one backend.
	// ctx is used for backend startup (e.g. Stream's app-server dial).
	// The returned SubsystemID is opaque to callers — only the factory
	// (and the Subsystem it manages) knows the encoding.
	Ensure(ctx context.Context, sessionID state.SessionID, project string, plan state.LaunchPlan) (Subsystem, state.SubsystemID, error)
}

// Reaper is an optional interface a Factory may implement to support
// early backend termination when a session's last frame is released.
// The runtime calls Remove when the ref-count for a subsystemID drops to zero.
type Reaper interface {
	Remove(ctx context.Context, id state.SubsystemID)
}
