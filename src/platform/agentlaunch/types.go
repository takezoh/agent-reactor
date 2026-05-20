package agentlaunch

import "context"

// LaunchPlan carries the pure launch parameters. ForceHost replaces the
// client/state SandboxOverride == SandboxOverrideHost sentinel.
type LaunchPlan struct {
	Command   string
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
// applied. Command/StartDir/Env are passed to TmuxBackend.SpawnWindow;
// Cleanup is called when the frame is destroyed.
type WrappedLaunch struct {
	Command          string
	StartDir         string
	Env              map[string]string
	Cleanup          func(context.Context) error
	ContainerSockDir string
	Mounts           []Mount
}
