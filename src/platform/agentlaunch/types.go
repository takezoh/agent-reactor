package agentlaunch

import "context"

// LaunchPlan carries the pure launch parameters. ForceHost replaces the
// client/state SandboxOverride == SandboxOverrideHost sentinel.
//
// Argv, when non-nil, holds the structured argv for Spawn (no host-side shell).
// Command is the shell-joined string form used by backend pane launchers.
// Both are populated by per-agent lib builders; callers choose which to use.
type LaunchPlan struct {
	Command   string
	Argv      []string
	Env       map[string]string
	StartDir  string
	Project   string
	ForceHost bool
}

// Mount is a host↔container path pair used to translate paths at the IPC boundary.
type Mount struct {
	Host, Container string
}

// WrappedLaunch is the resolved launch specification after sandboxing has been
// applied. Command/StartDir/Env are handed to the caller's spawn layer (a pane
// backend for the client, a direct stdio exec for the orchestrator);
// Cleanup is called when the launch is torn down.
//
// Argv, when non-nil, is the argv for Spawn (no host-side shell). Command is the
// shell-joined equivalent for backend pane launchers. Dispatchers populate both.
type WrappedLaunch struct {
	Command          string
	Argv             []string
	StartDir         string
	Env              map[string]string
	Cleanup          func(context.Context) error
	ContainerSockDir string
	Mounts           []Mount
}
