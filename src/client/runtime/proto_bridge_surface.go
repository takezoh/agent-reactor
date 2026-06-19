package runtime

import (
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

// broadcastSurfaceOutput fans a chunk of pane output to all ConnIDs subscribed
// to e.SessionID via State.SurfaceSubs. Each ConnID gets its own EvtSurfaceOutput
// message so the per-subscriber Sequence is handled by TerminalRelay (the
// internalBroadcastSurface path). This function is used when the effect comes
// from the reducer side (EffBroadcastSurfaceOutput).
func (r *Runtime) broadcastSurfaceOutput(e state.EffBroadcastSurfaceOutput) {
	ev := proto.EvtSurfaceOutput{
		SessionID: string(e.SessionID),
		TimeSec:   e.TimeSec,
		DataB64:   base64.StdEncoding.EncodeToString(e.Data),
		Sequence:  0, // sequence managed by TerminalRelay fan-out goroutines
	}
	wire, err := proto.EncodeEvent(ev)
	if err != nil {
		slog.Error("runtime: encode surface output failed", "err", err)
		return
	}
	// Iterate outer map (ConnID → set of SessionIDs) to find which connections
	// are subscribed to this session.
	for connID, sessions := range r.state.SurfaceSubs {
		if _, ok := sessions[e.SessionID]; !ok {
			continue
		}
		r.queueWireToConn(connID, wire, proto.EvtNameSurfaceOutput)
	}
}

// broadcastSurfaceFromInternal delivers a single internalBroadcastSurface to
// exactly one ConnID's outbox. Called from dispatchInternal when the TerminalRelay
// fan-out goroutine enqueues a new chunk.
func (r *Runtime) broadcastSurfaceFromInternal(ev internalBroadcastSurface) {
	out := proto.EvtSurfaceOutput{
		SessionID: string(ev.SessionID),
		TimeSec:   ev.TimeSec,
		DataB64:   base64.StdEncoding.EncodeToString(ev.Data),
		Sequence:  ev.Sequence,
	}
	wire, err := proto.EncodeEvent(out)
	if err != nil {
		slog.Error("runtime: encode surface output (internal) failed", "err", err)
		return
	}
	r.queueWireToConn(ev.ConnID, wire, proto.EvtNameSurfaceOutput)
}

// broadcastPromptEvent delivers EvtPromptEvent to all ConnIDs subscribed to
// the session that owns e.FrameID. FrameID → SessionID is resolved by scanning
// state.Sessions.
func (r *Runtime) broadcastPromptEvent(e state.EffBroadcastPromptEvent) {
	sessionID := r.sessionIDForFrame(e.FrameID)
	if sessionID == "" {
		slog.Warn("runtime: prompt event: no session for frame", "frame", e.FrameID)
		return
	}
	ev := proto.EvtPromptEvent{
		FrameID:  string(e.FrameID),
		Phase:    e.Phase,
		ExitCode: e.ExitCode,
		NowRFC:   time.Now().UTC().Format(time.RFC3339),
	}
	wire, err := proto.EncodeEvent(ev)
	if err != nil {
		slog.Error("runtime: encode prompt event failed", "err", err)
		return
	}
	for connID, sessions := range r.state.SurfaceSubs {
		if _, ok := sessions[sessionID]; !ok {
			continue
		}
		r.queueWireToConn(connID, wire, proto.EvtNamePromptEvent)
	}
}

// sessionIDForFrame resolves the SessionID that owns frameID by scanning state.Sessions.
// Returns "" if no session contains the frame.
func (r *Runtime) sessionIDForFrame(frameID state.FrameID) state.SessionID {
	for sessID, sess := range r.state.Sessions {
		for _, fr := range sess.Frames {
			if fr.ID == frameID {
				return sessID
			}
		}
	}
	return ""
}

// queueWireToConn enqueues raw wire bytes on a specific ConnID's outbox.
// Non-blocking: drops with a warning if the outbox is full or the conn is gone.
func (r *Runtime) queueWireToConn(connID state.ConnID, wire []byte, eventName string) {
	cc, ok := r.conns[connID]
	if !ok {
		return
	}
	select {
	case cc.outbox <- wire:
	case <-cc.done:
	default:
		slog.Warn("runtime: conn outbox full, dropping surface event",
			"conn", connID, "event", eventName)
	}
}
