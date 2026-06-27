package runtime

import (
	"errors"

	"github.com/takezoh/agent-reactor/client/state"
)

// ErrPaneMissing reports that the requested pane is not known to the backend.
// PtyBackend returns errors that wrap this sentinel when the termvt.Manager
// has no session under target. The runtime distinguishes vanished panes from
// transient failures via errors.Is(err, ErrPaneMissing).
var ErrPaneMissing = errors.New("pane missing")

// ErrNotImplemented is returned by backend methods that are not implemented
// on a given backend type.
var ErrNotImplemented = errors.New("runtime: not implemented on this backend")

// Backend interfaces. The runtime depends on these abstractions, not
// on concrete backend/persistence/fs/log libraries, so tests can plug in
// fakes and so the production wiring lives in one place (cmd/main).

// PaneLifecycle covers pane/window creation, destruction, and liveness.
type PaneLifecycle interface {
	// SpawnWindow creates a new pane window for a session. Returns the
	// window index (e.g. "1") and the pane id (e.g. "%5").
	SpawnWindow(name, command, startDir string, env map[string]string) (windowIndex, paneID string, err error)
	// KillPaneWindow destroys the pane window containing the named pane.
	KillPaneWindow(paneTarget string) error
	// RespawnPane runs respawn-pane against a dead pane.
	RespawnPane(target, command string) error
	// PaneAlive returns true if the named pane is currently alive
	// (i.e. #{pane_dead} == 0). False on error or dead pane.
	PaneAlive(target string) (bool, error)
	// PaneExitStatus reports the exit code of a dead pane via
	// #{pane_dead_status}. Returns (true, code) when the pane is
	// dead and exit status was captured; (false, -1) when the pane
	// is alive or no exit status is available. Requires the pane to
	// have been spawned with remain-on-exit=on.
	PaneExitStatus(target string) (dead bool, code int, err error)
}

// PaneIO covers key input and buffer operations directed at a pane.
type PaneIO interface {
	// SendKeys writes text into the pane's input followed by Enter.
	SendKeys(paneTarget, text string) error
	// SendKey writes a single named key (e.g. "Escape", "q") into the pane
	// without an appended Enter.
	SendKey(paneTarget, key string) error
	// SendEnter writes a bare Enter into the pane (used to submit a previously
	// pasted prompt without re-typing it).
	SendEnter(target string) error
	// LoadBuffer stores text under a name in a per-backend paste-buffer table.
	// PasteBuffer later consumes it. The split exists because the backend's
	// bracketed-paste path expects buffer-then-paste rather than a single write.
	LoadBuffer(name, text string) error
	// PasteBuffer writes the named buffer's contents into the target pane and
	// drops the buffer afterwards. Used by InjectPrompt to deliver multi-line
	// text without each newline being interpreted as submit by Ink TUIs.
	PasteBuffer(name, target string) error
	// PipePane attaches a shell command to the pane's output stream so the
	// runtime can observe raw bytes. An empty command stops a running pipe.
	// PtyBackend implements this as a no-op because PtyPaneTap subscribes
	// directly to the termvt.Manager.
	PipePane(paneTarget, command string) error
}

// PaneInspect covers read-only pane introspection.
type PaneInspect interface {
	// PaneID returns the pane id (e.g. "%5") for the target pane.
	PaneID(target string) (string, error)
	// PaneSize returns the visible size of the target pane.
	PaneSize(target string) (width, height int, err error)
	// CapturePane returns the trailing nLines of a pane's content (no SGR).
	// Used by polling drivers via the worker pool.
	CapturePane(paneTarget string, nLines int) (string, error)
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

// WindowLayout covers pane/window repositioning operations.
type WindowLayout interface {
	// SwapPane exchanges two pane positions without changing pane ids.
	SwapPane(srcPane, dstPane string) error
	// BreakPane moves a pane into another window.
	BreakPane(srcPane, dstWindow string) error
	// BreakPaneToNewWindow moves a pane into a newly created window and
	// returns that window's index.
	BreakPaneToNewWindow(srcPane, name string) (string, error)
	// JoinPane moves a pane into another pane slot. sizePct controls
	// the new pane size; before inserts before the target pane.
	JoinPane(srcPane, dstPane string, before bool, sizePct int) error
	// SelectPane focuses a backend pane.
	SelectPane(target string) error
	// ResizeWindow resizes the pane window containing the target.
	ResizeWindow(target string, width, height int) error
	// RunChain executes a sequence of swap-pane (or other) commands as
	// a single backend invocation. Used for the swap-pane preview chain.
	RunChain(ops ...[]string) error
}

// BackendControl covers session/client-level control operations.
type BackendControl interface {
	// SetStatusLine writes the status-left line (legacy; no-op on
	// PtyBackend).
	SetStatusLine(line string) error
	// DetachClient detaches the current client (legacy).
	DetachClient() error
	// KillSession destroys the client session (legacy).
	KillSession() error
	// DisplayPopup runs a popup window (legacy).
	DisplayPopup(width, height, cmd string) error
}

// PaneBackend is the full set of pane / window operations the runtime
// needs. PtyBackend is the production implementation; tests use stubs.
// Methods that return data are synchronous (the runtime calls them
// from execute() and waits for the result before queueing the
// follow-up event).
//
// New code that needs only a subset of these operations should depend on
// the narrower role interfaces (PaneLifecycle, PaneIO, PaneInspect,
// SessionEnv, WindowLayout, BackendControl) instead.
type PaneBackend interface {
	PaneLifecycle
	PaneIO
	PaneInspect
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
// bag (opaque map of strings). Pane ids are tracked in session env
// vars (ROOST_SESSION_<sid>); sessions.json stays pane-id free.
type SessionSnapshot struct {
	ID            string                 `json:"id"`
	Project       string                 `json:"project"`
	CreatedAt     string                 `json:"created_at"`
	Frames        []SessionFrameSnapshot `json:"frames"`
	ActiveFrameID string                 `json:"active_frame_id,omitempty"`
	MRUFrameIDs   []string               `json:"mru_frame_ids,omitempty"`
	Sandbox       state.SandboxOverride  `json:"sandbox,omitempty"`
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

func (noopBackend) SpawnWindow(name, command, startDir string, env map[string]string) (string, string, error) {
	return "", "", nil
}
func (noopBackend) KillPaneWindow(string) error    { return nil }
func (noopBackend) RunChain(...[]string) error     { return nil }
func (noopBackend) SwapPane(string, string) error  { return nil }
func (noopBackend) BreakPane(string, string) error { return nil }
func (noopBackend) BreakPaneToNewWindow(string, string) (string, error) {
	return "", nil
}
func (noopBackend) JoinPane(string, string, bool, int) error  { return nil }
func (noopBackend) PaneID(string) (string, error)             { return "", nil }
func (noopBackend) PaneSize(string) (int, int, error)         { return 0, 0, nil }
func (noopBackend) SelectPane(string) error                   { return nil }
func (noopBackend) ResizeWindow(string, int, int) error       { return nil }
func (noopBackend) SetStatusLine(string) error                { return nil }
func (noopBackend) SetEnv(string, string) error               { return nil }
func (noopBackend) UnsetEnv(string) error                     { return nil }
func (noopBackend) PaneAlive(string) (bool, error)            { return true, nil }
func (noopBackend) PaneExitStatus(string) (bool, int, error)  { return false, -1, nil }
func (noopBackend) RespawnPane(string, string) error          { return nil }
func (noopBackend) CapturePane(string, int) (string, error)   { return "", nil }
func (noopBackend) ShowEnvironment() (string, error)          { return "", nil }
func (noopBackend) DetachClient() error                       { return nil }
func (noopBackend) KillSession() error                        { return nil }
func (noopBackend) DisplayPopup(string, string, string) error { return nil }
func (noopBackend) PipePane(string, string) error             { return nil }
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
	case state.EvPaneSpawned:
		return "EvPaneSpawned"
	case state.EvSpawnFailed:
		return "EvSpawnFailed"
	case state.EvFileChanged:
		return "EvFileChanged"
	default:
		return "Event"
	}
}
