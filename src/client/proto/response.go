package proto

import (
	"time"

	stateview "github.com/takezoh/agent-reactor/client/state/view"
)

// Response is the closed sum type of every successful reply the
// daemon sends back to a request. Errors are not Responses — they
// are encoded directly into Envelope.Error / Envelope.Status as
// ErrorBody, since they have a uniform shape.
type Response interface {
	isResponse()
}

// RespOK is the empty success response. Used by commands that have
// nothing to return except "operation accepted" (stop-session,
// detach, ...).
type RespOK struct{}

func (RespOK) isResponse() {}

// RespCreateSession is the response to create-session. The runtime
// fills it in after the frame spawn callback completes.
type RespCreateSession struct {
	SessionID string `json:"session_id"`
}

func (RespCreateSession) isResponse() {}

// RespSessions is the response to list-sessions and the body of
// EvtSessionsChanged. Carries the full session table.
type RespSessions struct {
	Sessions []SessionInfo `json:"sessions"`
	Features []string      `json:"features,omitempty"`
}

func (RespSessions) isResponse() {}

// SessionInfo is the per-session payload shipped on the wire. Mirrors
// state.Session + the driver's View output. Carried inside
// RespSessions and EvtSessionsChanged. State and StateChangedAt are
// duplicated from View.Status / View.StatusChangedAt for client-side
// convenience (the TUI renders status colors and elapsed time
// without unwrapping the View).
type SessionInfo struct {
	ID                 string           `json:"id"`
	Project            string           `json:"project"`
	Workspace          string           `json:"workspace,omitempty"`
	Command            string           `json:"command"`
	RootDriver         string           `json:"root_driver,omitempty"`
	RootDriverForkable bool             `json:"root_driver_forkable,omitempty"`
	CreatedAt          string           `json:"created_at"`
	State              stateview.Status `json:"state,omitempty"`
	StateChangedAt     string           `json:"state_changed_at,omitempty"`
	View               stateview.View   `json:"view"`
	Frames             []FrameInfo      `json:"frames,omitempty"`
	HeadFrameID        string           `json:"head_frame_id,omitempty"`
}

// FrameInfo is the per-frame wire payload for header tab rendering.
type FrameInfo struct {
	ID          string `json:"id"`
	Command     string `json:"command"`
	SubsystemID string `json:"subsystem_id,omitempty"`
	TargetID    string `json:"target_id,omitempty"`
}

// Name returns the display name for the session (basename of project).
func (si SessionInfo) Name() string {
	return baseName(si.Project)
}

// DisplayCommand returns the command string or "idle" when empty.
func (si SessionInfo) DisplayCommand() string {
	if si.Command != "" {
		return si.Command
	}
	return "idle"
}

// CreatedAtTime parses the on-the-wire CreatedAt string.
func (si SessionInfo) CreatedAtTime() time.Time {
	t, _ := time.Parse(time.RFC3339, si.CreatedAt)
	return t
}

// StateChangedAtTime parses StateChangedAt, falling back to CreatedAt
// when the state has not been touched yet.
func (si SessionInfo) StateChangedAtTime() time.Time {
	if si.StateChangedAt == "" {
		return si.CreatedAtTime()
	}
	t, _ := time.Parse(time.RFC3339, si.StateChangedAt)
	return t
}

// RespSurfaceText is the response to surface.read_text.
type RespSurfaceText struct {
	Text string `json:"text"`
}

func (RespSurfaceText) isResponse() {}

// RespDriverList is the response to driver.list.
type RespDriverList struct {
	Drivers []DriverInfo `json:"drivers"`
}

func (RespDriverList) isResponse() {}

// DriverInfo is the per-driver payload in RespDriverList.
type DriverInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// baseName mirrors filepath.Base without importing filepath, so the
// proto package stays trim. Handles both "/" and OS-native separators.
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
