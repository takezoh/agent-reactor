// Package runtime is the imperative shell for the pure state package.
// It owns the single event loop goroutine, the worker pool, the IPC
// server, the fsnotify watcher, and the pane backend. Every state
// mutation goes through state.Reduce; every side effect is dispatched
// through the Effect interpreter in interpret.go.
//
// The event loop is the only goroutine that touches Runtime.state.
// Workers, IPC readers, and the fsnotify watcher feed events back via
// channels — they never read or write state directly.
package runtime

import (
	"context"
	"log/slog"
	"net"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/runtime/framereg"
	rsubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem"
	clisubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem/cli"
	cstream "github.com/takezoh/agent-reactor/client/runtime/subsystem/stream"
	"github.com/takezoh/agent-reactor/client/runtime/worker"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/features"
	"github.com/takezoh/agent-reactor/platform/procgroup"
)

// sameSessionMap returns true when the two maps refer to the same
// underlying map header. Reducers that intend to mutate sessions go
// through cloneSessions which allocates a new map, so identity check
// is sufficient to skip persistence work on events that did not touch
// sessions. Pure pointer comparison on Go maps isn't allowed, hence
// the reflect.Value.Pointer detour.
func sameSessionMap(a, b map[state.SessionID]state.Session) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// Config carries the runtime's startup parameters. Backends are
// injected (interfaces) so tests can swap fakes.
type Config struct {
	DataDir      string
	TickInterval time.Duration
	Workers      int

	Backend  PaneBackend
	Persist  PersistBackend
	EventLog EventLogBackend
	ToolLog  ToolLogBackend
	Watcher  FSWatcher
	Pool     *worker.Pool

	// TerminalEvict is called with the pane target string whenever a session
	// pane is unregistered. It should release the VT emulator held for that
	// pane to prevent unbounded memory growth. May be nil.
	TerminalEvict func(pane string)

	// Tap, if non-nil, is used to attach a raw byte stream reader to each
	// frame's pane. The reader feeds bytes into a VT emulator that fires
	// callbacks for OSC 9/99/777 notifications, OSC 133 prompt events, and
	// OSC 0/2 window titles; each callback enqueues the corresponding event.
	Tap PaneTap

	// Features is the set of runtime flags built from the config file.
	// Injected into state.State once at construction; never mutated.
	Features features.Set

	// Launcher wraps agent launch plans before they reach the backend, enabling
	// sandbox implementations. nil falls back to DirectLauncher (no-op).
	Launcher AgentLauncher

	// StreamDispatcher applies sandbox/container wrapping for codex app-server
	// launches (stdio, non-TTY). Separate from Launcher which uses TTY=true for
	// interactive backend panes. nil falls back to direct (no-op) dispatch.
	StreamDispatcher agentlaunch.Dispatcher

	// StreamReadTimeout overrides the codex app-server JSON-RPC read timeout.
	// Zero uses the 15 s default in codexclient.NewConn.
	// Maps to the codex.read_timeout_ms config key.
	StreamReadTimeout time.Duration
}

// Runtime owns the event loop goroutine and the side-effect backends.
// All fields are read/written from the event loop goroutine alone
// except where noted.
type Runtime struct {
	cfg Config

	state state.State

	// sessionPanes maps each FrameID to its pane id ("%5", "%12", ...).
	sessionPanes map[state.FrameID]string
	eventCh      chan state.Event   // public events from any goroutine
	internalCh   chan internalEvent // runtime-internal lifecycle (conn open/close)

	workers *worker.Pool

	relay *FileRelay

	listener net.Listener
	conns    map[state.ConnID]*ipcConn // owned by event loop
	nextConn state.ConnID              // owned by event loop

	done chan struct{}

	// shutdownMu guards shutdownAck. RequestShutdown lazily creates the
	// ack channel and the EffReleaseFrameSandboxes handler closes it via
	// ackShutdown so the signal-handler goroutine can wait for the drain
	// to finish before cancelling the runtime context.
	shutdownMu  sync.Mutex
	shutdownAck chan struct{}

	taps *tapManager

	// tickN is a monotonic counter incremented on each main tick, passed
	// as EvTick.N so reducers and drivers can gate work to every N-th tick.
	tickN uint64
	// workspaceResolver resolves the workspace name for each session's
	// project directory, with mtime-based caching of .agent-reactor/settings.toml.
	workspaceResolver *config.WorkspaceResolver

	// sandboxCleanups holds WrappedLaunch.Cleanup callbacks keyed by FrameID.
	// A plain loop-owned map: spawn goroutines no longer write it directly —
	// handleSpawnComplete stores the cleanup on the event loop.
	sandboxCleanups map[state.FrameID]func() error

	// frameReg maps container bearer tokens and bind-mount tables to frame IDs.
	// The event loop is the sole writer (via registerContainerFrame /
	// invokeFrameCleanup); container endpoint handlers read concurrently. Its
	// internal RWMutex is the only lock in the runtime root call path.
	frameReg *framereg.Registry
	// containerEndpoints holds one *containerEndpoint per project path.
	// Plain loop-owned map: started lazily on the event loop.
	containerEndpoints map[string]*containerEndpoint

	// warmFrames persists warm-only per-frame state (container tokens) to
	// <dataDir>/warm/ so they survive daemon warm restarts.
	warmFrames *warmFrameStore

	// subsystems holds every live Subsystem instance keyed by its opaque
	// SubsystemID. Subsystems are environment-aware (host vs container);
	// the runtime keeps them as opaque values and routes lifecycle calls
	// uniformly. Plain loop-owned map: handleSpawnComplete is the writer.
	subsystems map[state.SubsystemID]rsubsystem.Subsystem
	// subsystemFactories is the static dispatch table from LaunchSubsystem
	// kind to the Factory that constructs Backends for that kind. Populated
	// once in New; never mutated thereafter.
	subsystemFactories map[state.LaunchSubsystem]rsubsystem.Factory
	// frameSubsystems tracks which subsystem owns each live frame,
	// keyed by FrameID → subsystem.Subsystem. Used to route ReleaseFrame.
	frameSubsystems map[state.FrameID]rsubsystem.Subsystem
	// frameSubsystemIDs maps each live frame to its backend's SubsystemID.
	// Read in executeKillSessionWindow to reap the backend when a session's
	// last frame is released.
	frameSubsystemIDs map[state.FrameID]state.SubsystemID

	// baseCtx is the long-lived daemon context used as the parent for
	// subsystem goroutines and spawned process groups, so daemon shutdown
	// cascades into every backend. Set by SetBaseContext before cold-start
	// spawns (which run before Run) and defaulted in Run for the warm path.
	baseCtx context.Context

	// pgidTracker records host process-group pgids (codex app-server,
	// sockbridge) so PruneProcessGroups can reap them after a crash that
	// skipped graceful Stop. Nil when no data dir is configured.
	pgidTracker *procgroup.Tracker

	// terminalRelay fans pane output from TerminalRelay to subscribed ConnIDs.
	// Nil when cfg.Backend does not implement SurfaceBackend.
	terminalRelay *TerminalRelay

	// internalDrops counts per-event-type drops from enqueueInternal so a future
	// saturation incident can be attributed to a specific producer. Snapshot via
	// InternalDropStats.
	internalDrops *internalDropCounter
}

// PruneProcessGroups reaps host process groups left marked by an earlier daemon
// boot that died without a graceful Stop (SIGKILL/panic). Call once at startup
// before spawning. No-op without a data dir or on non-Linux platforms.
func (r *Runtime) PruneProcessGroups() { r.pgidTracker.Prune() }

// InternalDropStats returns the running counts of silently-dropped internal
// events keyed by short event-type label (matches internalEventName). Only
// non-zero buckets are returned; callers can treat absence as "0 drops". Safe
// to call from any goroutine.
func (r *Runtime) InternalDropStats() map[string]uint64 {
	if r.internalDrops == nil {
		return map[string]uint64{}
	}
	return r.internalDrops.snapshot()
}

// SetBaseContext stores the long-lived daemon context. It must be called
// before cold-start spawning so subsystems created during RecreateAll inherit
// the daemon's cancellation. Run also sets it for callers that start directly.
func (r *Runtime) SetBaseContext(ctx context.Context) { r.baseCtx = ctx }

// baseContext returns the daemon context, or context.Background() if one was
// never set (e.g. unit tests that exercise spawn helpers without a daemon).
func (r *Runtime) baseContext() context.Context {
	if r.baseCtx != nil {
		return r.baseCtx
	}
	return context.Background()
}

// New constructs a Runtime ready for Run. Backends must be set on the
// Config; missing backends are stubbed with no-ops at construction so
// the loop can start even if the caller has not wired everything yet
// (useful for incremental tests).
func applyConfigDefaults(cfg Config) Config {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = time.Second
	}
	if cfg.Backend == nil {
		cfg.Backend = noopBackend{}
	}
	if cfg.Persist == nil {
		cfg.Persist = noopPersist{}
	}
	if cfg.EventLog == nil {
		cfg.EventLog = noopEventLog{}
	}
	if cfg.Watcher == nil {
		cfg.Watcher = noopWatcher{}
	}
	if cfg.ToolLog == nil {
		cfg.ToolLog = noopToolLog{}
	}
	return cfg
}

func New(cfg Config) *Runtime {
	cfg = applyConfigDefaults(cfg)
	initial := state.New()
	initial.Features = cfg.Features
	r := &Runtime{
		cfg:                cfg,
		state:              initial,
		sessionPanes:       map[state.FrameID]string{},
		eventCh:            make(chan state.Event, 256),
		internalCh:         make(chan internalEvent, 64),
		conns:              map[state.ConnID]*ipcConn{},
		done:               make(chan struct{}),
		workspaceResolver:  config.NewWorkspaceResolver(),
		sandboxCleanups:    map[state.FrameID]func() error{},
		frameReg:           framereg.New(),
		containerEndpoints: map[string]*containerEndpoint{},
		subsystems:         map[state.SubsystemID]rsubsystem.Subsystem{},
		frameSubsystems:    map[state.FrameID]rsubsystem.Subsystem{},
		frameSubsystemIDs:  map[state.FrameID]state.SubsystemID{},
		internalDrops:      newInternalDropCounter(),
	}
	if cfg.Pool != nil {
		r.workers = cfg.Pool
	} else {
		r.workers = worker.NewPool(context.Background(), cfg.Workers)
	}
	if cfg.DataDir != "" {
		if wf, err := newWarmFrameStore(cfg.DataDir); err != nil {
			slog.Warn("runtime: warm frame store init failed", "err", err)
		} else {
			r.warmFrames = wf
		}
		r.pgidTracker = &procgroup.Tracker{
			Dir:   filepath.Join(cfg.DataDir, "run", "pgids"),
			Nonce: procgroup.NewBootNonce(),
		}
	}
	r.registerSubsystemFactories()
	if sb, ok := cfg.Backend.(SurfaceBackend); ok {
		r.terminalRelay = NewTerminalRelay(sb, r.enqueueInternal)
	}
	return r
}

// registerSubsystemFactories is the one place that knows which factories
// back each kind; adding a new subsystem only requires extending this map.
func (r *Runtime) registerSubsystemFactories() {
	r.subsystemFactories = map[state.LaunchSubsystem]rsubsystem.Factory{
		state.LaunchSubsystemCLI: clisubsystem.NewFactory(),
		state.LaunchSubsystemStream: cstream.NewFactory(cstream.FactoryConfig{
			Runtime:         r,
			Dispatcher:      r.cfg.StreamDispatcher,
			ResolveSockPath: r.resolveStreamListenPath,
			IsContainer:     func(project string) bool { return launcher(r.cfg).IsContainer(project) },
			ReadTimeout:     r.cfg.StreamReadTimeout,
			Tracker:         r.pgidTracker,
		}),
	}
}

// Done signals when Run has fully exited.
func (r *Runtime) Done() <-chan struct{} { return r.done }

// Launcher returns the resolved AgentLauncher (cfg.Launcher or DirectLauncher).
func (r *Runtime) Launcher() AgentLauncher { return launcher(r.cfg) }

// CleanupSubsystems removes untracked managed worktrees after cold start.
// Tracked paths come from driver state rather than per-subsystem caches so the
// result is correct regardless of which subsystem backends are live at cleanup time.
func (r *Runtime) CleanupSubsystems(ctx context.Context) {
	rsubsystem.CleanupUntracked(ctx, r.KnownProjects(), collectTrackedWorktrees(r.state))
}

func collectTrackedWorktrees(s state.State) map[string]struct{} {
	tracked := make(map[string]struct{})
	for _, sess := range s.Sessions {
		for _, fr := range sess.Frames {
			drv := state.GetDriver(fr.Command)
			sda, ok := drv.(state.StartDirAware)
			if !ok {
				continue
			}
			if dir := sda.StartDir(fr.Driver); dir != "" && rsubsystem.IsManagedWorktreePath(dir) {
				tracked[dir] = struct{}{}
			}
		}
	}
	return tracked
}

// KnownProjects returns the canonical project paths for all sessions currently
// loaded in state. Must be called before Run starts (or from the event loop).
func (r *Runtime) KnownProjects() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, sess := range r.state.Sessions {
		if sess.Project != "" {
			if _, ok := seen[sess.Project]; !ok {
				seen[sess.Project] = struct{}{}
				out = append(out, sess.Project)
			}
		}
	}
	return out
}

// Enqueue submits an event into the loop from outside. The runtime
// itself uses the same channel from inside the loop for self-events.
// Safe to call from any goroutine.
func (r *Runtime) Enqueue(ev state.Event) {
	select {
	case r.eventCh <- ev:
	default:
		slog.Warn("runtime: event channel full, dropping", "type", eventTypeName(ev))
	}
}

// RequestShutdown enqueues the EventShutdown command and blocks until the
// event loop has drained sandbox cleanups (= EffReleaseFrameSandboxes
// handler has finished docker rm on every per-frame closure). The caller
// is expected to cancel the runtime context after this returns — that is
// what shuts the event loop down. Times out after timeout, in which case
// it returns to the caller without an ack (cancel still proceeds).
//
// Safe to call from any goroutine. Calling twice — e.g. SIGTERM arriving
// before the first drain completes — reuses the same ack channel, so the
// second caller waits for the same drain and returns when it finishes.
//
// The send into eventCh is a blocking select rather than the non-blocking
// Enqueue helper. Enqueue silently drops on a full channel by design —
// fine for tick/spawn-style events the runtime will retry on the next
// tick, but unacceptable for shutdown: a dropped shutdown event leaves
// every container running until the next daemon crash recovers it. The
// blocking send is bounded by timeout so a wedged event loop still
// returns control to the signal handler.
func (r *Runtime) RequestShutdown(timeout time.Duration) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	r.shutdownMu.Lock()
	firstCall := r.shutdownAck == nil
	if firstCall {
		r.shutdownAck = make(chan struct{})
	}
	ack := r.shutdownAck
	r.shutdownMu.Unlock()

	if firstCall {
		select {
		case r.eventCh <- state.EvEvent{Event: state.EventShutdown}:
		case <-deadline.C:
			slog.Warn("runtime: shutdown enqueue timed out", "timeout", timeout)
			// Release any concurrent waiters on this ack channel and
			// clear the slot so a future call can re-arm with a fresh
			// enqueue attempt. Without this a second caller would
			// silently park on an ack that nothing closes.
			r.shutdownMu.Lock()
			if r.shutdownAck == ack {
				r.shutdownAck = nil
			}
			r.shutdownMu.Unlock()
			defer func() { _ = recover() }() // idempotent close
			close(ack)
			return
		}
	}
	select {
	case <-ack:
	case <-deadline.C:
		slog.Warn("runtime: shutdown drain timed out", "timeout", timeout)
	}
}

// ackShutdown is called from the EffReleaseFrameSandboxes handler after
// drainFrameCleanups completes. Safe to call when no shutdown was
// requested (the ack channel is nil) and safe to call more than once
// (close is guarded).
func (r *Runtime) ackShutdown() {
	r.shutdownMu.Lock()
	ack := r.shutdownAck
	r.shutdownMu.Unlock()
	if ack == nil {
		return
	}
	defer func() { _ = recover() }() // idempotent close
	close(ack)
}

// SetRelay registers a FileRelay with the runtime via the event loop.
func (r *Runtime) SetRelay(fr *FileRelay) {
	r.internalCh <- internalSetRelay{relay: fr}
}

// StartTapsForRestoredFrames attaches a pane tap to each frame that was
// restored from the snapshot.  Normal sessions route through
// EvPaneSpawned → EffRegisterPane → tapManager.start, but bootstrap
// paths (warm restart, cold-start RecreateAll) populate sessionPanes
// directly without emitting that effect, leaving restored frames
// without a tap.  Call once from the coordinator (see cmd/server/coordinator.go
// runAndWait) immediately after Run has been started, so internalCh is
// empty and the bootstrap event cannot be silently dropped by the
// enqueueInternal non-blocking send.
func (r *Runtime) StartTapsForRestoredFrames() {
	_ = r.enqueueInternal(internalStartRestoredTaps{})
}

// Run is the event loop. It blocks until ctx is cancelled.
//
// Internal events (connOpen, connClose) bypass state.Reduce and go
// straight to dispatchInternal — they manipulate runtime fields the
// reducer can't see (the conns map, the next conn id counter).
func (r *Runtime) Run(ctx context.Context) error {
	if r.baseCtx == nil {
		r.baseCtx = ctx
	}
	defer close(r.done)
	defer r.workers.Stop()
	defer r.shutdownIPC()
	if r.terminalRelay != nil {
		defer r.terminalRelay.Close()
	}
	defer r.cfg.EventLog.CloseAll()
	defer r.cfg.ToolLog.CloseAll()
	// Final persist flush. The dispatch loop already writes per-delta,
	// but signal-driven shutdown (SIGINT/SIGTERM) cancels ctx and exits
	// the loop without giving any reducer a chance to run a closing
	// pass. This defer guarantees the on-disk snapshot reflects the
	// last in-memory state regardless of termination path. SIGKILL /
	// panic still skips defers — that gap is owned by the next
	// cold-start's PruneOrphans.
	defer r.finalPersistFlush()
	// Sandbox resources are released via state.EffReleaseFrameSandboxes on
	// explicit shutdown. On daemon crash (SIGKILL) or panic, the defer stack
	// does not run; next startup's PruneOrphans recovers orphaned containers.

	r.taps = newTapManager(ctx, r.cfg.Tap)
	defer r.taps.stopAll()

	ticker := time.NewTicker(r.cfg.TickInterval)
	defer ticker.Stop()

	slog.Info("runtime: event loop started",
		"tick", r.cfg.TickInterval,
		"workers", r.cfg.Workers)

	for {
		select {
		case <-ctx.Done():
			slog.Info("runtime: event loop stopping (ctx done)")
			return ctx.Err()

		case ev, ok := <-r.eventCh:
			if !ok {
				return nil
			}
			r.dispatch(ev)

		case iev := <-r.internalCh:
			r.dispatchInternal(iev)

		case t := <-ticker.C:
			r.tickN++
			r.dispatch(state.EvTick{Now: t, PaneTargets: r.snapshotPaneTargets(), N: r.tickN})

		case res := <-r.workers.Results():
			r.dispatch(res)

		case fsev := <-r.cfg.Watcher.Events():
			r.dispatch(state.EvFileChanged{
				FrameID: fsev.FrameID,
				Path:    fsev.Path,
			})
		}
	}
}

// dispatch runs Reduce against the current state and executes every
// resulting effect. Effects may enqueue more events into r.eventCh
// (e.g. pane spawn → EvPaneSpawned), which are picked up on
// subsequent loop iterations.
//
// After Reduce returns, dispatch reconciles persistence with the
// state delta: sessions present in prev but absent in next are
// Deleted from disk; the remaining set is Saved (upsert). This makes
// persistence a runtime-level invariant rather than something each
// reducer must remember to emit via EffPersistSnapshot. See the
// "sessions.json が終了時の snapshot になっていない" investigation.
func (r *Runtime) dispatch(ev state.Event) {
	prev := r.state.Sessions
	next, effects := state.Reduce(r.state, ev)
	r.state = next
	for _, eff := range effects {
		r.execute(eff)
	}
	r.reconcilePersist(prev, r.state.Sessions, eventName(ev))
}

// eventName returns a short identifier for an Event, used in diagnostic
// logs that need to attribute a state mutation back to its trigger.
func eventName(ev state.Event) string {
	switch e := ev.(type) {
	case state.EvEvent:
		return "evt:" + e.Event
	case state.EvDriverEvent:
		return "drvev:" + e.Event
	case state.EvSubsystem:
		return "subsys:" + string(e.FrameID)
	case state.EvPaneSpawned:
		return "pane-spawned:" + string(e.FrameID)
	case state.EvSpawnFailed:
		return "spawn-failed:" + string(e.FrameID)
	case state.EvPaneWindowVanished:
		return "pane-vanished:" + string(e.FrameID)
	case state.EvFrameCommandExited:
		return "cmd-exit:" + string(e.FrameID)
	case state.EvTick:
		return "tick"
	case state.EvJobResult:
		return "job-result"
	case state.EvFileChanged:
		return "file-changed:" + string(e.FrameID)
	case state.EvPaneOsc:
		return "pane-osc:" + string(e.FrameID)
	case state.EvPanePrompt:
		return "pane-prompt:" + string(e.FrameID)
	default:
		return ""
	}
}

// finalPersistFlush writes the current snapshot one last time on
// event-loop exit. Idempotent with the per-dispatch reconcilePersist,
// but covers the window between an unpersisted state mutation and
// ctx.Done() — see T3 in regression_persist_test.go.
func (r *Runtime) finalPersistFlush() {
	if len(r.state.Sessions) == 0 {
		return
	}
	if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
		slog.Error("runtime: final persist flush failed", "err", err)
	}
}

// reconcilePersist writes the session-level delta produced by the
// last reduce step to disk. Map identity is the change signal:
// reducers that mutate sessions go through cloneSessions, which
// returns a new map header; reducers that don't touch sessions leave
// the reference intact and incur zero I/O here.
func (r *Runtime) reconcilePersist(prev, next map[state.SessionID]state.Session, evName string) {
	if sameSessionMap(prev, next) {
		return
	}
	var added, removed []string
	for id := range next {
		if _, had := prev[id]; !had {
			added = append(added, string(id))
		}
	}
	for id := range prev {
		if _, kept := next[id]; !kept {
			removed = append(removed, string(id))
			if err := r.cfg.Persist.Delete(string(id)); err != nil {
				slog.Warn("runtime: persist delete failed", "id", id, "err", err)
			}
		}
	}
	if len(added) > 0 || len(removed) > 0 {
		slog.Info("runtime: session delta", "event", evName, "added", added, "removed", removed, "now", len(next))
	}
	if len(next) == 0 {
		return
	}
	if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
		slog.Error("runtime: persist save failed", "err", err)
	}
}

// snapshotPaneTargets returns a copy of sessionPanes for inclusion in
// EvTick so reducers can forward pane targets to drivers without
// accessing the runtime directly.
func (r *Runtime) snapshotPaneTargets() map[state.FrameID]string {
	if len(r.state.Sessions) == 0 {
		return nil
	}
	out := make(map[state.FrameID]string, len(r.sessionPanes))
	for _, sess := range r.state.Sessions {
		for _, frame := range sess.Frames {
			if pane := r.subsystemPaneForFrame(frame); pane != "" {
				out[frame.ID] = pane
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// sessionPaneForSession returns the pane target for the active frame of the
// given session. Returns "" if the session has no registered pane.
func (r *Runtime) sessionPaneForSession(sid state.SessionID) string {
	sess, ok := r.state.Sessions[sid]
	if !ok {
		return ""
	}
	frame, ok := sessionActiveFrame(sess)
	if !ok {
		return ""
	}
	return r.subsystemPaneForFrame(frame)
}

func (r *Runtime) subsystemPaneForFrame(frame state.SessionFrame) string {
	return r.sessionPanes[frame.ID]
}

// HelperBinaryPath resolves a helper binary (e.g. "sockbridge") using the
// canonical exe-adjacent + libexec search implemented in runtime/rundir.go.
func (r *Runtime) HelperBinaryPath(name string) (string, error) {
	return findHelperBinary(name)
}
