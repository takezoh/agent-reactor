package web

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-reactor/client/proto"
)

func TestWireEncodeServerEvent_SurfaceOutput(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hi"))
	ev := proto.EvtSurfaceOutput{
		SessionID: "s1",
		TimeSec:   1.5,
		DataB64:   encoded,
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame for EvtSurfaceOutput")
	}

	var arr []any
	if err := json.Unmarshal(got, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3-element array, got %d", len(arr))
	}
	if arr[0].(float64) != 1.5 {
		t.Errorf("time: got %v, want 1.5", arr[0])
	}
	if arr[1].(string) != "o" {
		t.Errorf("type: got %v, want \"o\"", arr[1])
	}
	if arr[2].(string) != "hi" {
		t.Errorf("data: got %v, want \"hi\"", arr[2])
	}
}

func TestWireEncodeServerEvent_AgentNotification(t *testing.T) {
	ev := proto.EvtAgentNotification{
		SessionID: "s1",
		Cmd:       9,
		Title:     "t",
		Body:      "b",
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame for EvtAgentNotification")
	}

	var msg controlMsg
	if err := json.Unmarshal(got, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := controlMsg{K: "osc", Code: 9, Data: "t | b"}
	if msg != want {
		t.Errorf("got %+v, want %+v", msg, want)
	}
}

func TestWireEncodeServerEvent_NotificationTitleOnly(t *testing.T) {
	ev := proto.EvtAgentNotification{
		SessionID: "s1",
		Cmd:       9,
		Title:     "t",
		Body:      "",
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame")
	}

	var msg controlMsg
	if err := json.Unmarshal(got, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Data != "t" {
		t.Errorf("got Data=%q, want %q", msg.Data, "t")
	}
}

func TestWireEncodeServerEvent_UnknownEventReturnsNil(t *testing.T) {
	ev := proto.EvtSessionsChanged{}
	got := encodeServerEvent(ev)
	if got != nil {
		t.Errorf("expected nil for unknown event, got %s", got)
	}
}

func TestWireEncodeServerEvent_InvalidBase64IgnoresGracefully(t *testing.T) {
	ev := proto.EvtSurfaceOutput{
		SessionID: "s1",
		TimeSec:   0.0,
		DataB64:   "!!!not-valid-base64!!!",
	}
	// Must not panic; result should still be a valid JSON array.
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame even with invalid base64")
	}
	var arr []any
	if err := json.Unmarshal(got, &arr); err != nil {
		t.Fatalf("result should be valid JSON even with bad base64: %v", err)
	}
}
