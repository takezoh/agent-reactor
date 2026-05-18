package state

import (
	"encoding/json"
	"strings"
	"time"
)

// DriverState is the per-session, per-driver private state value. Each
// driver impl defines its own concrete type (e.g. driver.ClaudeState,
// driver.GenericState) by embedding DriverStateBase. DriverState values
// are stored inside Session.Driver and round-tripped through reduce.go
// without inspection.
//
// The marker method is unexported to seal the interface to types that
// embed DriverStateBase, so adding a new driver state requires going
// through the explicit embed (visible in code review) rather than
// satisfying the interface accidentally.
type DriverState interface {
	driverStateMarker()
}

// DriverStateBase is the embed-only marker that promotes a struct into
// a valid DriverState. Driver impls embed this as their first field:
//
//	type GenericState struct {
//	    state.DriverStateBase
//	    Status state.Status
//	    ...
//	}
type DriverStateBase struct{}

func (DriverStateBase) driverStateMarker() {}

// DriverEvent is the closed sum type the reducer hands to a Driver's
// Step method. Concrete cases below cover every reason a driver state
// might transition.
type DriverEvent interface {
	isDriverEvent()
}

// FrameContext carries read-only frame metadata into Driver.Step.
// It is assembled by stepDriver from the SessionFrame and is never
// stored inside DriverState or persisted to disk.
//
// IsRoot gating policy: drivers must early-return on every event except
// DEvHook when !IsRoot. Child frames keep only minimal hook-derived
// identity needed for relaunch/resume; UI-facing state, transcript work,
// and polling stay root-only.
type FrameContext struct {
	ID            FrameID
	Project       string
	Command       string
	LaunchOptions LaunchOptions
	CreatedAt     time.Time
	IsRoot        bool
}

// DEvHook is a hook event from the agent via `roost event <eventType>`.
// Payload is the raw JSON from stdin.
type DEvHook struct {
	Event          string
	Timestamp      time.Time
	RoostSessionID string
	Payload        json.RawMessage
}

func (DEvHook) isDriverEvent() {}

type SubsystemKind string

const (
	SubsystemCLI    SubsystemKind = "cli"
	SubsystemStream SubsystemKind = "stream"
)

type SubsystemEventKind string

const (
	SubsystemSessionReady      SubsystemEventKind = "session_ready"
	SubsystemFailed            SubsystemEventKind = "failed"
	SubsystemPromptSubmitted   SubsystemEventKind = "prompt_submitted"
	SubsystemTurnStarted       SubsystemEventKind = "turn_started"
	SubsystemTurnCompleted     SubsystemEventKind = "turn_completed"
	SubsystemToolStarted       SubsystemEventKind = "tool_started"
	SubsystemToolCompleted     SubsystemEventKind = "tool_completed"
	SubsystemApprovalRequested SubsystemEventKind = "approval_requested"
	SubsystemApprovalResolved  SubsystemEventKind = "approval_resolved"
	SubsystemPlanUpdated       SubsystemEventKind = "plan_updated"
	SubsystemDiffUpdated       SubsystemEventKind = "diff_updated"
	SubsystemMessageUpdated    SubsystemEventKind = "message_updated"
)

type SubsystemTurn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type SubsystemTool struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Command        string `json:"command"`
	Path           string `json:"path"`
	Error          string `json:"error"`
	PermissionMode string `json:"permission_mode"`
	IsInterrupt    bool   `json:"is_interrupt"`
}

type SubsystemApproval struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Command     string `json:"command"`
	Path        string `json:"path"`
	Reason      string `json:"reason"`
	AutoApprove bool   `json:"auto_approve"`
	Resolved    bool   `json:"resolved"`
	Denied      bool   `json:"denied"`
}

type SubsystemPlan struct {
	Summary string `json:"summary"`
}

type SubsystemDiff struct {
	Summary string   `json:"summary"`
	Paths   []string `json:"paths"`
}

type SubsystemMessage struct {
	RecentTurns []SubsystemTurn `json:"recent_turns"`
}

type SubsystemPayload struct {
	SessionID            string             `json:"session_id"`
	TurnID               string             `json:"turn_id"`
	TargetID             string             `json:"target_id"`
	RequestedTargetID    string             `json:"requested_target_id"`
	ObservedTargetID     string             `json:"observed_target_id"`
	ResumePhase          string             `json:"resume_phase"`
	Prompt               string             `json:"prompt"`
	Error                string             `json:"error"`
	LastAssistantMessage string             `json:"last_assistant_message"`
	StatusLine           string             `json:"status_line"`
	TranscriptPath       string             `json:"transcript_path"`
	Tool                 *SubsystemTool     `json:"tool,omitempty"`
	Approval             *SubsystemApproval `json:"approval,omitempty"`
	Plan                 *SubsystemPlan     `json:"plan,omitempty"`
	Diff                 *SubsystemDiff     `json:"diff,omitempty"`
	Message              *SubsystemMessage  `json:"message,omitempty"`
}

type DEvSubsystem struct {
	Source    SubsystemKind
	Kind      SubsystemEventKind
	Timestamp time.Time
	Payload   SubsystemPayload
}

func (DEvSubsystem) isDriverEvent() {}

// DEvTick is the periodic tick. Active reflects whether this session is
// currently shown in pane 0.0 — drivers use it to gate expensive work
// that only matters when the user is looking. PaneTarget is the tmux
// pane id (e.g. "%5").
// N and Seq are used for bucketing: drivers gate periodic work to ticks
// where (N+Seq)%interval==0, so sessions are spread across different
// ticks rather than all firing simultaneously.
type DEvTick struct {
	Now        time.Time
	Active     bool
	Project    string
	PaneTarget string
	N          uint64 // monotonic tick counter from EvTick.N
	Seq        uint64 // position of this session in sorted order (0-indexed)
}

func (DEvTick) isDriverEvent() {}

// DEvJobResult delivers an async worker pool result back to the driver
// that requested it. Result is typed by the worker (the driver dispatches
// on its concrete type) and Err is non-nil when the job failed. Now is
// the time the result is being applied; drivers use it to stamp
// StatusInfo / Activity rather than reading wall-clock from inside Step.
type DEvJobResult struct {
	Result any
	Err    error
	Now    time.Time
}

func (DEvJobResult) isDriverEvent() {}

// DEvFileChanged is fired by the runtime fsnotify watcher when a
// session's watched file changes on disk. Drivers typically respond
// by emitting EffStartJob{JobTranscriptParse}.
type DEvFileChanged struct {
	Path string
}

func (DEvFileChanged) isDriverEvent() {}

// DEvPaneOsc delivers a parsed OSC sequence from the PaneTap byte stream to
// the driver. Only OSC 0/2 (window title) is routed here; OSC 9/99/777 go
// directly to EffRecordNotification in the state reducer instead. The driver
// interprets the title string and may update its status accordingly.
type DEvPaneOsc struct {
	Cmd   int
	Title string
	Body  string
	Now   time.Time
}

func (DEvPaneOsc) isDriverEvent() {}

// DEvPanePrompt delivers an OSC 133 semantic-prompt event to the driver.
// ExitCode is non-nil only for PromptPhaseComplete (133;D;<exit-code>).
type DEvPanePrompt struct {
	Phase    PromptPhase
	ExitCode *int
	Now      time.Time
}

func (DEvPanePrompt) isDriverEvent() {}

// DEvStatusLineClick is fired when the user clicks the tmux status bar
// (bound to MouseDown1Status in the root key table). Range is the
// tmux #{mouse_status_range} value — the name registered via
// #[range=user|<name>] in the driver's StatusLine format string.
// An empty Range means the click landed outside any named region.
type DEvStatusLineClick struct {
	Range string
	Now   time.Time
}

func (DEvStatusLineClick) isDriverEvent() {}

// DEvWorktreeResolved is delivered to the driver after the subsystem's
// BindFrame has created a managed worktree for this frame. StartDir is
// the worktree directory; Name is the petname used for the worktree.
// Drivers update their persisted StartDir/WorktreeName so cold-start
// PrepareLaunch can reconstruct the same directory without re-creating.
type DEvWorktreeResolved struct {
	StartDir string
	Name     string
}

func (DEvWorktreeResolved) isDriverEvent() {}

// ViewProvider is an optional capability for drivers that provide a
// custom TUI view.
type ViewProvider interface {
	// View is a pure getter for the current TUI payload. Same View
	// that Step returns, but callable without an event — used by the
	// runtime when serializing SessionInfo for broadcasts and when
	// flushing the active session's status line to tmux.
	View(s DriverState) View
}

// Persister is an optional capability for drivers that support
// session persistence across daemon restarts.
type Persister interface {
	// Persist serializes the driver state into a JSON-friendly map for
	// sessions.json. The reverse is Restore.
	Persist(s DriverState) map[string]string

	// Restore deserializes the persisted bag back into a DriverState.
	// Empty / unknown bags must return a usable zero-state value.
	Restore(bag map[string]string, now time.Time) DriverState
}

type LaunchMode int

const (
	LaunchModeCreate LaunchMode = iota
	LaunchModeColdStart
	LaunchModeWarmStart
)

type WorktreeOption struct {
	Enabled bool `json:"enabled,omitempty"`
}

// SandboxOverride selects the sandbox mode for a session. It is set once at
// session creation and applies to all frames in the session.
type SandboxOverride int

const (
	SandboxOverrideAuto SandboxOverride = iota // follow project config
	SandboxOverrideHost                        // force direct (host) launch
)

type LaunchOptions struct {
	Worktree     WorktreeOption `json:"worktree,omitempty"`
	InitialInput []byte         `json:"initial_input,omitempty"`
}

type LaunchSubsystem string

const (
	LaunchSubsystemCLI    LaunchSubsystem = "cli"
	LaunchSubsystemStream LaunchSubsystem = "stream"
)

type StreamSandboxPolicy string

const (
	StreamSandboxPolicyDefault  StreamSandboxPolicy = ""
	StreamSandboxPolicyExternal StreamSandboxPolicy = "external"
)

type StreamApprovalPolicy string

const (
	StreamApprovalPolicyDefault     StreamApprovalPolicy = ""
	StreamApprovalPolicyAutoApprove StreamApprovalPolicy = "auto_approve"
)

type StreamLaunchOptions struct {
	SandboxPolicy  StreamSandboxPolicy  `json:"sandbox_policy,omitempty"`
	ApprovalPolicy StreamApprovalPolicy `json:"approval_policy,omitempty"`
	ResumeThreadID string               `json:"resume_thread_id,omitempty"`
}

type LaunchPlan struct {
	Command   string
	StartDir  string
	Project   string          // canonical project root passed opaquely to the sandbox launcher
	Sandbox   SandboxOverride // session-level sandbox mode, written by reducer before dispatch
	Options   LaunchOptions
	Subsystem LaunchSubsystem
	Stream    StreamLaunchOptions
	Stdin     []byte // content piped into the spawned command; nil = no stdin
}

type LaunchPreparer interface {
	PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string, options LaunchOptions, sandboxed bool) (LaunchPlan, error)
}

// Driver is the interface every per-driver-type plugin implements. Each
// impl is a stateless value type registered once at init time; the
// per-session state lives in DriverState values returned by NewState.
type Driver interface {
	// Name is the registry key (e.g. "mydriver").
	Name() string

	// DisplayName is the human-readable label shown in card / palette.
	DisplayName() string

	// NewState constructs a fresh DriverState for a brand-new session.
	// Initial status, idle counters, etc. live here.
	NewState(now time.Time) DriverState

	// Step is the per-driver reducer. It must be a pure function: no
	// I/O, no goroutines, no globals (other than the registry). All
	// side effects are returned as []Effect for the runtime to execute.
	Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View)

	// Status returns the current driver status without building the
	// full View. Used by the tick reducer to skip idle/stopped sessions.
	Status(s DriverState) Status

	ViewProvider
	Persister
	LaunchPreparer
}

// CreateLaunch is the fully resolved process launch information for a
// newly created session: command string plus tmux start directory.
type CreateLaunch struct {
	Command  string
	StartDir string
	Options  LaunchOptions
}

// CreateSessionPlanner is an optional driver extension for commands
// that need to transform or prepare their start environment during
// create-session before tmux spawn happens. The subsystem resolves any
// deferred setup (e.g. worktree creation) during BindFrame; drivers
// only strip tool-specific flags and set LaunchOptions.
type CreateSessionPlanner interface {
	PrepareCreate(s DriverState, sessionID SessionID, project, command string, options LaunchOptions) (DriverState, CreateLaunch, error)
}

// Forkable is an optional driver extension for drivers whose CLI supports
// forking the current conversation into a new independent branch.
// ForkCommand returns the full launch command for the forked session derived
// from s and the root frame's baseCommand. Returns ok=false when the driver
// state does not yet have enough identity to fork (e.g. no session ID).
type Forkable interface {
	ForkCommand(s DriverState, baseCommand string) (command string, ok bool)
}

// WarmStartRecoverer is an optional driver extension for restoring
// driver-owned runtime state after a warm start. Drivers use this to
// re-install watches and resume async parsing from already-restored
// DriverState without the runtime inspecting driver-specific fields.
type WarmStartRecoverer interface {
	WarmStartRecover(s DriverState, now time.Time) (DriverState, []Effect)
}

// SessionBootstrapper is an optional driver capability for running
// post-spawn initialization when a brand-new root frame has been
// created, but the external agent has not yet emitted its own startup
// hook.
type SessionBootstrapper interface {
	BootstrapSessionStart(s DriverState, ctx FrameContext, now time.Time) (DriverState, []Effect)
}

// StartDirAware is an optional driver extension that lets the state
// layer read and write the session's working directory without
// inspecting driver-specific concrete types. Used by reducePushDriver
// to inherit the root frame's directory into a new child frame.
type StartDirAware interface {
	// StartDir returns the working directory stored in the given DriverState.
	StartDir(s DriverState) string
	// WithStartDir returns a copy of s with the working directory set to dir.
	WithStartDir(s DriverState, dir string) DriverState
}

// driver registry and default-driver factory. Set once at init time by each
// driver impl package.
var (
	driverRegistry = make(map[string]Driver)
	defaultFactory func(command string) Driver
)

// Register adds a Driver to the registry under its Name(). Called from
// init() in each driver impl package. Panics on duplicate names so the
// daemon fails fast at startup if two impls collide.
func Register(d Driver) {
	if _, exists := driverRegistry[d.Name()]; exists {
		panic("state: duplicate driver registration: " + d.Name())
	}
	driverRegistry[d.Name()] = d
}

// RegisterDefaultFactory installs a factory used by GetDriver when
// the command does not match any registered driver. The factory
// receives the raw command string and returns a fresh Driver instance.
// This is distinct from the "No fallbacks" principle (which prohibits
// synthesising status when a data source is unavailable); this factory
// selects a driver for unrecognised commands, not a substitute status.
func RegisterDefaultFactory(factory func(command string) Driver) {
	defaultFactory = factory
}

// GetRegistry returns the current driver registry. Used for testing.
func GetRegistry() map[string]Driver {
	return driverRegistry
}

// ClearRegistry clears the driver registry and default factory. Used for testing.
func ClearRegistry() {
	driverRegistry = map[string]Driver{}
	defaultFactory = nil
}

// FirstToken extracts the first whitespace-delimited word from a command string.
func FirstToken(command string) string {
	if idx := strings.IndexAny(command, " \t"); idx != -1 {
		return command[:idx]
	}
	return command
}

// GetDriver returns the Driver for the given session command. It first
// tries to resolve the command's first token against the registry. If
// no registered driver matches and a fallback factory is installed, the
// factory is called to build a fresh driver. Otherwise the "" fallback
// driver is returned as the last resort.
func GetDriver(command string) Driver {
	name := FirstToken(command)
	if d, ok := driverRegistry[name]; ok {
		return d
	}
	if defaultFactory != nil {
		return defaultFactory(command)
	}
	return driverRegistry[""]
}
