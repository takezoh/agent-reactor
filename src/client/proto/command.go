package proto

import (
	"encoding/json"
	"time"
)

// Command is the closed sum type of every IPC request the daemon
// accepts. Each impl carries the typed args + a Name() string that
// matches the wire "cmd" field.
type Command interface {
	isCommand()
	CommandName() string
}

// Command name constants — used by both Encode and Decode so a typo
// breaks both ends symmetrically.
const (
	CmdNameSubscribe   = "subscribe"
	CmdNameUnsubscribe = "unsubscribe"
	CmdNameEvent       = "event"
	CmdNameSubsystem   = "subsystem-event"

	// surface.* — pane read/write operations
	CmdNameSurfaceReadText = "surface.read_text"
	CmdNameSurfaceSendText = "surface.send_text"
	CmdNameSurfaceSendKey  = "surface.send_key"

	// driver.* — driver registry queries
	CmdNameDriverList = "driver.list"

	// hook-event — container endpoint only. Carries a driver hook notification
	// with a bearer token that resolves to the spawning frame. Not accepted on
	// the host endpoint.
	CmdNameHookEvent = "hook-event"
)

type CmdSubscribe struct {
	Filters []string `json:"filters,omitempty"`
}

func (CmdSubscribe) isCommand()          {}
func (CmdSubscribe) CommandName() string { return CmdNameSubscribe }

type CmdUnsubscribe struct{}

func (CmdUnsubscribe) isCommand()          {}
func (CmdUnsubscribe) CommandName() string { return CmdNameUnsubscribe }

// CmdEvent is the generic event envelope sent by the `server event` CLI.
type CmdEvent struct {
	Event     string          `json:"event"`
	Timestamp time.Time       `json:"timestamp"`
	SenderID  string          `json:"sender_id"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (CmdEvent) isCommand()          {}
func (CmdEvent) CommandName() string { return CmdNameEvent }

type CmdSubsystemEvent struct {
	Token     string          `json:"token,omitempty"`
	FrameID   string          `json:"frame_id,omitempty"`
	Source    string          `json:"source"`
	Kind      string          `json:"kind"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (CmdSubsystemEvent) isCommand()          {}
func (CmdSubsystemEvent) CommandName() string { return CmdNameSubsystem }

// CmdSurfaceReadText reads the trailing Lines of a session's pane content.
// SessionID identifies the target session; Lines=0 uses the server default (30).
type CmdSurfaceReadText struct {
	SessionID string `json:"session_id"`
	Lines     int    `json:"lines,omitempty"`
}

func (CmdSurfaceReadText) isCommand()          {}
func (CmdSurfaceReadText) CommandName() string { return CmdNameSurfaceReadText }

// CmdSurfaceSendText sends Text followed by Enter to a session's active pane.
type CmdSurfaceSendText struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

func (CmdSurfaceSendText) isCommand()          {}
func (CmdSurfaceSendText) CommandName() string { return CmdNameSurfaceSendText }

// CmdSurfaceSendKey sends a named key (e.g. "Escape", "C-c") to a session's
// active pane without appending Enter.
type CmdSurfaceSendKey struct {
	SessionID string `json:"session_id"`
	Key       string `json:"key"`
}

func (CmdSurfaceSendKey) isCommand()          {}
func (CmdSurfaceSendKey) CommandName() string { return CmdNameSurfaceSendKey }

// CmdDriverList lists all registered driver names and display names.
type CmdDriverList struct{}

func (CmdDriverList) isCommand()          {}
func (CmdDriverList) CommandName() string { return CmdNameDriverList }

// CmdHookEvent is the container-only command that delivers a driver hook
// notification. Token authenticates the sender and resolves to the FrameID
// of the spawning frame.
type CmdHookEvent struct {
	Token     string          `json:"token"`
	Hook      string          `json:"hook"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (CmdHookEvent) isCommand()          {}
func (CmdHookEvent) CommandName() string { return CmdNameHookEvent }
