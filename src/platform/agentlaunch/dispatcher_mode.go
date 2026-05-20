package agentlaunch

import (
	"context"
	"fmt"

	platformconfig "github.com/takezoh/agent-roost/platform/config"
)

// SandboxDispatcher implements Dispatcher by selecting the correct backend
// (direct or devcontainer) based on the effective sandbox mode for each project.
type SandboxDispatcher struct {
	Resolver     *platformconfig.SandboxResolver
	Direct       Dispatcher
	Devcontainer *DevcontainerLauncher // nil when devcontainer backend is not configured
}

// Wrap resolves the effective sandbox mode for plan.Project and delegates.
func (d *SandboxDispatcher) Wrap(ctx context.Context, frameID string, plan LaunchPlan) (WrappedLaunch, error) {
	if plan.ForceHost {
		return d.Direct.Wrap(ctx, frameID, plan)
	}
	mode := d.Resolver.Resolve(plan.Project).Mode
	switch mode {
	case "devcontainer":
		if d.Devcontainer == nil {
			return WrappedLaunch{}, fmt.Errorf("sandbox dispatcher: devcontainer mode for %q but devcontainer backend unavailable", plan.Project)
		}
		return d.Devcontainer.Wrap(ctx, frameID, plan)
	case "", "direct":
		return d.Direct.Wrap(ctx, frameID, plan)
	default:
		return WrappedLaunch{}, fmt.Errorf("sandbox dispatcher: unknown mode %q for project %q", mode, plan.Project)
	}
}

// EnsureProject warms up the container for projectPath without allocating a frame.
func (d *SandboxDispatcher) EnsureProject(ctx context.Context, projectPath string) error {
	mode := d.Resolver.Resolve(projectPath).Mode
	switch mode {
	case "devcontainer":
		if d.Devcontainer == nil {
			return nil
		}
		return d.Devcontainer.EnsureProject(ctx, projectPath)
	case "", "direct":
		return d.Direct.EnsureProject(ctx, projectPath)
	default:
		return fmt.Errorf("sandbox dispatcher: unknown mode %q for project %q", mode, projectPath)
	}
}

// IsContainer reports whether projectPath will run inside a container.
func (d *SandboxDispatcher) IsContainer(projectPath string) bool {
	if d.Devcontainer == nil {
		return false
	}
	return d.Resolver.Resolve(projectPath).Mode == "devcontainer"
}

// BeginColdStart / EndColdStart forward to every backend that supports it.
func (d *SandboxDispatcher) BeginColdStart() {
	if d.Devcontainer != nil {
		d.Devcontainer.BeginColdStart()
	}
	if cs, ok := d.Direct.(ColdStartAware); ok {
		cs.BeginColdStart()
	}
}

func (d *SandboxDispatcher) EndColdStart() {
	if d.Devcontainer != nil {
		d.Devcontainer.EndColdStart()
	}
	if cs, ok := d.Direct.(ColdStartAware); ok {
		cs.EndColdStart()
	}
}

// AdoptFrame reclaims a pre-running sandbox frame.
func (d *SandboxDispatcher) AdoptFrame(ctx context.Context, frameID, projectPath string) (func(context.Context) error, []Mount, error) {
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

// DevcontainerLauncherFor extracts the *DevcontainerLauncher from a Dispatcher,
// handling both a bare *DevcontainerLauncher and a *SandboxDispatcher wrapper.
// Returns nil if d has no devcontainer backend.
func DevcontainerLauncherFor(d Dispatcher) *DevcontainerLauncher {
	switch v := d.(type) {
	case *DevcontainerLauncher:
		return v
	case *SandboxDispatcher:
		return v.Devcontainer
	}
	return nil
}
