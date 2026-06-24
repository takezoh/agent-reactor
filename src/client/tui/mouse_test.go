package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

func TestDeactivateDoneMsgClearsState(t *testing.T) {
	m := Model{
		active:   "@1",
		anchored: "@1",
		folded:   make(map[string]bool),
	}
	result, cmd := m.Update(deactivateDoneMsg{err: nil})
	got := result.(Model)
	if got.active != "" {
		t.Errorf("active = %q, want empty", got.active)
	}
	if got.anchored != "" {
		t.Errorf("anchored = %q, want empty", got.anchored)
	}
	if cmd == nil {
		t.Error("expected focusCmd, got nil")
	}
}

func TestDeactivateDoneMsgPreservesStateOnError(t *testing.T) {
	m := Model{
		active:   "@1",
		anchored: "@1",
		folded:   make(map[string]bool),
	}
	result, cmd := m.Update(deactivateDoneMsg{err: errDummy})
	got := result.(Model)
	if got.active != "@1" {
		t.Errorf("active = %q, want @1 (preserved on error)", got.active)
	}
	if got.anchored != "@1" {
		t.Errorf("anchored = %q, want @1 (preserved on error)", got.anchored)
	}
	if cmd == nil {
		t.Error("expected focusCmd even on error, got nil")
	}
}

var errDummy = fmt.Errorf("dummy")

func TestClickHeaderWithActiveSession(t *testing.T) {
	m := Model{
		active: "@1",
		height: 20,
		width:  80,
		folded: make(map[string]bool),
		filter: allOnFilter(),
	}
	msg := tea.MouseClickMsg(tea.Mouse{X: 5, Y: 0, Button: tea.MouseLeft})
	_, cmd := m.handleMouseClick(msg)
	if cmd == nil {
		t.Fatal("expected deactivateCmd, got nil")
	}
}

func TestClickHeaderWithoutActiveSession(t *testing.T) {
	m := Model{
		active: "",
		height: 20,
		width:  80,
		folded: make(map[string]bool),
		filter: allOnFilter(),
	}
	msg := tea.MouseClickMsg(tea.Mouse{X: 5, Y: 0, Button: tea.MouseLeft})
	_, cmd := m.handleMouseClick(msg)
	if cmd == nil {
		t.Fatal("expected focusCmd, got nil")
	}
}

func TestHandleServerEventClearsMissingActiveAndAnchor(t *testing.T) {
	m := Model{
		active:   "gone",
		anchored: "gone",
		folded:   make(map[string]bool),
		filter:   allOnFilter(),
		sessions: []proto.SessionInfo{
			{ID: "gone", Project: "/tmp/p", View: state.View{DisplayName: "p"}},
		},
	}
	m.rebuildItems()

	result, _ := m.handleServerEvent(proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{
			{ID: "keep", Project: "/tmp/p", View: state.View{DisplayName: "p"}},
		},
	})
	got := result.(Model)
	if got.active != "" {
		t.Errorf("active = %q, want empty", got.active)
	}
	if got.anchored != "" {
		t.Errorf("anchored = %q, want empty", got.anchored)
	}
}

func TestHandleServerEventWithoutActiveSessionClearsSelection(t *testing.T) {
	m := Model{
		cursor:   1,
		active:   "",
		anchored: "keep",
		folded:   make(map[string]bool),
		filter:   allOnFilter(),
	}
	result, _ := m.handleServerEvent(proto.EvtSessionsChanged{
		Sessions: []proto.SessionInfo{
			{ID: "keep", Project: "/tmp/p", View: state.View{DisplayName: "p"}},
		},
	})
	got := result.(Model)
	if got.cursor != -1 {
		t.Errorf("cursor = %d, want -1", got.cursor)
	}
	if got.active != "" {
		t.Errorf("active = %q, want empty", got.active)
	}
	if got.anchored != "" {
		t.Errorf("anchored = %q, want empty", got.anchored)
	}
}

func TestRebuildItemsPreservesUnselectedCursor(t *testing.T) {
	m := Model{
		cursor: -1,
		folded: make(map[string]bool),
		filter: allOnFilter(),
		sessions: []proto.SessionInfo{
			{ID: "keep", Project: "/tmp/p", View: state.View{DisplayName: "p"}},
		},
	}

	m.rebuildItems()

	if m.cursor != -1 {
		t.Errorf("cursor = %d, want -1", m.cursor)
	}
}
