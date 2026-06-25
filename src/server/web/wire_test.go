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
	// Wire-binary safety: the third element is the base64 STRING straight
	// from EvtSurfaceOutput.DataB64; the browser decodes it back to bytes.
	// Passing decoded bytes through encoding/json would corrupt non-UTF-8
	// terminal output (U+FFFD replacement), so we test for the base64 form.
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
	if len(arr) != 4 {
		t.Fatalf("expected 4-element array, got %d", len(arr))
	}
	if arr[0].(float64) != 1.5 {
		t.Errorf("time: got %v, want 1.5", arr[0])
	}
	if arr[1].(string) != "o" {
		t.Errorf("type: got %v, want \"o\"", arr[1])
	}
	if arr[2].(string) != encoded {
		t.Errorf("data: got %q, want %q (base64-encoded)", arr[2], encoded)
	}
	if arr[3].(string) != "s1" {
		t.Errorf("sessionID: got %v, want \"s1\" (multiplex routing element)", arr[3])
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

	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "n" {
		t.Errorf("k: got %v, want \"n\"", m["k"])
	}
	if m["sessionId"] != "s1" {
		t.Errorf("sessionId: got %v, want \"s1\"", m["sessionId"])
	}
	if m["cmd"].(float64) != 9 {
		t.Errorf("cmd: got %v, want 9", m["cmd"])
	}
	if m["title"] != "t" {
		t.Errorf("title: got %v, want \"t\"", m["title"])
	}
	if m["body"] != "b" {
		t.Errorf("body: got %v, want \"b\"", m["body"])
	}
	if m["nowMs"].(float64) <= 0 {
		t.Errorf("nowMs: expected > 0, got %v", m["nowMs"])
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

	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["title"] != "t" {
		t.Errorf("title: got %v, want \"t\"", m["title"])
	}
	if _, hasBody := m["body"]; hasBody {
		t.Errorf("body should be omitted when empty")
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

// TestWireEncodeServerEvent_SessionsChanged_DropsActiveID guards the bug
// where a non-empty daemon-side ActiveSessionID leaked into every web
// client's view-update frame and clobbered their locally-tracked
// selection. Each new session spawn (reduceTmuxPaneSpawned) and many
// other reducers mutate state.ActiveSession, and EffBroadcastSessionsChanged
// fan-outs reach every connected browser — so even with a single web user
// open, a background session creation or driver frame push would yank
// their active tab away. The wire must NOT carry activeSessionID on
// view-update frames; web selection is per-client and managed locally.
func TestWireEncodeServerEvent_SessionsChanged_DropsActiveID(t *testing.T) {
	ev := proto.EvtSessionsChanged{
		Sessions:        []proto.SessionInfo{{ID: "s1", CreatedAt: "2026-06-20T00:00:00Z"}},
		ActiveSessionID: "s1",
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame")
	}
	if strings.Contains(string(got), "activeSessionID") {
		t.Errorf("activeSessionID must not appear on view-update frames; got: %s", got)
	}
}

// TestWireEncodeServerEvent_SessionsChanged_ForwardsActiveOccupant validates
// that proto.EvtSessionsChanged.ActiveOccupant ("main" | "log" | "frame")
// IS forwarded on view-update frames. Unlike ActiveSessionID (per-client
// state — must not be clobbered by daemon-wide events), ActiveOccupant is
// a daemon-global UI property the browser palette consumes to gate the
// push scope (FR-005/FR-006). ADR-0044 forbids per-session occupant
// fields; the daemon-global one carried here is exempt.
func TestWireEncodeServerEvent_SessionsChanged_ForwardsActiveOccupant(t *testing.T) {
	ev := proto.EvtSessionsChanged{
		Sessions:       []proto.SessionInfo{{ID: "s1", CreatedAt: "2026-06-20T00:00:00Z"}},
		ActiveOccupant: "frame",
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame")
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["activeOccupant"] != "frame" {
		t.Errorf("activeOccupant: got %v, want \"frame\"", m["activeOccupant"])
	}
}

// TestWireEncodeServerEvent_SessionsChanged_OmitsEmptyActiveOccupant verifies
// that the omitempty contract holds — an empty ActiveOccupant must not appear
// on the wire, so older clients (pre-this-change) treat the absence as
// "no frame" via the existing fail-closed path.
func TestWireEncodeServerEvent_SessionsChanged_OmitsEmptyActiveOccupant(t *testing.T) {
	ev := proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{{ID: "s1", CreatedAt: "2026-06-20T00:00:00Z"}},
		// ActiveOccupant left empty
	}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame")
	}
	if strings.Contains(string(got), "activeOccupant") {
		t.Errorf("activeOccupant should be omitted when empty; got: %s", got)
	}
}

func TestWireEncodeTranscriptTail(t *testing.T) {
	ev := proto.EvtSessionFileLine{SessionID: "s1", Kind: "transcript", Line: "hello"}
	got := encodeFromSessionFileLine(ev)
	if got == nil {
		t.Fatal("expected non-nil frame for transcript kind")
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "tt" {
		t.Errorf("k: got %v, want \"tt\"", m["k"])
	}
	if m["sessionId"] != "s1" {
		t.Errorf("sessionId: got %v, want \"s1\"", m["sessionId"])
	}
	if m["line"] != "hello" {
		t.Errorf("line: got %v, want \"hello\"", m["line"])
	}
}

func TestWireEncodeEventLogTail(t *testing.T) {
	ev := proto.EvtSessionFileLine{SessionID: "s2", Kind: "event-log", Line: "world"}
	got := encodeFromSessionFileLine(ev)
	if got == nil {
		t.Fatal("expected non-nil frame for event-log kind")
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "et" {
		t.Errorf("k: got %v, want \"et\"", m["k"])
	}
	if m["sessionId"] != "s2" {
		t.Errorf("sessionId: got %v, want \"s2\"", m["sessionId"])
	}
	if m["line"] != "world" {
		t.Errorf("line: got %v, want \"world\"", m["line"])
	}
}

func TestWireEncodeTranscriptTail_UnknownKindSkipped(t *testing.T) {
	ev := proto.EvtSessionFileLine{SessionID: "s1", Kind: "other", Line: "data"}
	got := encodeFromSessionFileLine(ev)
	if got != nil {
		t.Errorf("expected nil for unknown Kind, got %s", got)
	}
}

func TestWireEncodeNotification(t *testing.T) {
	before := time.Now().UnixMilli()
	ev := proto.EvtAgentNotification{SessionID: "s1", Cmd: 9, Title: "t", Body: "b"}
	got := encodeFromAgentNotification(ev)
	after := time.Now().UnixMilli()
	if got == nil {
		t.Fatal("expected non-nil frame for EvtAgentNotification")
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "n" {
		t.Errorf("k: got %v, want \"n\"", m["k"])
	}
	if m["cmd"].(float64) != 9 {
		t.Errorf("cmd: got %v, want 9", m["cmd"])
	}
	if m["title"] != "t" {
		t.Errorf("title: got %v, want \"t\"", m["title"])
	}
	if m["body"] != "b" {
		t.Errorf("body: got %v, want \"b\"", m["body"])
	}
	nowMs := int64(m["nowMs"].(float64))
	if nowMs < before || nowMs > after {
		t.Errorf("nowMs %d not in range [%d, %d]", nowMs, before, after)
	}
}

func TestWireEncodeServerEvent_SessionFileLine_Transcript(t *testing.T) {
	ev := proto.EvtSessionFileLine{SessionID: "s1", Kind: "transcript", Line: "line1"}
	got := encodeServerEvent(ev)
	if got == nil {
		t.Fatal("expected non-nil frame for EvtSessionFileLine transcript")
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["k"] != "tt" {
		t.Errorf("k: got %v, want \"tt\"", m["k"])
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
