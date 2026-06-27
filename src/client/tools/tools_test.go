package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/platform/features"
)

func TestHiddenToolExcludedFromAll(t *testing.T) {
	r := NewRegistry()
	r.Register(Tool{Name: "visible", Description: "visible"})
	r.Register(Tool{Name: "hidden", Description: "hidden", Hidden: true})

	all := r.All()
	for _, t2 := range all {
		if t2.Name == "hidden" {
			t.Error("hidden tool should not appear in All()")
		}
	}
	if len(all) != 1 {
		t.Errorf("All() len = %d, want 1", len(all))
	}
}

func TestHiddenToolExcludedFromMatch(t *testing.T) {
	r := NewRegistry()
	r.Register(Tool{Name: "visible", Description: "visible"})
	r.Register(Tool{Name: "hidden", Description: "hidden", Hidden: true})

	matched := r.Match("")
	for _, t2 := range matched {
		if t2.Tool.Name == "hidden" {
			t.Error("hidden tool should not appear in Match()")
		}
	}
	matched2 := r.Match("hidden")
	if len(matched2) != 0 {
		t.Errorf("Match('hidden') = %v, want empty", matched2)
	}
}

func TestGetReturnsHiddenTool(t *testing.T) {
	r := NewRegistry()
	r.Register(Tool{Name: "hidden", Description: "hidden", Hidden: true})

	got := r.Get("hidden")
	if got == nil {
		t.Fatal("Get(hidden) should return the tool")
	}
	if got.Name != "hidden" {
		t.Errorf("Get(hidden).Name = %q, want hidden", got.Name)
	}
}

func TestPushCommandsHiddenWhenNoDriverFrame(t *testing.T) {
	// Without MainHasDriverFrame, no command: entries are registered.
	r := DefaultRegistry(features.Set{}, PaletteContext{
		Scope:        ScopeProject,
		PushCommands: []string{"shell", "vim"},
	})
	if got := r.Get("command: shell"); got != nil {
		t.Error("command: shell should not be registered when MainHasDriverFrame is false")
	}
	for _, tool := range r.All() {
		if tool.Name == "command: shell" || tool.Name == "command: vim" {
			t.Errorf("push command %q should not appear in All() when MainHasDriverFrame is false", tool.Name)
		}
	}
}

func TestPushCommandsVisibleWhenMainHasDriverFrame(t *testing.T) {
	r := DefaultRegistry(features.Set{}, PaletteContext{
		Scope:              ScopeProject,
		MainHasDriverFrame: true,
		PushCommands:       []string{"shell", "vim"},
	})
	for _, name := range []string{"command: shell", "command: vim"} {
		got := r.Get(name)
		if got == nil {
			t.Fatalf("%q should be registered when MainHasDriverFrame is true and PushCommands contains it", name)
		}
		if got.Hidden {
			t.Errorf("%q should not be Hidden", name)
		}
	}
	var found []string
	for _, tool := range r.All() {
		if tool.Name == "command: shell" || tool.Name == "command: vim" {
			found = append(found, tool.Name)
		}
	}
	if len(found) != 2 {
		t.Errorf("All() should contain both push commands, got %v", found)
	}
}

func TestPushCommandsEmptyWhenNoPushCommands(t *testing.T) {
	// MainHasDriverFrame=true but PushCommands is nil — no command: entries registered.
	r := DefaultRegistry(features.Set{}, PaletteContext{
		Scope:              ScopeProject,
		MainHasDriverFrame: true,
	})
	for _, tool := range r.All() {
		if strings.HasPrefix(tool.Name, "command: ") {
			t.Errorf("unexpected push command %q when PushCommands is nil", tool.Name)
		}
	}
}

func TestForkHiddenWhenNoForkableDriver(t *testing.T) {
	r := DefaultRegistry(features.Set{})
	// Without MainHasForkableDriver, fork is not registered.
	if got := r.Get("fork-session"); got != nil {
		t.Error("fork should not be registered when MainHasForkableDriver is false")
	}
	for _, tool := range r.All() {
		if tool.Name == "fork-session" {
			t.Error("fork should not appear in All() when MainHasForkableDriver is false")
		}
	}
}

func TestForkVisibleWhenForkableDriver(t *testing.T) {
	r := DefaultRegistry(features.Set{}, PaletteContext{Scope: ScopeProject, MainHasForkableDriver: true})
	got := r.Get("fork-session")
	if got == nil {
		t.Fatal("fork should be registered when MainHasForkableDriver is true")
	}
	if got.Hidden {
		t.Error("fork should not be Hidden")
	}
	found := false
	for _, tool := range r.All() {
		if tool.Name == "fork-session" {
			found = true
		}
	}
	if !found {
		t.Error("fork should appear in All() when MainHasForkableDriver is true")
	}
}

func TestStandardScopeOmitsProjectTools(t *testing.T) {
	r := DefaultRegistry(features.Set{})
	for _, name := range []string{"command: shell", "fork-session"} {
		if r.Get(name) != nil {
			t.Errorf("standard scope: %q should not be registered", name)
		}
	}
	for _, name := range []string{"shutdown", "new-session"} {
		if r.Get(name) == nil {
			t.Errorf("standard scope: %q should be registered", name)
		}
	}
}

func TestProjectScopeOmitsStandardOnlyTools(t *testing.T) {
	r := DefaultRegistry(features.Set{}, PaletteContext{Scope: ScopeProject})
	for _, name := range []string{"shutdown", "create-project", "stop-session", "command: shell"} {
		if r.Get(name) != nil {
			t.Errorf("project scope: %q should not be registered", name)
		}
	}
	if r.Get("new-session") == nil {
		t.Error("project scope: new-session should be registered")
	}
}

func TestMatchEmptyQueryReturnsAll(t *testing.T) {
	r := NewRegistry()
	r.Register(Tool{Name: "alpha"})
	r.Register(Tool{Name: "beta"})
	r.Register(Tool{Name: "hidden-tool", Hidden: true})

	got := r.Match("")
	if len(got) != 2 {
		t.Fatalf("Match('') len = %d, want 2", len(got))
	}
	if got[0].Tool.Name != "alpha" || got[1].Tool.Name != "beta" {
		t.Errorf("Match('') order = %v/%v, want alpha/beta", got[0].Tool.Name, got[1].Tool.Name)
	}
	for _, m := range got {
		if len(m.Indexes) != 0 {
			t.Error("empty query should produce no match indexes")
		}
	}
}

func TestMatchFuzzySubsequence(t *testing.T) {
	r := NewRegistry()
	r.Register(Tool{Name: "new-session"})
	r.Register(Tool{Name: "stop-session"})
	r.Register(Tool{Name: "detach"})

	got := r.Match("sess")
	names := make([]string, len(got))
	for i, m := range got {
		names[i] = m.Tool.Name
	}
	// Both session tools match "sess" as subsequence; detach does not
	if len(got) != 2 {
		t.Fatalf("Match('sess') = %v, want 2 results", names)
	}
	for _, m := range got {
		if len(m.Indexes) == 0 {
			t.Errorf("Match('sess') %q: expected non-empty indexes", m.Tool.Name)
		}
	}
}

func TestMatchNoResults(t *testing.T) {
	r := NewRegistry()
	r.Register(Tool{Name: "new-session"})

	got := r.Match("zzz")
	if len(got) != 0 {
		t.Errorf("Match('zzz') = %v, want empty", got)
	}
}

func TestMatchMultiToken(t *testing.T) {
	r := NewRegistry()
	r.Register(Tool{Name: "new-session"})
	r.Register(Tool{Name: "stop-session"})
	r.Register(Tool{Name: "detach"})

	// both session tools match "new" OR "session" independently, but both must match
	got := r.Match("new session")
	if len(got) != 1 || got[0].Tool.Name != "new-session" {
		t.Errorf("Match('new session') = %v, want [new-session]", got)
	}
	if len(got[0].Indexes) == 0 {
		t.Error("Match('new session') should have non-empty indexes")
	}

	// one token that matches nothing → zero results
	got2 := r.Match("zzz sess")
	if len(got2) != 0 {
		t.Errorf("Match('zzz sess') = %v, want empty", got2)
	}

	// all-whitespace input → all visible tools, no indexes
	got3 := r.Match("   ")
	if len(got3) != 3 {
		t.Errorf("Match('   ') len = %d, want 3", len(got3))
	}
	for _, m := range got3 {
		if len(m.Indexes) != 0 {
			t.Error("all-whitespace query should produce no match indexes")
		}
	}
}

var defaultEditorCfg = EditorConfig{
	Command:    "code",
	Extensions: []string{".code-workspace"},
}

func TestOpenEditorHiddenWithoutActiveProject(t *testing.T) {
	for _, scope := range []PaletteScope{ScopeStandard, ScopeProject} {
		r := DefaultRegistry(features.Set{}, PaletteContext{Scope: scope, HasActiveProject: false})
		if r.Get("open-editor") != nil {
			t.Errorf("scope %v: open-editor should not be registered when HasActiveProject=false", scope)
		}
	}
}

func TestOpenEditorVisibleWithActiveProject(t *testing.T) {
	for _, scope := range []PaletteScope{ScopeStandard, ScopeProject} {
		r := DefaultRegistry(features.Set{}, PaletteContext{Scope: scope, HasActiveProject: true})
		if r.Get("open-editor") == nil {
			t.Errorf("scope %v: open-editor should be registered when HasActiveProject=true", scope)
		}
	}
}

func TestOpenEditorRunOpensFolder(t *testing.T) {
	dir := t.TempDir()
	var gotCmd, gotTarget string
	origLaunch := editorLaunch
	editorLaunch = func(cmd, target string) error {
		gotCmd, gotTarget = cmd, target
		return nil
	}
	t.Cleanup(func() { editorLaunch = origLaunch })

	r := DefaultRegistry(features.Set{}, PaletteContext{HasActiveProject: true})
	tool := r.Get("open-editor")
	if tool == nil {
		t.Fatal("open-editor not found")
	}
	ctx := &ToolContext{Config: ToolConfig{ActiveProject: dir, Editor: defaultEditorCfg}}
	if _, err := tool.Run(ctx, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotCmd != "code" {
		t.Errorf("command = %q, want code", gotCmd)
	}
	if gotTarget != dir {
		t.Errorf("target = %q, want dir %q (no workspace file)", gotTarget, dir)
	}
}

func TestOpenEditorRunOpensWorkspaceFile(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "myproject.code-workspace")
	if err := os.WriteFile(ws, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotTarget string
	origLaunch := editorLaunch
	editorLaunch = func(_, target string) error { gotTarget = target; return nil }
	t.Cleanup(func() { editorLaunch = origLaunch })

	r := DefaultRegistry(features.Set{}, PaletteContext{HasActiveProject: true})
	tool := r.Get("open-editor")
	ctx := &ToolContext{Config: ToolConfig{ActiveProject: dir, Editor: defaultEditorCfg}}
	if _, err := tool.Run(ctx, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotTarget != ws {
		t.Errorf("target = %q, want workspace file %q", gotTarget, ws)
	}
}

func TestOpenEditorRunFallsBackToCodeWhenCommandEmpty(t *testing.T) {
	dir := t.TempDir()
	var gotCmd string
	origLaunch := editorLaunch
	editorLaunch = func(cmd, _ string) error { gotCmd = cmd; return nil }
	t.Cleanup(func() { editorLaunch = origLaunch })

	r := DefaultRegistry(features.Set{}, PaletteContext{HasActiveProject: true})
	tool := r.Get("open-editor")
	ctx := &ToolContext{Config: ToolConfig{ActiveProject: dir, Editor: EditorConfig{Extensions: []string{".code-workspace"}}}}
	if _, err := tool.Run(ctx, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotCmd != "code" {
		t.Errorf("fallback command = %q, want code", gotCmd)
	}
}

func TestOpenEditorRunErrorsWithNoActiveProject(t *testing.T) {
	origLaunch := editorLaunch
	editorLaunch = func(_, _ string) error { return fmt.Errorf("should not be called") }
	t.Cleanup(func() { editorLaunch = origLaunch })

	r := DefaultRegistry(features.Set{}, PaletteContext{HasActiveProject: true})
	tool := r.Get("open-editor")
	ctx := &ToolContext{Config: ToolConfig{ActiveProject: "", Editor: defaultEditorCfg}}
	if _, err := tool.Run(ctx, nil); err == nil {
		t.Error("Run with empty ActiveProject should return error")
	}
}
