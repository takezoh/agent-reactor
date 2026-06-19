// Package web bridges a browser to arc daemon over WebSocket. wire.go
// encodes daemon-side proto events into the asciicast v2 + control wire
// the browser UI already speaks, and decodes inbound browser frames
// into proto.Command values.
package web

import (
	"encoding/base64"
	"encoding/json"

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

// controlFrameFromNotification encodes EvtAgentNotification as
// {"k":"osc","code":<Cmd>,"data":"<Title>|<Body>"} (existing UI 互換).
func controlFrameFromNotification(e proto.EvtAgentNotification) []byte {
	data := e.Title
	if e.Body != "" {
		if data != "" {
			data += " | "
		}
		data += e.Body
	}
	b, _ := json.Marshal(controlMsg{K: "osc", Code: e.Cmd, Data: data})
	return b
}

// encodeServerEvent renders a proto.ServerEvent as one WebSocket text frame.
// Returns nil for events the browser does not need (the gateway drops nil).
func encodeServerEvent(ev proto.ServerEvent) []byte {
	switch e := ev.(type) {
	case proto.EvtSurfaceOutput:
		return outputFrameFromSurface(e)
	case proto.EvtAgentNotification:
		return controlFrameFromNotification(e)
	}
	return nil
}

// inbound is a browser→server message (always a JSON object).
type inbound struct {
	K    string `json:"k"` // "i" input | "r" resize
	D    string `json:"d"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}
