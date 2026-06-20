package web

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state/view"
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
	ev := proto.EvtProjectSelected{Project: "x"}
	got := encodeServerEvent(ev)
	if got != nil {
		t.Errorf("expected nil for unknown event, got %s", got)
	}
}

func TestWireEncodeServerEvent_SessionsChanged_ViewUpdate(t *testing.T) {
	changedAt := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	ev := proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{
			{
				ID:        "s1",
				Project:   "p",
				Command:   "claude",
				CreatedAt: "2026-06-20T00:00:00Z",
				View: view.View{
					Card: view.Card{
						Title:       "T",
						Subtitle:    "S",
						Tags:        []view.Tag{{Text: "tag"}},
						BorderTitle: view.Tag{Text: "BT"},
					},
					StatusLine:      "line",
					Status:          view.StatusWaiting,
					StatusChangedAt: changedAt,
				},
			},
		},
		ActiveSessionID: "s1",
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame for EvtSessionsChanged")
	}

	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "v" {
		t.Errorf("k: got %v, want \"v\"", m["k"])
	}
	if m["activeSessionID"] != "s1" {
		t.Errorf("activeSessionID: got %v, want \"s1\"", m["activeSessionID"])
	}
	sessions, ok := m["sessions"].([]any)
	if !ok || len(sessions) != 1 {
		t.Fatalf("sessions: expected []any of len 1, got %T %v", m["sessions"], m["sessions"])
	}
	sess := sessions[0].(map[string]any)
	v := sess["view"].(map[string]any)
	card := v["card"].(map[string]any)
	if card["title"] != "T" {
		t.Errorf("view.card.title: got %v, want \"T\"", card["title"])
	}
	if v["status"] != "waiting" {
		t.Errorf("view.status: got %v, want \"waiting\"", v["status"])
	}
}

func TestWireEncodeServerEvent_SessionsChanged_OmitsEmptyActiveID(t *testing.T) {
	ev := proto.EvtSessionsChanged{
		Sessions:        []proto.SessionInfo{{ID: "s1", CreatedAt: "2026-06-20T00:00:00Z"}},
		ActiveSessionID: "",
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame")
	}
	if strings.Contains(string(got), "activeSessionID") {
		t.Errorf("expected activeSessionID to be omitted when empty, got: %s", got)
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
