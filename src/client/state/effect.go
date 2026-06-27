package state

import "github.com/takezoh/agent-reactor/client/uiproc"

// Effect is the closed sum type of every side effect the reducer can
// request. The runtime's effect interpreter (runtime.execute) is the
// only place that turns these into actual I/O. Adding a new effect =
// adding a struct + an interpret case.
type Effect interface {
	isEffect()
}

// === pane backend operations (synchronous, fast — interpret inline) ===

// EffSpawnPaneWindow asks the runtime to create a new pane window for
// the given session. The runtime executes this and feeds back
// EvPaneSpawned / EvSpawnFailed, forwarding the Reply*
// fields so the reducer can complete the create-session round trip.
type EffSpawnPaneWindow struct {
	SessionID  SessionID
	FrameID    FrameID
	Mode       LaunchMode
	Project    string
	Command    string
	StartDir   string
	Sandbox    SandboxOverride
	Options    LaunchOptions
	Subsystem  LaunchSubsystem
	Stream     StreamLaunchOptions
	Stdin      []byte // piped into the spawned command; nil = no stdin
	Env        map[string]string
	ReplyConn  ConnID
	ReplyReqID string
}

// EffKillSessionWindow destroys the pane window containing the given session pane.
// The runtime looks up the pane target from its sessionPanes map.
type EffKillSessionWindow struct {
	FrameID FrameID
}

// EffActivateSession moves a session's agent pane into pane 0.0.
// The runtime resolves the current pane target from its sessionPanes map.
type EffActivateSession struct {
	SessionID SessionID
	Reason    string
}

// EffDeactivateSession moves the currently active session back to its
// own window, leaving pane 0.0 showing the main TUI.
type EffDeactivateSession struct{}

// EffRegisterPane records the pane target for a session in the runtime
// and saves it as a session-level env var. Tap controls whether a
// byte tap (PaneTap) is started for this pane — only root frames need
// taps since driver state for non-root frames is not displayed in the UI.
type EffRegisterPane struct {
	FrameID    FrameID
	PaneTarget string
	Tap        bool
}

// EffUnregisterPane removes a session from the runtime's pane map and
// deletes the corresponding session-level env var.
type EffUnregisterPane struct {
	FrameID FrameID
}

// EffSelectPane focuses a backend pane.
type EffSelectPane struct {
	Target string
}

// EffSyncStatusLine pushes a string into the status-left line.
type EffSyncStatusLine struct {
	Line string
}

// EffSetPaneEnv writes a session-level environment variable.
// Empty Value is treated as unset.
type EffSetPaneEnv struct {
	Key   string
	Value string
}

// EffUnsetPaneEnv removes a session-level env var.
type EffUnsetPaneEnv struct {
	Key string
}

// EffCheckPaneAlive asks the runtime to query #{pane_dead} for the
// named pane. If dead, runtime emits EvPaneDied.
type EffCheckPaneAlive struct {
	Pane string
}

// EffRespawnPane respawns a backend pane (used by health monitor).
// Proc identifies which UI process to launch; the runtime calls
// Proc.Command(roostExe) to build the shell command string.
type EffRespawnPane struct {
	Pane string
	Proc uiproc.UIProcess
}

// EffDetachClient asks the backend to detach the current attached client.
// Survives from the tmux era when arc could attach to a shared multiplexer
// session; PtyBackend implements DetachClient as a no-op so the runtime
// stays backend-agnostic and reducers can remain unaware of the swap.
type EffDetachClient struct{}

// EffReleaseFrameSandboxes asks the runtime to destroy all sandbox resources
// (Docker containers, VMs, …) held by active frames. Emitted by reduceShutdown
// only — reduceDetach must NOT emit it so containers survive for warm-restart
// adoption. The runtime handles this with drainFrameCleanups (parallel, blocking).
type EffReleaseFrameSandboxes struct{}

// EffDisplayPopup launches a transient popup window for a named tool.
// PtyBackend implements DisplayPopup as a no-op (no popup support without an
// external multiplexer); reducers still emit it so a future client-side
// overlay layer (see plan A follow-up) can consume the same path without
// reducer surgery. Tool and Args are structured values — the runtime builds
// the shell command string with proper escaping, avoiding injection.
type EffDisplayPopup struct {
	Width  string
	Height string
	Tool   string
	Args   map[string]string
}

// EffKillSession asks the backend to destroy its own session and exit. On
// tmux this collapsed the whole client; PtyBackend implements KillSession as
// a no-op because the backend's lifetime is already bound to the daemon
// process (ADR 0004, decision 2). reduceShutdown emits this regardless so
// future backends with separable lifecycles can plug in without reducer
// surgery.
type EffKillSession struct{}

// === IPC operations ===

// EffSendResponse sends a typed response to a specific connection.
// The Body is encoded by the runtime as a proto.Response value.
type EffSendResponse struct {
	ConnID ConnID
	ReqID  string
	Body   any // proto.Response (kept any here so state pkg doesn't import proto)
}

// EffSendResponseSync writes and flushes a typed response to a specific
// connection before the runtime proceeds to later effects.
type EffSendResponseSync struct {
	ConnID ConnID
	ReqID  string
	Body   any // proto.Response (kept any here so state pkg doesn't import proto)
}

// EffSendError sends an error response. Code is a proto.ErrCode (string)
// kept generic at the state layer.
type EffSendError struct {
	ConnID  ConnID
	ReqID   string
	Code    string
	Message string
	Details map[string]any
}

// EffBroadcastSessionsChanged tells the runtime to build the current
// sessions-changed payload from State and broadcast it to subscribers.
// No payload is carried — runtime reads State directly so we don't
// pay for an extra clone.
type EffBroadcastSessionsChanged struct {
	IsPreview bool
}

// EffBroadcastEvent broadcasts a generic typed event to subscribers
// matching FilterTag (empty = no filter).
type EffBroadcastEvent struct {
	Name      string
	Payload   any
	FilterTag string
}

// EffCloseConn closes a specific connection.
type EffCloseConn struct {
	ConnID ConnID
}

// EffSendPaneKeys asks the runtime to send key input against the pane
// belonging to SessionID. WithEnter=true appends an Enter keypress (send_text
// semantics); WithEnter=false sends Key as a literal key name (send_key semantics).
type EffSendPaneKeys struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
	Text      string // non-empty when WithEnter=true
	Key       string // non-empty when WithEnter=false
	WithEnter bool
}

// === Persistence / fs ===

// EffPersistSnapshot tells the runtime to write the current State to
// sessions.json. No payload — runtime reads State directly.
type EffPersistSnapshot struct{}

// EffWatchFile registers a file with the fsnotify watcher.
type EffWatchFile struct {
	FrameID FrameID
	Path    string
	Kind    string
}

// EffUnwatchFile removes a file from the watcher.
type EffUnwatchFile struct {
	FrameID FrameID
}

// EffEventLogAppend appends a single line to a session's event log
// file. The runtime owns the file handles (lazy-opened, kept open
// across appends, closed on session destroy).
type EffEventLogAppend struct {
	FrameID FrameID
	Line    string
}

// EffToolLogAppend appends a pre-marshalled JSONL line to the
// per-project tool log at <dataDir>/<namespace>/tool-logs/<project>.jsonl.
// Namespace is an opaque driver-supplied token; the runtime must not branch on its value.
// Project is the projectDir() slug (e.g. "-workspace-agent-reactor").
// Line must not contain a trailing newline; the backend adds it.
type EffToolLogAppend struct {
	Namespace string
	Project   string
	Line      string
}

// === Reconciliation ===

// EffReconcileWindows asks the runtime to compare the live backend
// window list against state.Sessions and emit EvPaneWindowVanished
// for any session whose window has disappeared.
type EffReconcileWindows struct{}

// === Async work ===

// JobInput is implemented by all job input types. JobKind returns the
// registry key used to look up the runner.
type JobInput interface {
	JobKind() string
}

// EffRecordNotification broadcasts an OSC-sourced in-pane notification
// to TUI subscribers and logs it to the session event log.
// SessionID and FrameID are filled by postProcessEffect when blank.
type EffRecordNotification struct {
	SessionID SessionID
	FrameID   FrameID
	Cmd       int // 9 / 99 / 777
	Title     string
	Body      string
}

// EffPushDriver asks the reducer to push a new driver frame onto the
// given session. Drivers return this from Step to request a frame
// push without knowing the session ID directly. The reducer fills in
// SessionID when it is empty (using the owning session).
type EffPushDriver struct {
	SessionID SessionID
	Command   string
}

func (EffPushDriver) isEffect() {}

// EffStartJob enqueues a job on the worker pool. JobID is allocated
// by the reducer (via State.NextJobID) and recorded in State.Jobs so
// the EvJobResult callback can be routed back to the right session.
type EffStartJob struct {
	JobID JobID
	Input JobInput
}

// === isEffect markers ===

func (EffSpawnPaneWindow) isEffect()          {}
func (EffKillSessionWindow) isEffect()        {}
func (EffActivateSession) isEffect()          {}
func (EffDeactivateSession) isEffect()        {}
func (EffRegisterPane) isEffect()             {}
func (EffUnregisterPane) isEffect()           {}
func (EffSelectPane) isEffect()               {}
func (EffSyncStatusLine) isEffect()           {}
func (EffSetPaneEnv) isEffect()               {}
func (EffUnsetPaneEnv) isEffect()             {}
func (EffCheckPaneAlive) isEffect()           {}
func (EffRespawnPane) isEffect()              {}
func (EffDetachClient) isEffect()             {}
func (EffReleaseFrameSandboxes) isEffect()    {}
func (EffDisplayPopup) isEffect()             {}
func (EffKillSession) isEffect()              {}
func (EffSendResponse) isEffect()             {}
func (EffSendResponseSync) isEffect()         {}
func (EffSendError) isEffect()                {}
func (EffBroadcastSessionsChanged) isEffect() {}
func (EffBroadcastEvent) isEffect()           {}
func (EffCloseConn) isEffect()                {}
func (EffPersistSnapshot) isEffect()          {}
func (EffWatchFile) isEffect()                {}
func (EffUnwatchFile) isEffect()              {}
func (EffEventLogAppend) isEffect()           {}
func (EffToolLogAppend) isEffect()            {}
func (EffReconcileWindows) isEffect()         {}
func (EffStartJob) isEffect()                 {}
func (EffRecordNotification) isEffect()       {}
func (EffSendPaneKeys) isEffect()             {}

// EffInjectPrompt asks the runtime to paste text into the backend pane belonging
// to FrameID using bracketed paste (load-buffer + paste-buffer -d) followed by
// Enter. The pane must be idle; callers are responsible for checking status.
type EffInjectPrompt struct {
	FrameID FrameID
	Text    string
}

func (EffInjectPrompt) isEffect() {}

// EffSwapHidden exchanges pane 0.1 with the hidden pane (__hidden__ window).
// Used to switch pane 0.1 between the main TUI and the log TUI without
// killing either process — both TUIs stay alive as persistent processes.
type EffSwapHidden struct{}

func (EffSwapHidden) isEffect() {}

// === Surface streaming (PR-2 reducer-only; runtime wires in PR-3) ===

// EffSurfaceSubscribeStart asks the runtime to begin streaming pane
// output for SessionID toward ConnID. The reducer guarantees the
// session exists and that ActiveFrame is non-nil before emitting this.
// The runtime is responsible for starting the relay goroutine.
type EffSurfaceSubscribeStart struct {
	ConnID    ConnID
	SessionID SessionID
}

func (EffSurfaceSubscribeStart) isEffect() {}

// EffSurfaceSubscribeStop asks the runtime to stop the relay started by
// EffSurfaceSubscribeStart. Idempotent on the runtime side. Emitted on
// explicit unsubscribe and on EvConnClosed for every still-subscribed
// (ConnID, SessionID) pair.
type EffSurfaceSubscribeStop struct {
	ConnID    ConnID
	SessionID SessionID
}

func (EffSurfaceSubscribeStop) isEffect() {}

// EffSurfaceResize forwards a logical resize request to the pty backend
// behind SessionID. The runtime resolves the backend from its internal
// sessionPanes map.
type EffSurfaceResize struct {
	SessionID SessionID
	Cols      uint16
	Rows      uint16
}

func (EffSurfaceResize) isEffect() {}

// EffSurfaceWriteRaw forwards uninterpreted bytes to SessionID's pane.
// Data is owned by the effect; the reducer must not retain a reference
// after emitting.
type EffSurfaceWriteRaw struct {
	SessionID SessionID
	Data      []byte
}

func (EffSurfaceWriteRaw) isEffect() {}

// EffBroadcastSurfaceOutput tells the runtime to broadcast one chunk of
// pane output as EvtSurfaceOutput to all ConnIDs subscribed to SessionID
// (per State.SurfaceSubs). The runtime is responsible for assigning the
// per-subscribe Sequence number.
type EffBroadcastSurfaceOutput struct {
	SessionID SessionID
	Data      []byte
	TimeSec   float64
}

func (EffBroadcastSurfaceOutput) isEffect() {}

// EffBroadcastPromptEvent tells the runtime to broadcast a prompt-state
// transition (EvtPromptEvent on the wire) to all subscribers interested
// in FrameID's session.
type EffBroadcastPromptEvent struct {
	FrameID  FrameID
	Phase    string
	ExitCode int
}

func (EffBroadcastPromptEvent) isEffect() {}
