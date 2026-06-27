package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-reactor/platform/lib/codex"
	"github.com/takezoh/agent-reactor/platform/procgroup"
)

// FactoryConfig holds runtime-supplied dependencies the Stream Factory needs
// to instantiate Backends. The runtime is responsible for resolving paths
// and capabilities — the Factory itself encapsulates environment knowledge
// (host vs container) within the SubsystemID it constructs.
type FactoryConfig struct {
	// Runtime is the hook used by Backends to enqueue events.
	Runtime RuntimeHook
	// Dispatcher applies sandbox/container wrapping to each app-server launch.
	// Nil falls back to a direct (no-op) dispatch.
	Dispatcher agentlaunch.Dispatcher
	// ResolveSockPath returns the UDS path the per-session app-server binds for
	// the given project. In container mode it is a container-absolute path under
	// ContainerRunDir; the backend derives the host dial path from the launch's
	// bind mounts. Paths are unique per session to allow multiple concurrent
	// app-server processes. project is passed explicitly so that the callback
	// need not access shared mutable state from a goroutine.
	ResolveSockPath func(sessionID state.SessionID, project string) (listen string, err error)
	// IsContainer reports whether the given project runs in a devcontainer.
	IsContainer func(project string) bool
	// ReadTimeout overrides the per-request JSON-RPC timeout.  Zero uses the
	// default (15 seconds).  Corresponds to the codex.read_timeout_ms config key.
	ReadTimeout time.Duration
	// Tracker records the app-server process-group pgids so a future boot can
	// reap them if this daemon dies without a graceful Stop.
	// Nil disables crash-path tracking (e.g. tests, non-Linux).
	Tracker *procgroup.Tracker
}

// Factory creates Stream Backends keyed by session. One Backend (= one
// app-server process) exists per client Session. All frames (root + pushed
// frames) in the same Session share one Backend; different Sessions get
// separate Backends.
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
func (f *Factory) Ensure(ctx context.Context, sessionID state.SessionID, project string, plan state.LaunchPlan) (subsystem.Subsystem, state.SubsystemID, error) {
	argv, err := agentlaunch.SplitArgs(plan.Command)
	if err != nil {
		return nil, "", fmt.Errorf("stream factory: parse command: %w", err)
	}
	cmdCfg, err := libcodex.ParseCommand(argv)
	if err != nil {
		return nil, "", err
	}
	id := f.makeID(sessionID)

	f.mu.Lock()
	if b, ok := f.backends[id]; ok {
		f.mu.Unlock()
		return b, id, nil
	}
	f.mu.Unlock()

	listen, err := f.cfg.ResolveSockPath(sessionID, project)
	if err != nil {
		return nil, "", fmt.Errorf("stream factory: resolve sock path: %w", err)
	}
	b := New(
		f.cfg.Runtime,
		f.cfg.Dispatcher,
		id,
		sessionID,
		project,
		cmdCfg.ServerBin,
		cmdCfg.ServerArgs,
		cmdCfg.Model,
		plan.Stream.SandboxPolicy == state.StreamSandboxPolicyExternal,
		plan.Stream.ApprovalPolicy == state.StreamApprovalPolicyAutoApprove,
		listen,
		f.cfg.ReadTimeout,
	)
	b.tracker = f.cfg.Tracker

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

// Remove implements subsystem.Reaper. It stops the backend for the given
// subsystemID and removes it from the factory. Called when a session's last
// frame is released.
func (f *Factory) Remove(ctx context.Context, id state.SubsystemID) {
	f.mu.Lock()
	b, ok := f.backends[id]
	if ok {
		delete(f.backends, id)
	}
	f.mu.Unlock()
	if ok {
		b.Stop(ctx)
	}
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

// makeID derives the SubsystemID from the session identifier.
// Every client Session gets its own app-server, so the ID is keyed purely on
// sessionID. All frames (root + pushed frames) within the same session share
// the same ID and therefore the same Backend.
func (f *Factory) makeID(sessionID state.SessionID) state.SubsystemID {
	return state.SubsystemID("stream:session:" + string(sessionID))
}
