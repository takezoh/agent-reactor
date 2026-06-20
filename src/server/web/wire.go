// Package web bridges a browser to arc daemon over WebSocket. wire.go
// encodes daemon-side proto events into the asciicast v2 + control wire
// the browser UI already speaks, and decodes inbound browser frames
// into proto.Command values.
package web

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
)

// outputFrameFromSurface encodes EvtSurfaceOutput as the asciicast v2
// output array: [TimeSec, "o", string(base64.Decode(DataB64))].
func outputFrameFromSurface(e proto.EvtSurfaceOutput) []byte {
	data, _ := base64.StdEncoding.DecodeString(e.DataB64)
	b, _ := json.Marshal([]any{e.TimeSec, "o", string(data)})
	return b
}

// controlMsg is a server→client control event distinct from output by
// being a JSON object rather than the asciicast array.
type controlMsg struct {
	K    string `json:"k"`
	Code int    `json:"code,omitempty"`
	Data string `json:"data,omitempty"`
}

// controlFrame encodes a controlMsg as a JSON text frame.
func controlFrame(kind string, code int, data string) []byte {
	b, _ := json.Marshal(controlMsg{K: kind, Code: code, Data: data})
	return b
}

// viewUpdateFrame is the server→browser frame derived from
// proto.EvtSessionsChanged. ADR 0023: 1:1 mirror.
type viewUpdateFrame struct {
	K               string                `json:"k"` // always "v"
	Sessions        []proto.SessionInfo   `json:"sessions"`
	ActiveSessionID string                `json:"activeSessionID,omitempty"`
	Connectors      []proto.ConnectorInfo `json:"connectors,omitempty"`
}

// encodeFromSessionsChanged encodes EvtSessionsChanged as a view-update
// frame {"k":"v","sessions":[…],"activeSessionID":"…","connectors":[…]}.
// Returns nil on marshal error (gateway drops nil frames).
// nil slices are normalised to empty arrays so the browser codec, which
// requires `sessions` to be an array, never receives `"sessions":null`.
func encodeFromSessionsChanged(ev proto.EvtSessionsChanged) []byte {
	sessions := ev.Sessions
	if sessions == nil {
		sessions = []proto.SessionInfo{}
	}
	f := viewUpdateFrame{
		K:               "v",
		Sessions:        sessions,
		ActiveSessionID: ev.ActiveSessionID,
		Connectors:      ev.Connectors, // omitempty: nil/empty stays out of wire
	}
	b, err := json.Marshal(f)
	if err != nil {
		slog.Error("wire: failed to encode view-update frame", "err", err)
		return nil
	}
	return b
}

// encodeServerEvent renders a proto.ServerEvent as one WebSocket text frame.
// Returns nil for events the browser does not need (the gateway drops nil).
func encodeServerEvent(ev proto.ServerEvent) []byte {
	switch e := ev.(type) {
	case proto.EvtSurfaceOutput:
		return outputFrameFromSurface(e)
	case proto.EvtSessionFileLine:
		return encodeFromSessionFileLine(e)
	case proto.EvtAgentNotification:
		return encodeFromAgentNotification(e)
	case proto.EvtSessionsChanged:
		return encodeFromSessionsChanged(e)
	}
	return nil
}

// transcriptTailFrame is the server→browser frame for a transcript tail line.
type transcriptTailFrame struct {
	K         string `json:"k"`
	SessionID string `json:"sessionId"`
	Line      string `json:"line"`
}

// eventLogTailFrame is the server→browser frame for an event-log tail line.
type eventLogTailFrame struct {
	K         string `json:"k"`
	SessionID string `json:"sessionId"`
	Line      string `json:"line"`
}

// encodeFromSessionFileLine encodes EvtSessionFileLine as a tail frame.
// Kind "transcript" → {"k":"tt",...}; Kind "event-log" → {"k":"et",...}.
// Unknown Kind returns nil (gateway drops nil frames).
// JSON marshal failure logs an error and returns nil.
func encodeFromSessionFileLine(e proto.EvtSessionFileLine) []byte {
	switch e.Kind {
	case "transcript":
		b, err := json.Marshal(transcriptTailFrame{K: "tt", SessionID: e.SessionID, Line: e.Line})
		if err != nil {
			slog.Error("wire: failed to encode transcript-tail frame", "err", err)
			return nil
		}
		return b
	case "event-log":
		b, err := json.Marshal(eventLogTailFrame{K: "et", SessionID: e.SessionID, Line: e.Line})
		if err != nil {
			slog.Error("wire: failed to encode event-log-tail frame", "err", err)
			return nil
		}
		return b
	default:
		return nil
	}
}

// notificationFrame is the server→browser frame for an agent notification.
type notificationFrame struct {
	K         string `json:"k"`
	SessionID string `json:"sessionId"`
	Cmd       int    `json:"cmd"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	NowMs     int64  `json:"nowMs"`
}

// encodeFromAgentNotification encodes EvtAgentNotification as
// {"k":"n","sessionId":...,"cmd":...,"title":...,"body":...,"nowMs":...}.
// JSON marshal failure logs an error and returns nil.
func encodeFromAgentNotification(e proto.EvtAgentNotification) []byte {
	b, err := json.Marshal(notificationFrame{
		K:         "n",
		SessionID: e.SessionID,
		Cmd:       e.Cmd,
		Title:     e.Title,
		Body:      e.Body,
		NowMs:     time.Now().UnixMilli(),
	})
	if err != nil {
		slog.Error("wire: failed to encode notification frame", "err", err)
		return nil
	}
	return b
}

// inbound is a browser→server message (always a JSON object). It carries the
// union of fields used by per-session surface frames (AttachWS path: "i", "r")
// and lifecycle-multiplexed frames (AttachLifecycleWS path: "s" subscribe,
// "u" unsubscribe, "i"/"r" with sessionId). Unused fields decode to zero.
type inbound struct {
	K         string `json:"k"` // "i" input | "r" resize | "s" subscribe | "u" unsubscribe
	D         string `json:"d"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	SessionID string `json:"sessionId,omitempty"`
	ReqID     string `json:"reqId,omitempty"`
}
