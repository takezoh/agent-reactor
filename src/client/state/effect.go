package state

// Effect is the closed sum type of every side effect the reducer can
// request. The runtime's effect interpreter (runtime.execute) is the
// only place that turns these into actual I/O. Adding a new effect =
// adding a struct + an interpret case.
type Effect interface {
	isEffect()
}

// === pane backend operations (synchronous, fast — interpret inline) ===

// EffSpawnFrame asks the runtime to create a new pane window for
// the given session. The runtime executes this and feeds back
// EvFrameSpawned / EvSpawnFailed, forwarding the Reply*
// fields so the reducer can complete the create-session round trip.
type EffSpawnFrame struct {
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

// EffKillFrame destroys the pane window for the given frame. The pane
// backend is keyed directly on string(FrameID), so the runtime hands the
// frame id straight to FrameBackend.KillFrame without an intermediate lookup.
type EffKillFrame struct {
	FrameID FrameID
}

// EffRegisterFrame fires post-spawn registration for a frame. Tap controls
// whether a byte tap (FrameTap) is started — only root frames need taps since
// driver state for non-root frames is not displayed in the UI.
type EffRegisterFrame struct {
	FrameID FrameID
	Tap     bool
}

// EffUnregisterFrame fires per-frame teardown after a frame is evicted: stops
// the tap, closes the event log, and evicts the VT emulator.
type EffUnregisterFrame struct {
	FrameID FrameID
}

// EffSetSessionEnv writes a session-level environment variable.
// Empty Value is treated as unset.
type EffSetSessionEnv struct {
	Key   string
	Value string
}

// EffUnsetSessionEnv removes a session-level env var.
type EffUnsetSessionEnv struct {
	Key string
}

// EffReleaseFrameSandboxes asks the runtime to destroy all sandbox resources
// (Docker containers, VMs, …) held by active frames. Emitted by reduceShutdown
// only — daemon shutdown drains frame cleanups in parallel before the process
// exits. ctx-cancel-driven shutdown (warm restart) must NOT emit it so
// containers survive for adoption next boot.
type EffReleaseFrameSandboxes struct{}

// EffReleaseFrameSandbox asks the runtime to invoke the per-frame sandbox
// cleanup closure. The runtime delegates to invokeFrameCleanup which fires
// the closure stored at handleSpawnComplete (devcontainer.makeCleanup =
// Manager.ReleaseFrame → 0 なら DestroyInstance). Per-project / shared 両
// mode は manager 側で cs.isolation().ContainerKey によって正しい粒度に
// 分かれるので、 reducer は frame ID だけ送れば良い。
//
// Emitted by evictFrame (root + child) and reduceFrameCommandExited (異常
// exit 枝) so pane が vanish しても crash しても sandbox は確実に release
// される。 EffKillFrame とは責務を分けていて、 後者は backend
// pane window を kill するためだけのもの。 pane window が既に消えていて
// EffKillFrame が要らない経路 (EvFrameVanished) でも sandbox
// release は別途必要なので別 effect に分けてある。
type EffReleaseFrameSandbox struct {
	FrameID FrameID
}

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
type EffBroadcastSessionsChanged struct{}

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

// EffSendFrameKeys asks the runtime to send key input against the pane
// belonging to SessionID. WithEnter=true appends an Enter keypress (send_text
// semantics); WithEnter=false sends Key as a literal key name (send_key semantics).
type EffSendFrameKeys struct {
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
// window list against state.Sessions and emit EvFrameVanished
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

func (EffSpawnFrame) isEffect()               {}
func (EffKillFrame) isEffect()                {}
func (EffRegisterFrame) isEffect()            {}
func (EffUnregisterFrame) isEffect()          {}
func (EffSetSessionEnv) isEffect()            {}
func (EffUnsetSessionEnv) isEffect()          {}
func (EffReleaseFrameSandboxes) isEffect()    {}
func (EffReleaseFrameSandbox) isEffect()      {}
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
func (EffSendFrameKeys) isEffect()            {}

// === Surface streaming (PR-2 reducer-only; runtime wires in PR-3) ===

// EffSurfaceSubscribeStart asks the runtime to begin streaming pane
// output for SessionID toward ConnID. The reducer guarantees the
// session exists and that the head frame is non-nil before emitting this.
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
// behind SessionID. The runtime resolves the pane id by reading the session's
// head frame and using string(HeadFrameID) directly.
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
