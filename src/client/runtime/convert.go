package runtime

import (
	"encoding/json"

	"github.com/takezoh/agent-roost/client/proto"
	"github.com/takezoh/agent-roost/client/state"
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
	case proto.CmdSurfaceReadText:
		return state.EvCmdSurfaceReadText{
			ConnID:    connID,
			ReqID:     reqID,
			SessionID: state.SessionID(c.SessionID),
			Lines:     c.Lines,
		}
	case proto.CmdSurfaceSendText:
		return state.EvCmdSurfaceSendText{
			ConnID:    connID,
			ReqID:     reqID,
			SessionID: state.SessionID(c.SessionID),
			Text:      c.Text,
		}
	case proto.CmdSurfaceSendKey:
		return state.EvCmdSurfaceSendKey{
			ConnID:    connID,
			ReqID:     reqID,
			SessionID: state.SessionID(c.SessionID),
			Key:       c.Key,
		}
	case proto.CmdDriverList:
		return state.EvCmdDriverList{ConnID: connID, ReqID: reqID}
	case proto.CmdPeerSend:
		return peerEvEvent(connID, reqID, state.EventPeerSend, state.PeerSendParams{
			ToFrameID:   c.ToFrameID,
			FromFrameID: c.FromFrameID,
			Text:        c.Text,
			ReplyTo:     c.ReplyTo,
		})
	case proto.CmdPeerList:
		return peerEvEvent(connID, reqID, state.EventPeerList, state.PeerListParams{
			Scope:       c.Scope,
			FromFrameID: c.FromFrameID,
		})
	case proto.CmdPeerSetSummary:
		return peerEvEvent(connID, reqID, state.EventPeerSetSummary, state.PeerSetSummaryParams{
			Summary:     c.Summary,
			FromFrameID: c.FromFrameID,
		})
	case proto.CmdPeerDrainInbox:
		return peerEvEvent(connID, reqID, state.EventPeerDrainInbox, state.PeerDrainInboxParams{
			FromFrameID: c.FromFrameID,
		})
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

// peerEvEvent marshals a peer params struct and wraps it in state.EvEvent
// for dispatch via state.eventHandlers.
func peerEvEvent(connID state.ConnID, reqID, eventName string, params any) state.Event {
	b, _ := json.Marshal(params)
	return state.EvEvent{ConnID: connID, ReqID: reqID, Event: eventName, Payload: b}
}
