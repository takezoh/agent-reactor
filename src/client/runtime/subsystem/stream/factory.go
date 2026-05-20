package stream

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
)

// FactoryConfig holds runtime-supplied dependencies the Stream Factory needs
// to instantiate Backends. The runtime is responsible for resolving paths
// and capabilities — the Factory itself encapsulates environment knowledge
// (host vs container) within the SubsystemID it constructs.
type FactoryConfig struct {
	// Runtime is the hook used by Backends to enqueue events and resolve
	// container exec config.
	Runtime RuntimeHook
	// ResolveSockPaths returns the host-side sock path and the container-side
	// sock path for the given project. The container path equals the host
	// path when the project runs directly on the host.
	ResolveSockPaths func(project string) (host string, container string, err error)
	// IsContainer reports whether the given project runs in a devcontainer.
	IsContainer func(project string) bool
	// RunDirKey returns the container key the project shares (the same key
	// DevcontainerLauncher uses for its run dir): "__shared__" for shared
	// isolation, the project path for project isolation, "" when not
	// containerized. Used in makeID so all frames living in one container
	// reduce to a single subsystem instance.
	RunDirKey func(project string) string
	// ActiveFrameID returns the currently active FrameID; used by Backends
	// to route events to the foreground frame.
	ActiveFrameID func() state.FrameID
	// ReadTimeout overrides the per-request JSON-RPC timeout.  Zero uses the
	// default (15 seconds).  Corresponds to the codex.read_timeout_ms config key.
	ReadTimeout time.Duration
}

// Factory creates Stream Backends keyed by (sandbox mode × project). Sandbox
// mode resolution is internal to the Factory — Runtime, Driver, and Frame
// never see it.
type Factory struct {
	cfg      FactoryConfig
	mu       sync.Mutex
	backends map[state.SubsystemID]*Backend
}

// NewFactory constructs a Stream Factory.
func NewFactory(cfg FactoryConfig) *Factory {
	return &Factory{cfg: cfg, backends: make(map[state.SubsystemID]*Backend)}
}

// Ensure implements subsystem.Factory.
func (f *Factory) Ensure(ctx context.Context, project string, plan state.LaunchPlan) (subsystem.Subsystem, state.SubsystemID, error) {
	cmdCfg, err := ParseCommand(plan.Command)
	if err != nil {
		return nil, "", err
	}
	id := f.makeID(project, plan.Sandbox)

	f.mu.Lock()
	if b, ok := f.backends[id]; ok {
		f.mu.Unlock()
		return b, id, nil
	}
	f.mu.Unlock()

	host, container, err := f.cfg.ResolveSockPaths(project)
	if err != nil {
		return nil, "", fmt.Errorf("stream factory: resolve sock paths: %w", err)
	}
	b := New(
		f.cfg.Runtime,
		id,
		project,
		cmdCfg.ServerBin,
		cmdCfg.ServerArgs,
		cmdCfg.Model,
		plan.Stream.SandboxPolicy == state.StreamSandboxPolicyExternal,
		plan.Stream.ApprovalPolicy == state.StreamApprovalPolicyAutoApprove,
		host,
		container,
		LoopbackPort,
		f.cfg.ActiveFrameID,
		f.cfg.ReadTimeout,
	)

	f.mu.Lock()
	if existing, ok := f.backends[id]; ok {
		f.mu.Unlock()
		return existing, id, nil
	}
	f.backends[id] = b
	f.mu.Unlock()

	if err := b.Start(ctx); err != nil {
		f.mu.Lock()
		delete(f.backends, id)
		f.mu.Unlock()
		return nil, "", err
	}
	return b, id, nil
}

// Range iterates all live backends. Used by the runtime for shutdown.
func (f *Factory) Range(fn func(*Backend) bool) {
	f.mu.Lock()
	snapshot := make([]*Backend, 0, len(f.backends))
	for _, b := range f.backends {
		snapshot = append(snapshot, b)
	}
	f.mu.Unlock()
	for _, b := range snapshot {
		if !fn(b) {
			return
		}
	}
}

// makeID derives the SubsystemID from (sandbox kind, container key).
//
// Container-mode IDs key on the container the project lives in, not the
// project itself: every frame inside one container shares one stream backend
// because they share one in-container sockbridge listening on 127.0.0.1:8282.
// In shared isolation that collapses N projects onto a single ID
// (stream:container:__shared__); in project isolation each project still gets
// its own container and thus its own ID (stream:container:<projectPath>).
//
// Host-mode IDs stay per-project — each host project runs its own codex
// app-server in its own cwd, so collapsing them would mix unrelated threads.
//
// SandboxOverrideHost wins over IsContainer: it's the per-frame "use the host"
// escape hatch even when the project would otherwise containerize.
func (f *Factory) makeID(project string, sandbox state.SandboxOverride) state.SubsystemID {
	escapedProject := strings.ReplaceAll(project, ":", "_")
	if sandbox == state.SandboxOverrideHost {
		return state.SubsystemID("stream:host:" + escapedProject)
	}
	if f.cfg.IsContainer != nil && f.cfg.IsContainer(project) {
		key := project
		if f.cfg.RunDirKey != nil {
			key = f.cfg.RunDirKey(project)
		}
		return state.SubsystemID("stream:container:" + strings.ReplaceAll(key, ":", "_"))
	}
	return state.SubsystemID("stream:host:" + escapedProject)
}
