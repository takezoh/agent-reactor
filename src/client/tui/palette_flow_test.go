package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/takezoh/agent-roost/client/tools"
)

// TestHandleParamSelectNoPanicAfterRunError verifies that pressing Enter after
// a tool's Run has already been called (paramIndex == len(Params)) does not
// panic. This guards against the race between tea.Quit taking effect and a
// queued Enter keypress reaching handleParamSelect.
func TestHandleParamSelectNoPanicAfterRunError(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.Tool{
		Name: "two-param",
		Params: []tools.Param{
			{Name: "a", Options: func(ctx *tools.ToolContext) []string { return []string{"x"} }},
			{Name: "b", Options: func(ctx *tools.ToolContext) []string { return []string{"y"} }},
		},
		Run: func(ctx *tools.ToolContext, args map[string]string) (*tools.ToolInvocation, error) {
			return nil, errors.New("simulated failure")
		},
	})

	m := NewPaletteModel(registry, &tools.ToolContext{}, "")
	m.phase = phaseParamSelect
	m.selectedTool = registry.Get("two-param")
	m.paramIndex = len(m.selectedTool.Params) // out-of-bounds condition

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}

	// Must not panic.
	got, _ := m.handleParamSelect(enterMsg)
	pm, ok := got.(PaletteModel)
	if !ok {
		t.Fatalf("expected PaletteModel, got %T", got)
	}
	// State must be unchanged — bounds check should have returned early.
	if pm.paramIndex != len(m.selectedTool.Params) {
		t.Errorf("paramIndex changed unexpectedly: got %d", pm.paramIndex)
	}
}

// TestHandleParamSelectNoPanicWhenNoTool verifies that Enter with a nil
// selectedTool does not panic.
func TestHandleParamSelectNoPanicWhenNoTool(t *testing.T) {
	registry := tools.NewRegistry()
	m := NewPaletteModel(registry, &tools.ToolContext{}, "")
	m.phase = phaseParamSelect
	m.selectedTool = nil
	m.paramIndex = 0

	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	// Must not panic.
	_, _ = m.handleParamSelect(enterMsg)
}
