package agentlaunch

import "context"

// Dispatcher wraps a LaunchPlan before it reaches tmux, applying any sandbox
// or container logic. Implementations must be safe for concurrent use.
type Dispatcher interface {
	// Wrap applies sandbox logic to plan and returns the resolved launch spec.
	Wrap(ctx context.Context, frameID string, plan LaunchPlan) (WrappedLaunch, error)

	// AdoptFrame re-registers a pre-existing frame during warm start without
	// starting or restarting the sandbox.
	AdoptFrame(ctx context.Context, frameID, projectPath string) (func(context.Context) error, []Mount, error)

	// EnsureProject warms up the sandbox environment for a project without
	// allocating a frame. No-op for non-sandbox dispatchers.
	EnsureProject(ctx context.Context, projectPath string) error

	// IsContainer reports whether the given project runs inside a container.
	IsContainer(projectPath string) bool
}

// ColdStartAware is an optional capability for dispatchers that manage
// persistent sandbox state (e.g. devcontainer). The coordinator's cold-start
// path calls BeginColdStart / EndColdStart around its provisioning window so
// any pre-existing container is discarded and reprovisioned cleanly.
type ColdStartAware interface {
	BeginColdStart()
	EndColdStart()
}
