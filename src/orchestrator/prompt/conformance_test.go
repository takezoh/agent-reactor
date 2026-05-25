package prompt_test

import (
	"errors"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/prompt"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

// SPEC §17.1 — prompt rendering fails on unknown variables (strict mode / §5.4).
func TestSPEC_17_1_StrictTemplateUnknownVarErrors(t *testing.T) {
	vars := prompt.Vars{Issue: ptrackerv.Issue{ID: "1", Identifier: "T-1", Title: "title"}}

	_, err := prompt.Render("{{ unknown_var }}", vars)
	if err == nil {
		t.Fatal("want error for unknown variable, got nil")
	}
	if !errors.Is(err, prompt.ErrTemplateRender) {
		t.Errorf("want ErrTemplateRender, got %v", err)
	}
}
