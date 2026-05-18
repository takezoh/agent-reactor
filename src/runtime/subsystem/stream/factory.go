package stream

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/takezoh/agent-roost/runtime/subsystem"
	"github.com/takezoh/agent-roost/state"
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
	// ActiveFrameID returns the currently active FrameID; used by Backends
	// to route events to the foreground frame.
	ActiveFrameID func() state.FrameID
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

// makeID encodes (kind, sandbox mode, project) into an opaque SubsystemID.
// Sandbox override is resolved here so it does not leak outside this package.
func (f *Factory) makeID(project string, sandbox state.SandboxOverride) state.SubsystemID {
	mode := "auto"
	if sandbox == state.SandboxOverrideHost {
		mode = "host"
	}
	return state.SubsystemID("stream:" + mode + ":" + strings.ReplaceAll(project, ":", "_"))
}
