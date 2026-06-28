package proto

import (
	"testing"
	"time"
)

func TestCommandNames(t *testing.T) {
	cases := []struct {
		c    Command
		name string
	}{
		{CmdSubscribe{}, CmdNameSubscribe},
		{CmdUnsubscribe{}, CmdNameUnsubscribe},
		{CmdEvent{}, CmdNameEvent},
		{CmdSubsystemEvent{}, CmdNameSubsystem},
		{CmdSurfaceReadText{}, CmdNameSurfaceReadText},
		{CmdSurfaceSendText{}, CmdNameSurfaceSendText},
		{CmdSurfaceSendKey{}, CmdNameSurfaceSendKey},
		{CmdDriverList{}, CmdNameDriverList},
		{CmdHookEvent{}, CmdNameHookEvent},
	}
	for _, c := range cases {
		if got := c.c.CommandName(); got != c.name {
			t.Errorf("%T CommandName = %q, want %q", c.c, got, c.name)
		}
	}
}

func TestEventNames(t *testing.T) {
	cases := []struct {
		e    ServerEvent
		name string
	}{
		{EvtSessionsChanged{}, EvtNameSessionsChanged},
		{EvtProjectSelected{}, EvtNameProjectSelected},
		{EvtLogLine{}, EvtNameLogLine},
		{EvtSessionFileLine{}, EvtNameSessionFileLine},
		{EvtAgentNotification{}, EvtNameAgentNotification},
	}
	for _, c := range cases {
		if got := c.e.EventName(); got != c.name {
			t.Errorf("%T EventName = %q, want %q", c.e, got, c.name)
		}
	}
}

func TestResponseInterface(t *testing.T) {
	var _ Response = RespOK{}
	var _ Response = RespCreateSession{}
	var _ Response = RespSessions{}
	var _ Response = RespSurfaceText{}
	var _ Response = RespDriverList{}
}

func TestBaseName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/a/b/c", "c"},
		{"a/b/c", "c"},
		{`C:\foo\bar`, "bar"},
		{"plain", "plain"},
		{"", ""},
		{"/trailing/", ""},
	}
	for _, c := range cases {
		if got := baseName(c.in); got != c.want {
			t.Errorf("baseName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSessionInfoHelpers(t *testing.T) {
	si := SessionInfo{Project: "/repo/proj", Command: "claude"}
	if si.Name() != "proj" {
		t.Errorf("Name = %q", si.Name())
	}
	if si.DisplayCommand() != "claude" {
		t.Errorf("DisplayCommand = %q", si.DisplayCommand())
	}
	si2 := SessionInfo{}
	if si2.DisplayCommand() != "idle" {
		t.Errorf("empty DisplayCommand should be idle, got %q", si2.DisplayCommand())
	}

	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339)
	si3 := SessionInfo{CreatedAt: ts}
	if !si3.CreatedAtTime().Equal(time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("CreatedAtTime parse failed: %v", si3.CreatedAtTime())
	}
	// StateChangedAtTime falls back to CreatedAt when empty
	if !si3.StateChangedAtTime().Equal(si3.CreatedAtTime()) {
		t.Errorf("StateChangedAtTime fallback wrong")
	}
	si4 := SessionInfo{StateChangedAt: ts}
	if si4.StateChangedAtTime().IsZero() {
		t.Errorf("StateChangedAtTime parse failed")
	}
}
