package runtime

import (
	"encoding/json"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

func commandToStateEvent(connID state.ConnID, reqID string, cmd proto.Command) state.Event {
	switch c := cmd.(type) {
	case proto.CmdSubscribe:
		return state.EvCmdSubscribe{ConnID: connID, ReqID: reqID, Filters: c.Filters}
	case proto.CmdUnsubscribe:
		return state.EvCmdUnsubscribe{ConnID: connID, ReqID: reqID}
	case proto.CmdEvent:
		if state.IsRegisteredEvent(c.Event) {
			return state.EvEvent{
				ConnID:  connID,
				ReqID:   reqID,
				Event:   c.Event,
				Payload: c.Payload,
			}
		}
		return state.EvDriverEvent{
			ConnID:    connID,
			ReqID:     reqID,
			Event:     c.Event,
			Timestamp: c.Timestamp,
			SenderID:  state.FrameID(c.SenderID),
			Payload:   c.Payload,
		}
	case proto.CmdSubsystemEvent:
		return state.EvSubsystem{
			ConnID:    connID,
			ReqID:     reqID,
			FrameID:   state.FrameID(c.FrameID),
			Source:    state.SubsystemKind(c.Source),
			Kind:      state.SubsystemEventKind(c.Kind),
			Timestamp: c.Timestamp,
			Payload:   decodeSubsystemPayload(c.Payload),
		}
	case proto.CmdDriverList:
		return state.EvCmdDriverList{ConnID: connID, ReqID: reqID}
	}
	if ev := surfaceCommandToEvent(connID, reqID, cmd); ev != nil {
		return ev
	}
	return nil
}

// surfaceCommandToEvent translates surface.* commands into their state events.
// Returns nil if cmd is not a surface command.
func surfaceCommandToEvent(connID state.ConnID, reqID string, cmd proto.Command) state.Event {
	switch c := cmd.(type) {
	case proto.CmdSurfaceReadText:
		return state.EvCmdSurfaceReadText{ConnID: connID, ReqID: reqID, SessionID: state.SessionID(c.SessionID), Lines: c.Lines}
	case proto.CmdSurfaceSendText:
		return state.EvCmdSurfaceSendText{ConnID: connID, ReqID: reqID, SessionID: state.SessionID(c.SessionID), Text: c.Text}
	case proto.CmdSurfaceSendKey:
		return state.EvCmdSurfaceSendKey{ConnID: connID, ReqID: reqID, SessionID: state.SessionID(c.SessionID), Key: c.Key}
	case proto.CmdSurfaceSubscribe:
		return state.EvCmdSurfaceSubscribe{ConnID: connID, ReqID: reqID, SessionID: state.SessionID(c.SessionID)}
	case proto.CmdSurfaceUnsubscribe:
		return state.EvCmdSurfaceUnsubscribe{ConnID: connID, ReqID: reqID, SessionID: state.SessionID(c.SessionID)}
	case proto.CmdSurfaceResize:
		return state.EvCmdSurfaceResize{ConnID: connID, ReqID: reqID, SessionID: state.SessionID(c.SessionID), Cols: c.Cols, Rows: c.Rows}
	case proto.CmdSurfaceWriteRaw:
		return state.EvCmdSurfaceWriteRaw{ConnID: connID, ReqID: reqID, SessionID: state.SessionID(c.SessionID), Data: c.Data}
	}
	return nil
}

func decodeSubsystemPayload(raw json.RawMessage) state.SubsystemPayload {
	if len(raw) == 0 {
		return state.SubsystemPayload{}
	}
	var payload state.SubsystemPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return state.SubsystemPayload{}
	}
	return payload
}
