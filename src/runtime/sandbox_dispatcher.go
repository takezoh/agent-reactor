package runtime

import (
	"context"
	"fmt"

	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/state"
)

// SandboxDispatcher implements AgentLauncher by selecting the correct backend
// (direct or devcontainer) based on the effective sandbox mode for each project.
// The mode is resolved per call via a SandboxResolver so project-scope
// overrides are applied without restarting the daemon.
type SandboxDispatcher struct {
	Resolver     *config.SandboxResolver
	Direct       AgentLauncher
	Devcontainer *DevcontainerLauncher // nil when devcontainer backend is not configured
}

// WrapLaunch resolves the effective sandbox mode for plan.Project and
// delegates to the appropriate backend launcher.
func (d *SandboxDispatcher) WrapLaunch(frameID state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	mode := d.Resolver.Resolve(plan.Project).Mode
	switch mode {
	case "devcontainer":
		if d.Devcontainer == nil {
			return WrappedLaunch{}, fmt.Errorf("sandbox dispatcher: devcontainer mode for %q but devcontainer backend unavailable", plan.Project)
		}
		return d.Devcontainer.WrapLaunch(frameID, plan, env)
	case "", "direct":
		return d.Direct.WrapLaunch(frameID, plan, env)
	default:
		return WrappedLaunch{}, fmt.Errorf("sandbox dispatcher: unknown mode %q for project %q", mode, plan.Project)
	}
}

// AdoptFrame resolves the effective sandbox mode for projectPath and delegates
// to the appropriate backend to reclaim the pre-running sandbox frame.
func (d *SandboxDispatcher) AdoptFrame(ctx context.Context, frameID state.FrameID, projectPath string) (func() error, pathmap.Mounts, error) {
	mode := d.Resolver.Resolve(projectPath).Mode
	switch mode {
	case "devcontainer":
		if d.Devcontainer == nil {
			return nil, nil, nil
		}
		return d.Devcontainer.AdoptFrame(ctx, frameID, projectPath)
	case "", "direct":
		return d.Direct.AdoptFrame(ctx, frameID, projectPath)
	default:
		return nil, nil, fmt.Errorf("sandbox dispatcher: unknown mode %q for project %q", mode, projectPath)
	}
}
