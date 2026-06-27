package state

import (
	"encoding/json"
	"time"
)

// Event is the closed sum type of every input the reducer accepts.
// Adding a new event = adding a struct + a Reduce case. The compiler
// + the panic in Reduce's default branch ensures we cover them all.
type Event interface {
	isEvent()
}

// Event type constants for dispatch by reduceEvent.
const (
	EventCreateSession  = "create-session"
	EventStopSession    = "stop-session"
	EventListSessions   = "list-sessions"
	EventPreviewSession = "preview-session"
	EventSwitchSession  = "switch-session"
	EventPreviewProject = "preview-project"
	EventShutdown       = "shutdown"
	EventPushDriver     = "push-driver"
	EventForkSession    = "fork-session"
	EventActivateFrame  = "activate-frame"
)

// === IPC commands (caller → daemon) ===

// EvCmdSubscribe registers ConnID as a broadcast subscriber. Filters
// is the set of event names to receive; an empty list means all.
type EvCmdSubscribe struct {
	ConnID  ConnID
	ReqID   string
	Filters []string
}

// EvCmdUnsubscribe removes ConnID from the broadcast list without
// closing the connection.
type EvCmdUnsubscribe struct {
	ConnID ConnID
	ReqID  string
}

// EvEvent is a registered command event (create-session, stop-session, etc.)
// dispatched from TUI/tools/keybindings via the registry.
type EvEvent struct {
	ConnID  ConnID
	ReqID   string
	Event   string
	Payload json.RawMessage
}

// EvCmdSurfaceReadText requests the trailing lines of a session's pane.
type EvCmdSurfaceReadText struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
	Lines     int // 0 = server default
}

// EvCmdSurfaceSendText sends Text + Enter to a session's active pane.
type EvCmdSurfaceSendText struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
	Text      string
}

// EvCmdSurfaceSendKey sends a named key to a session's active pane.
type EvCmdSurfaceSendKey struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
	Key       string
}

// EvCmdSurfaceSubscribe registers ConnID as a streaming subscriber for
// the named session's pane output. While subscribed, the runtime emits
// EvtSurfaceOutput broadcasts addressed to ConnID. Multiple SessionIDs
// can be subscribed under one ConnID; the per-ConnID cap (see ADR 0007)
// is enforced by the reducer.
type EvCmdSurfaceSubscribe struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
}

// EvCmdSurfaceUnsubscribe removes ConnID's subscription for SessionID.
// Idempotent: unsubscribing an already-removed entry returns RespOK.
type EvCmdSurfaceUnsubscribe struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
}

// EvCmdSurfaceResize requests a logical pane resize to (Cols, Rows) for
// SessionID. The reducer forwards this to the runtime via EffSurfaceResize;
// the runtime delegates to the pty backend.
type EvCmdSurfaceResize struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
	Cols      uint16
	Rows      uint16
}

// EvCmdSurfaceWriteRaw writes uninterpreted bytes to SessionID's pane.
// Data is the raw byte slice (already base64-decoded by the proto layer).
// No Enter is appended; key names are not interpreted.
type EvCmdSurfaceWriteRaw struct {
	ConnID    ConnID
	ReqID     string
	SessionID SessionID
	Data      []byte
}

// EvCmdDriverList requests the list of registered drivers.
type EvCmdDriverList struct {
	ConnID ConnID
	ReqID  string
}

// EvDriverEvent is a driver hook event from the agent process via
// `server event <eventType>`. Routed to the session's driver.
type EvDriverEvent struct {
	ConnID    ConnID
	ReqID     string
	Event     string
	Timestamp time.Time
	SenderID  FrameID
	Payload   json.RawMessage
}

type EvSubsystem struct {
	ConnID    ConnID
	ReqID     string
	FrameID   FrameID
	Source    SubsystemKind
	Kind      SubsystemEventKind
	Timestamp time.Time
	Payload   SubsystemPayload
}

// === Connection lifecycle ===

type EvConnOpened struct {
	ConnID ConnID
}

type EvConnClosed struct {
	ConnID ConnID
}

// === Timer / I/O feedback ===

// EvTick is the periodic tick fired by runtime's ticker. Drivers run
// their Step{DEvTick} on every tick. PaneTargets maps each FrameID
// to its pane id (e.g. "%5"), pre-filled by the runtime so reducers
// can forward it to drivers without touching the runtime directly.
// N is a monotonic counter used for effect bucketing (gate expensive
// effects to every N-th tick rather than every tick).
type EvTick struct {
	Now         time.Time
	PaneTargets map[FrameID]string
	N           uint64
}

// EvFileChanged is fired by runtime's fsnotify watcher when a
// session's watched file changes on disk.
type EvFileChanged struct {
	FrameID FrameID
	Path    string
}

// EvJobResult delivers a worker pool job's result back to the reducer.
type EvJobResult struct {
	JobID  JobID
	Result any
	Err    error
}

// EvPaneWindowVanished is fired by ReconcileWindows when the pane backend's
// window backing a frame has truly disappeared (the user closed the
// window via the backend's own kill-window, for instance). The frame is
// always evicted because there is nothing left to inspect.
type EvPaneWindowVanished struct {
	FrameID FrameID
}

// EvFrameCommandExited is fired by ReconcileWindows when a frame's
// command process has exited but the pane is still around (windows
// are spawned with remain-on-exit=on so the tail output and exit
// status can be inspected). The reducer decides:
//   - ExitCode == 0  → intentional exit, evict the frame and kill
//     the dead window.
//   - ExitCode != 0  → abnormal exit, mark the frame status=stopped
//     and leave the window for the user to inspect.
//
// Idempotent: the reducer ignores the event when the frame's driver
// is already at StatusStopped, so re-detection on subsequent ticks is
// safe.
type EvFrameCommandExited struct {
	FrameID  FrameID
	ExitCode int
}

// EvPaneSpawned is the async result of a backend new-window call
// initiated by EffSpawnPaneWindow. PaneTarget is the pane id the runtime
// uses to route activate/capture effects. SubsystemID is the opaque
// identifier the subsystem factory chose for this frame's backend; the
// reducer writes it onto the frame for future routing. WorktreeStartDir
// is non-empty when the subsystem created a managed worktree during
// BindFrame; the reducer routes DEvWorktreeResolved to the frame's driver
// so the path is persisted for cold-start reconstruction.
type EvPaneSpawned struct {
	SessionID        SessionID
	FrameID          FrameID
	SubsystemID      SubsystemID
	PaneTarget       string
	WorktreeStartDir string
	WorktreeName     string
	ReplyConn        ConnID
	ReplyReqID       string
}

// EvSpawnFailed is the async failure of a backend new-window call.
// The reducer evicts the half-created session and replies to the
// original caller with an error.
type EvSpawnFailed struct {
	SessionID  SessionID
	FrameID    FrameID
	Err        string
	ReplyConn  ConnID
	ReplyReqID string
}

// EvPaneOsc is fired by the PaneTap reader goroutine when an OSC
// notification is detected in the raw byte stream from a pane.
// Title and Body are already parsed from the raw payload.
type EvPaneOsc struct {
	FrameID FrameID
	Cmd     int
	Title   string
	Body    string
	Now     time.Time
}

// PromptPhase classifies an OSC 133 semantic-prompt event.
type PromptPhase int

const (
	PromptPhaseNone     PromptPhase = iota
	PromptPhaseStart                // 133;A — prompt rendering started
	PromptPhaseInput                // 133;B — prompt done, awaiting input
	PromptPhaseCommand              // 133;C — command execution started
	PromptPhaseComplete             // 133;D — command finished
)

// EvPanePrompt is fired by the PaneTap reader goroutine when an OSC 133
// semantic-prompt sequence is detected in the raw byte stream from a pane.
type EvPanePrompt struct {
	FrameID  FrameID
	Phase    PromptPhase
	ExitCode *int
	Now      time.Time
}

// === isEvent markers ===

func (EvCmdSubscribe) isEvent()          {}
func (EvCmdUnsubscribe) isEvent()        {}
func (EvCmdSurfaceReadText) isEvent()    {}
func (EvCmdSurfaceSendText) isEvent()    {}
func (EvCmdSurfaceSendKey) isEvent()     {}
func (EvCmdSurfaceSubscribe) isEvent()   {}
func (EvCmdSurfaceUnsubscribe) isEvent() {}
func (EvCmdSurfaceResize) isEvent()      {}
func (EvCmdSurfaceWriteRaw) isEvent()    {}
func (EvCmdDriverList) isEvent()         {}
func (EvEvent) isEvent()                 {}
func (EvDriverEvent) isEvent()           {}
func (EvSubsystem) isEvent()             {}
func (EvConnOpened) isEvent()            {}
func (EvConnClosed) isEvent()            {}
func (EvTick) isEvent()                  {}
func (EvFileChanged) isEvent()           {}
func (EvJobResult) isEvent()             {}
func (EvPaneWindowVanished) isEvent()    {}
func (EvFrameCommandExited) isEvent()    {}
func (EvPaneSpawned) isEvent()           {}
func (EvSpawnFailed) isEvent()           {}
func (EvPaneOsc) isEvent()               {}
func (EvPanePrompt) isEvent()            {}
