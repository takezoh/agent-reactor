package proto

// surface.* subscribe / unsubscribe / resize / write_raw — frame surface streaming and input operations.
const (
	CmdNameSurfaceSubscribe   = "surface.subscribe"
	CmdNameSurfaceUnsubscribe = "surface.unsubscribe"
	CmdNameSurfaceResize      = "surface.resize"
	CmdNameSurfaceWriteRaw    = "surface.write_raw"
)

// CmdSurfaceSubscribe begins streaming the head frame surface output of
// SessionID as EvtSurfaceOutput events to the caller.
type CmdSurfaceSubscribe struct {
	SessionID string `json:"session_id"`
}

func (CmdSurfaceSubscribe) isCommand()          {}
func (CmdSurfaceSubscribe) CommandName() string { return CmdNameSurfaceSubscribe }

// CmdSurfaceUnsubscribe cancels an active surface.subscribe for SessionID.
type CmdSurfaceUnsubscribe struct {
	SessionID string `json:"session_id"`
}

func (CmdSurfaceUnsubscribe) isCommand()          {}
func (CmdSurfaceUnsubscribe) CommandName() string { return CmdNameSurfaceUnsubscribe }

// CmdSurfaceResize changes the logical size of the SessionID head frame surface to (Cols, Rows).
type CmdSurfaceResize struct {
	SessionID string `json:"session_id"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

func (CmdSurfaceResize) isCommand()          {}
func (CmdSurfaceResize) CommandName() string { return CmdNameSurfaceResize }

// CmdSurfaceWriteRaw writes raw bytes to the SessionID head frame surface.
// No Enter is appended and key names are not interpreted.
// Data is transmitted as base64 on the wire (encoding/json default behaviour for []byte).
type CmdSurfaceWriteRaw struct {
	SessionID string `json:"session_id"`
	Data      []byte `json:"data"`
}

func (CmdSurfaceWriteRaw) isCommand()          {}
func (CmdSurfaceWriteRaw) CommandName() string { return CmdNameSurfaceWriteRaw }
