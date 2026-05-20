// Package runtime is the imperative shell for the pure state package.
// It owns the single event loop goroutine, the worker pool, the IPC
// server, the fsnotify watcher, and the tmux backend. Every state
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
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-roost/client/config"
	rsubsystem "github.com/takezoh/agent-roost/client/runtime/subsystem"
	clisubsystem "github.com/takezoh/agent-roost/client/runtime/subsystem/cli"
	cstream "github.com/takezoh/agent-roost/client/runtime/subsystem/stream"
	"github.com/takezoh/agent-roost/client/runtime/worker"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/features"
	"github.com/takezoh/agent-roost/platform/pathmap"
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
	SessionName       string
	RoostExe          string
	DataDir           string
	TickInterval      time.Duration
	FastTickInterval  time.Duration // default 100ms; fast-detects active-frame pane death.
	Workers           int
	MainPaneHeightPct int

	Tmux     TmuxBackend
	Persist  PersistBackend
	EventLog EventLogBackend
	ToolLog  ToolLogBackend
	Watcher  FSWatcher
	Pool     *worker.Pool
	Notifier Notifier

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

	// Launcher wraps agent launch plans before they reach tmux, enabling
	// sandbox implementations. nil falls back to DirectLauncher (no-op).
	Launcher AgentLauncher
}

// Runtime owns the event loop goroutine and the side-effect backends.
// All fields are read/written from the event loop goroutine alone
// except where noted.
type Runtime struct {
	cfg Config

	state state.State

	// sessionPanes maps each FrameID to its tmux pane id ("%5", "%12", ...).
	sessionPanes map[state.FrameID]string
	// mainPaneSession is the SessionID whose frame is currently in pane 0.1,
	// or "". Distinct from state.ActiveSession (logical focus): this tracks
	// the physical occupant only.
	mainPaneSession state.SessionID
	activeFrameID   state.FrameID
	eventCh         chan state.Event   // public events from any goroutine
	internalCh      chan internalEvent // runtime-internal lifecycle (conn open/close)

	workers *worker.Pool

	relay *FileRelay

	listener net.Listener
	conns    map[state.ConnID]*ipcConn // owned by event loop
	nextConn state.ConnID              // owned by event loop

	done chan struct{}

	taps *tapManager

	// fastProbeInFlight guards against spawning multiple concurrent
	// PaneAlive probes from the fastTicker. Written from any goroutine,
	// read from the event loop.
	fastProbeInFlight atomic.Bool

	// tickN is a monotonic counter incremented on each main tick, passed
	// as EvTick.N so reducers and drivers can gate work to every N-th tick.
	tickN uint64
	// workspaceResolver resolves the workspace name for each session's
	// project directory, with mtime-based caching of .roost/settings.toml.
	workspaceResolver *config.WorkspaceResolver

	// sandboxCleanups holds WrappedLaunch.Cleanup callbacks keyed by FrameID.
	// Protected by sandboxCleanupsMu because storeFrameCleanup is called from
	// goroutines (spawnTmuxWindowAsync) while invoke/drain run in the event loop.
	sandboxCleanupsMu sync.Mutex
	sandboxCleanups   map[state.FrameID]func() error

	// containerTokens holds per-frame bearer tokens for the container endpoint.
	containerTokens tokenStore
	// containerEndpoints holds one *containerEndpoint per project path.
	// Access via sync.Map to allow concurrent startup from spawn goroutines.
	containerEndpoints sync.Map // string (project path) → *containerEndpoint
	// containerMounts holds the pathmap.Mounts for each frame that was launched
	// inside a devcontainer. Used by the container endpoint to translate
	// container-absolute paths to host-absolute paths in hook payloads.
	containerMounts sync.Map // state.FrameID → pathmap.Mounts

	// warmFrames persists warm-only per-frame state (container tokens) to
	// <dataDir>/warm/ so they survive daemon warm restarts.
	warmFrames *warmFrameStore

	// subsystems holds every live Subsystem instance keyed by its opaque
	// SubsystemID. Subsystems are environment-aware (host vs container);
	// the runtime keeps them as opaque values and routes lifecycle calls
	// uniformly. Access via sync.Map because spawnTmuxWindowAsync runs in
	// goroutines.
	subsystems sync.Map // state.SubsystemID → rsubsystem.Subsystem
	// subsystemFactories is the static dispatch table from LaunchSubsystem
	// kind to the Factory that constructs Backends for that kind. Populated
	// once in New; never mutated thereafter.
	subsystemFactories map[state.LaunchSubsystem]rsubsystem.Factory
	// frameSubsystems tracks which subsystem owns each live frame,
	// keyed by FrameID → subsystem.Subsystem. Used to route ReleaseFrame.
	frameSubsystems sync.Map // state.FrameID → subsystem.Subsystem
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
	if cfg.FastTickInterval <= 0 {
		cfg.FastTickInterval = 100 * time.Millisecond
	}
	if cfg.MainPaneHeightPct <= 0 {
		cfg.MainPaneHeightPct = 70
	}
	if cfg.Tmux == nil {
		cfg.Tmux = noopTmux{}
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
	if cfg.Notifier == nil {
		cfg.Notifier = noopNotifier{}
	}
	return cfg
}

func New(cfg Config) *Runtime {
	cfg = applyConfigDefaults(cfg)
	initial := state.New()
	initial.Features = cfg.Features
	r := &Runtime{
		cfg:               cfg,
		state:             initial,
		sessionPanes:      map[state.FrameID]string{},
		eventCh:           make(chan state.Event, 256),
		internalCh:        make(chan internalEvent, 64),
		conns:             map[state.ConnID]*ipcConn{},
		done:              make(chan struct{}),
		workspaceResolver: config.NewWorkspaceResolver(),
		sandboxCleanups:   map[state.FrameID]func() error{},
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
	}
	r.registerSubsystemFactories()
	return r
}

// registerSubsystemFactories is the one place that knows which factories
// back each kind; adding a new subsystem only requires extending this map.
func (r *Runtime) registerSubsystemFactories() {
	r.subsystemFactories = map[state.LaunchSubsystem]rsubsystem.Factory{
		state.LaunchSubsystemCLI: clisubsystem.NewFactory(),
		state.LaunchSubsystemStream: cstream.NewFactory(cstream.FactoryConfig{
			Runtime:          r,
			ResolveSockPaths: r.resolveStreamSockPaths,
			IsContainer:      func(project string) bool { return launcher(r.cfg).IsContainer(project) },
			RunDirKey:        r.streamRunDirKey,
			ActiveFrameID:    func() state.FrameID { return r.activeFrameID },
		}),
	}
}

// Done signals when Run has fully exited.
func (r *Runtime) Done() <-chan struct{} { return r.done }

// Launcher returns the resolved AgentLauncher (cfg.Launcher or DirectLauncher).
// Used by the coordinator to opt-in to ColdStartAware capability outside the
// runtime package.
func (r *Runtime) Launcher() AgentLauncher { return launcher(r.cfg) }

func (r *Runtime) ResetWarmState() error {
	if r.warmFrames == nil {
		return nil
	}
	return r.warmFrames.Reset()
}

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

// MountsForFrame returns the pathmap.Mounts registered for the given frame,
// used by the container endpoint to translate container paths to host paths.
// Returns (nil, false) for non-sandbox frames.
func (r *Runtime) MountsForFrame(frameID state.FrameID) (pathmap.Mounts, bool) {
	v, ok := r.containerMounts.Load(frameID)
	if !ok {
		return nil, false
	}
	ms, ok := v.(pathmap.Mounts)
	return ms, ok
}

// SetRelay registers a FileRelay with the runtime via the event loop.
func (r *Runtime) SetRelay(fr *FileRelay) {
	r.internalCh <- internalSetRelay{relay: fr}
}

// StartTapsForRestoredFrames attaches a pane tap to each frame that was
// restored from the snapshot.  Normal sessions route through
// EvTmuxPaneSpawned → EffRegisterPane → tapManager.start, but bootstrap
// paths (warm restart, cold-start RecreateAll) populate sessionPanes
// directly without emitting that effect, leaving restored frames
// without a tap.  Call once from main.go after Run has been started.
func (r *Runtime) StartTapsForRestoredFrames() {
	r.enqueueInternal(internalStartRestoredTaps{})
}

// Run is the event loop. It blocks until ctx is cancelled.
//
// Internal events (connOpen, connClose) bypass state.Reduce and go
// straight to dispatchInternal — they manipulate runtime fields the
// reducer can't see (the conns map, the next conn id counter).
func (r *Runtime) Run(ctx context.Context) error {
	defer close(r.done)
	defer r.workers.Stop()
	defer r.shutdownIPC()
	defer r.cfg.EventLog.CloseAll()
	defer r.cfg.ToolLog.CloseAll()
	defer r.deactivateBeforeExit()
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
	fastTicker := time.NewTicker(r.cfg.FastTickInterval)
	defer fastTicker.Stop()

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

		case <-fastTicker.C:
			r.scheduleActiveFramePaneProbe()

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

// scheduleActiveFramePaneProbe は active frame (pane 0.1 にスワップ中) の
// 死亡を高速検出する。PaneAlive の tmux shell-out をゴルーチンに委譲して
// event loop をブロックしない。同時実行は atomic guard で 1 本に制限する。
//
// ターゲットは frame の pane_id を使う。remain-on-exit off で frame が exit
// すると tmux が pane を破棄し layout を詰めるため、positional "0.1" は
// 別の生存 pane を指してしまう。pane_id ならプロセスと一緒に消えるので、
// display-message が err を返したケースも dead 扱いにする。
func (r *Runtime) scheduleActiveFramePaneProbe() {
	if r.activeFrameID == "" {
		return
	}
	target := r.sessionPanes[r.activeFrameID]
	if target == "" {
		return
	}
	if !r.fastProbeInFlight.CompareAndSwap(false, true) {
		return
	}
	frameID := r.activeFrameID // snapshot owned by event loop goroutine
	go func() {
		defer r.fastProbeInFlight.Store(false)
		alive, err := r.cfg.Tmux.PaneAlive(target)
		if err == nil && alive {
			return
		}
		slog.Info("runtime: fast probe detected pane dead",
			"frame", frameID, "target", target, "err", err)
		r.Enqueue(state.EvPaneDied{
			Pane:         "{sessionName}:0.1",
			OwnerFrameID: frameID,
		})
	}()
}

// dispatch runs Reduce against the current state and executes every
// resulting effect. Effects may enqueue more events into r.eventCh
// (e.g. tmux spawn → EvTmuxWindowSpawned), which are picked up on
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
	case state.EvTmuxPaneSpawned:
		return "tmux-spawned:" + string(e.FrameID)
	case state.EvTmuxSpawnFailed:
		return "tmux-spawn-failed:" + string(e.FrameID)
	case state.EvTmuxWindowVanished:
		return "tmux-vanished:" + string(e.FrameID)
	case state.EvFrameCommandExited:
		return "cmd-exit:" + string(e.FrameID)
	case state.EvPaneDied:
		return "pane-died:" + string(e.OwnerFrameID)
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
