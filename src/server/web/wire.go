// Package web bridges a browser to the server daemon over WebSocket. wire.go
// encodes daemon-side proto events into the asciicast v2 + control wire
// the browser UI already speaks, and decodes inbound browser frames
// into proto.Command values.
package web

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
)

// outputFrameFromSurface encodes EvtSurfaceOutput as an asciicast-style
// output array extended with the originating SessionID:
// [TimeSec, "o", DataB64, SessionID].
//
// SessionID routing: AttachLifecycleWS multiplexes surface output for every
// session the browser has subscribed to, so the browser cannot tell which
// session each output frame belongs to from element [0..2] alone. The 4th
// element carries the daemon-side SessionID; TerminalPane drops frames
// whose SessionID does not match its active session. Without this, the
// brief overlap during a session switch (subscribe is async, unsubscribe
// is fire-and-forget) leaks output from the previous session into the
// newly-mounted terminal — see round-4 code-review finding.
//
// Wire-binary safety: the third element is the daemon-side base64 STRING
// (NOT the decoded bytes). Decoding to a Go string and JSON-marshalling
// raw PTY bytes is unsafe — encoding/json silently replaces any non-UTF-8
// byte with U+FFFD, garbling 256-color sequences, raw binary, and most
// non-ASCII output. Keeping the wire payload base64-encoded means the
// browser (TerminalPane.tsx) atobs it back to raw bytes and feeds the
// resulting Uint8Array directly to xterm.write — byte-faithful end to end.
// Also avoids the prior encode→decode→encode round-trip from the daemon
// runtime down to the browser (round-3 finding: gratuitous base64 churn).
func outputFrameFromSurface(e proto.EvtSurfaceOutput) []byte {
	b, _ := json.Marshal([]any{e.TimeSec, "o", e.DataB64, e.SessionID})
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
// proto.EvtSessionsChanged.
//
// ActiveSessionID is deliberately NOT mirrored on view-update frames (cf.
// ADR 0023 which described an early 1:1 mirror). The daemon's ActiveSession
// is mutated whenever a frame is spawned or pushed (reducePaneSpawned
// and friends) and is broadcast to every connected web client; passing it
// through here would clobber each browser's locally-tracked selection
// every time another session in the same daemon emits an event — a
// background session creation or driver frame push would yank the active
// tab away from the user. Web clients own their own selection (SessionList
// click + CreateSessionForm post-create selectSession) and only need the
// daemon's notion of activeSessionID as an initial seed via the hello
// frame.
type viewUpdateFrame struct {
	K        string              `json:"k"` // always "v"
	Sessions []proto.SessionInfo `json:"sessions"`
}

// encodeFromSessionsChanged encodes EvtSessionsChanged as a view-update
// frame {"k":"v","sessions":[…]}.
// Returns nil on marshal error (gateway drops nil frames).
// nil slices are normalised to empty arrays so the browser codec, which
// requires `sessions` to be an array, never receives `"sessions":null`.
//
// ev.ActiveSessionID is intentionally dropped — see viewUpdateFrame doc.
func encodeFromSessionsChanged(ev proto.EvtSessionsChanged) []byte {
	sessions := ev.Sessions
	if sessions == nil {
		sessions = []proto.SessionInfo{}
	}
	f := viewUpdateFrame{
		K:        "v",
		Sessions: sessions,
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
