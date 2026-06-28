package runtime

import (
	"errors"

	"github.com/takezoh/agent-reactor/client/state"
)

// ErrFrameMissing reports that the requested frame is not known to the backend.
// PtyBackend returns errors that wrap this sentinel when the termvt.Manager
// has no session under target. The runtime distinguishes vanished frames from
// transient failures via errors.Is(err, ErrFrameMissing).
var ErrFrameMissing = errors.New("frame missing")

// ErrNotImplemented is returned by backend methods that are not implemented
// on a given backend type.
var ErrNotImplemented = errors.New("runtime: not implemented on this backend")

// Backend interfaces. The runtime depends on these abstractions, not
// on concrete backend/persistence/fs/log libraries, so tests can plug in
// fakes and so the production wiring lives in one place (cmd/main).

// FrameLifecycle covers frame (pty session) creation, destruction, and liveness.
type FrameLifecycle interface {
	// SpawnFrame creates a new pty session backed by frameID. The frame backend
	// uses frameID as its termvt.Manager session key, so the runtime can
	// address every subsequent op by string(FrameID) without an indirection.
	SpawnFrame(frameID, name, command, startDir string, env map[string]string) error
	// KillFrame destroys the pty session identified by frameID.
	KillFrame(frameID string) error
	// RespawnFrame restarts the command in a dead frame.
	RespawnFrame(target, command string) error
	// FrameExitStatus reports the exit code of a dead frame.
	// Returns (true, code) when the frame is dead and exit status was
	// captured; (false, -1) when the frame is alive or no exit status is
	// available. Requires the frame to have been spawned with
	// remain-on-exit=on.
	FrameExitStatus(target string) (dead bool, code int, err error)
}

// FrameIO covers key input and buffer operations directed at a frame.
type FrameIO interface {
	// SendKeys writes text into the frame's input followed by Enter.
	SendKeys(frameID, text string) error
	// SendKey writes a single named key (e.g. "Escape", "q") into the frame
	// without an appended Enter.
	SendKey(frameID, key string) error
	// SendEnter writes a bare Enter into the frame (used to submit a previously
	// pasted prompt without re-typing it).
	SendEnter(target string) error
	// LoadBuffer stores text under a name in a per-backend paste-buffer table.
	// PasteBuffer later consumes it. The split exists because the backend's
	// bracketed-paste path expects buffer-then-paste rather than a single write.
	LoadBuffer(name, text string) error
	// PasteBuffer writes the named buffer's contents into the target frame and
	// drops the buffer afterwards. Used by InjectPrompt to deliver multi-line
	// text without each newline being interpreted as submit by Ink TUIs.
	PasteBuffer(name, target string) error
	// PipeFrame attaches a shell command to the frame's output stream so the
	// runtime can observe raw bytes. An empty command stops a running pipe.
	// PtyBackend implements this as a no-op because PtyFrameTap subscribes
	// directly to the termvt.Manager.
	PipeFrame(frameID, command string) error
}

// FrameInspect covers read-only frame introspection.
type FrameInspect interface {
	// ResolveID returns the backend's internal id for the target frame.
	ResolveID(target string) (string, error)
	// FrameSize returns the visible size of the target frame.
	FrameSize(target string) (width, height int, err error)
	// CaptureFrame returns the trailing nLines of a frame's surface content
	// (no SGR). Used by polling drivers via the worker pool.
	CaptureFrame(frameID string, nLines int) (string, error)
}

// SessionEnv covers session-level environment variable operations.
type SessionEnv interface {
	// SetEnv writes a session-level environment variable.
	SetEnv(key, value string) error
	// UnsetEnv removes a session-level env var.
	UnsetEnv(key string) error
	// ShowEnvironment returns the session environment as a
	// newline-delimited KEY=VALUE string (output of show-environment).
	ShowEnvironment() (string, error)
}

// WindowLayout covers frame repositioning operations (legacy tmux era).
type WindowLayout interface {
	// SwapPane exchanges two frame positions without changing frame ids.
	SwapPane(srcPane, dstPane string) error
	// BreakPane moves a frame into another window.
	BreakPane(srcPane, dstWindow string) error
	// BreakPaneToNewWindow moves a frame into a newly created window and
	// returns that window's index.
	BreakPaneToNewWindow(srcPane, name string) (string, error)
	// JoinPane moves a frame into another frame slot. sizePct controls
	// the new frame size; before inserts before the target frame.
	JoinPane(srcPane, dstPane string, before bool, sizePct int) error
	// SelectPane focuses a backend frame.
	SelectPane(target string) error
	// ResizeWindow resizes the frame window containing the target.
	ResizeWindow(target string, width, height int) error
	// RunChain executes a sequence of swap (or other) commands as
	// a single backend invocation. Used for the swap preview chain.
	RunChain(ops ...[]string) error
}

// BackendControl covers session/client-level control operations that only
// the legacy tmux backend implemented. PtyBackend stubs each method as a
// no-op so the runtime stays backend-agnostic and reducers can emit the
// corresponding Eff{StatusLine,DetachClient,KillSession,DisplayPopup}
// without checking which backend is wired in. See effect.go for the per-
// effect rationale.
type BackendControl interface {
	// SetStatusLine writes the status-left line (tmux era).
	SetStatusLine(line string) error
	// DetachClient detaches the current attached client (tmux era).
	DetachClient() error
	// KillSession destroys the client session (tmux era).
	KillSession() error
	// DisplayPopup runs a popup window for a named tool (tmux era).
	DisplayPopup(width, height, cmd string) error
}

// FrameBackend is the full set of frame (pty session) operations the runtime
// needs. PtyBackend is the production implementation; tests use stubs.
// Methods that return data are synchronous (the runtime calls them
// from execute() and waits for the result before queueing the
// follow-up event).
//
// New code that needs only a subset of these operations should depend on
// the narrower role interfaces (FrameLifecycle, FrameIO, FrameInspect,
// SessionEnv, WindowLayout, BackendControl) instead.
//
// FrameID identifiers are passed as plain strings (string(FrameID)) at the
// backend boundary; persistence layer governs the FrameID typedef.
type FrameBackend interface {
	FrameLifecycle
	FrameIO
	FrameInspect
	SessionEnv
	WindowLayout
	BackendControl
}

// PersistBackend abstracts sessions.json persistence so tests don't
// touch the filesystem.
//
// Save is upsert-only: it writes each snapshot but never removes
// existing records that are absent from the input. Removal is
// explicit via Delete(id). This split prevents catastrophic data loss
// when in-memory state is transiently empty — the empty-list signal
// is no longer interpreted as "wipe everything".
type PersistBackend interface {
	Save(sessions []SessionSnapshot) error
	Delete(id string) error
	Load() ([]SessionSnapshot, error)
}

// SessionSnapshot is the on-disk format for one session in
// sessions.json. Includes the static metadata + the driver's persisted
// bag (opaque map of strings). Frame ids are tracked in session env
// vars (ROOST_SESSION_<sid>); sessions.json stays frame-id free.
type SessionSnapshot struct {
	ID          string                 `json:"id"`
	Project     string                 `json:"project"`
	CreatedAt   string                 `json:"created_at"`
	Frames      []SessionFrameSnapshot `json:"frames"`
	HeadFrameID string                 `json:"head_frame_id,omitempty"`
	MRUFrameIDs []string               `json:"mru_frame_ids,omitempty"`
	Sandbox     state.SandboxOverride  `json:"sandbox,omitempty"`
}

type SessionFrameSnapshot struct {
	ID            string              `json:"id"`
	SubsystemID   string              `json:"subsystem_id,omitempty"`
	TargetID      string              `json:"target_id,omitempty"`
	Project       string              `json:"project"`
	Command       string              `json:"command"`
	LaunchOptions state.LaunchOptions `json:"launch_options,omitempty"`
	CreatedAt     string              `json:"created_at"`
	Driver        string              `json:"driver"`
	DriverState   map[string]string   `json:"driver_state"`
}

// EventLogBackend writes per-session event log lines. The
// implementation lazily opens the file on first append and keeps it
// open until Close(sessionID) is called.
type EventLogBackend interface {
	Append(frameID state.FrameID, line string) error
	Close(frameID state.FrameID)
	CloseAll()
}

// ToolLogBackend writes per-project tool-use JSONL lines. Namespace
// identifies the driver (opaque to the runtime). Project is the
// projectDir() slug (e.g. "-workspace-agent-reactor"). Files are kept open
// and flushed lazily; CloseAll must be called on shutdown.
type ToolLogBackend interface {
	Append(namespace, project, line string) error
	CloseAll()
}

// FSWatcher is the fsnotify wrapper. It watches per-session
// files and emits FSEvent values on Events() when they change.
type FSWatcher interface {
	Watch(frameID state.FrameID, path string) error
	Unwatch(frameID state.FrameID) error
	Events() <-chan FSEvent
	Close() error
}

// FSEvent is the runtime-side representation of a file change.
// SessionID is set by the watcher (which knows the path → session
// mapping it was given via Watch).
type FSEvent struct {
	FrameID state.FrameID
	Path    string
}

// === noop backends (used until production wiring lands) ===

type noopBackend struct{}

func (noopBackend) SpawnFrame(frameID, name, command, startDir string, env map[string]string) error {
	return nil
}
func (noopBackend) KillFrame(string) error         { return nil }
func (noopBackend) RunChain(...[]string) error     { return nil }
func (noopBackend) SwapPane(string, string) error  { return nil }
func (noopBackend) BreakPane(string, string) error { return nil }
func (noopBackend) BreakPaneToNewWindow(string, string) (string, error) {
	return "", nil
}
func (noopBackend) JoinPane(string, string, bool, int) error  { return nil }
func (noopBackend) ResolveID(string) (string, error)          { return "", nil }
func (noopBackend) FrameSize(string) (int, int, error)        { return 0, 0, nil }
func (noopBackend) SelectPane(string) error                   { return nil }
func (noopBackend) ResizeWindow(string, int, int) error       { return nil }
func (noopBackend) SetStatusLine(string) error                { return nil }
func (noopBackend) SetEnv(string, string) error               { return nil }
func (noopBackend) UnsetEnv(string) error                     { return nil }
func (noopBackend) FrameExitStatus(string) (bool, int, error) { return false, -1, nil }
func (noopBackend) RespawnFrame(string, string) error         { return nil }
func (noopBackend) CaptureFrame(string, int) (string, error)  { return "", nil }
func (noopBackend) ShowEnvironment() (string, error)          { return "", nil }
func (noopBackend) DetachClient() error                       { return nil }
func (noopBackend) KillSession() error                        { return nil }
func (noopBackend) DisplayPopup(string, string, string) error { return nil }
func (noopBackend) PipeFrame(string, string) error            { return nil }
func (noopBackend) SendKeys(string, string) error             { return nil }
func (noopBackend) SendKey(string, string) error              { return nil }
func (noopBackend) LoadBuffer(string, string) error           { return nil }
func (noopBackend) PasteBuffer(string, string) error          { return nil }
func (noopBackend) SendEnter(string) error                    { return nil }

type noopPersist struct{}

func (noopPersist) Save([]SessionSnapshot) error     { return nil }
func (noopPersist) Delete(string) error              { return nil }
func (noopPersist) Load() ([]SessionSnapshot, error) { return nil, nil }

type noopEventLog struct{}

func (noopEventLog) Append(state.FrameID, string) error { return nil }
func (noopEventLog) Close(state.FrameID)                {}
func (noopEventLog) CloseAll()                          {}

type noopToolLog struct{}

func (noopToolLog) Append(string, string, string) error { return nil }
func (noopToolLog) CloseAll()                           {}

type noopWatcher struct {
	ch chan FSEvent
}

func (n noopWatcher) Watch(state.FrameID, string) error { return nil }
func (n noopWatcher) Unwatch(state.FrameID) error       { return nil }
func (n noopWatcher) Events() <-chan FSEvent {
	if n.ch == nil {
		return nil
	}
	return n.ch
}
func (n noopWatcher) Close() error { return nil }

// === eventTypeName for diagnostic logging (avoid pulling in fmt %T) ===

func eventTypeName(ev state.Event) string {
	switch ev.(type) {
	case state.EvTick:
		return "EvTick"
	case state.EvEvent:
		return "EvEvent"
	case state.EvJobResult:
		return "EvJobResult"
	case state.EvFrameSpawned:
		return "EvFrameSpawned"
	case state.EvSpawnFailed:
		return "EvSpawnFailed"
	case state.EvFileChanged:
		return "EvFileChanged"
	default:
		return "Event"
	}
}
