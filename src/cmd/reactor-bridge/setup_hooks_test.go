package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-reactor/client/lib/agenthook"
	"github.com/takezoh/agent-reactor/platform/appid"
)

// TestLookupSetupHooksSpec_DispatchTable locks the bridge dispatch table to
// agenthook.All. If a Spec is added there but the bridge dispatch falls
// through to "unknown subcommand", devcontainer postCreate would silently
// no-op for that agent — exactly the bug the unified All slice was added to
// prevent.
func TestLookupSetupHooksSpec_DispatchTable(t *testing.T) {
	for _, spec := range agenthook.All {
		got, ok := lookupSetupHooksSpec(spec.SubcmdName)
		if !ok {
			t.Errorf("lookupSetupHooksSpec(%q) ok=false; bridge cannot dispatch this agent", spec.SubcmdName)
			continue
		}
		if got.Name != spec.Name {
			t.Errorf("lookupSetupHooksSpec(%q) returned Spec.Name=%q, want %q",
				spec.SubcmdName, got.Name, spec.Name)
		}
	}

	if _, ok := lookupSetupHooksSpec("event"); ok {
		t.Errorf("non-setup-hooks subcommand 'event' should not match dispatch table")
	}
	if _, ok := lookupSetupHooksSpec("nonexistent-setup-hooks"); ok {
		t.Errorf("unknown agent should not match dispatch table")
	}
}

// TestRunAgentSetupHooks_WritesContainerPath verifies the hookCmd carries
// the canonical in-container bridge path (not the test binary's path) so
// the registered settings.json points at /opt/agent-reactor/run/reactor-bridge
// regardless of where the test binary lives.
func TestRunAgentSetupHooks_WritesContainerPath(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")

	if err := runAgentSetupHooks(
		[]string{"-settings", settings},
		agenthook.Claude,
	); err != nil {
		t.Fatalf("runAgentSetupHooks: %v", err)
	}

	raw, err := os.ReadFile(settings)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("parse settings: %v", err)
	}

	wantCmd := appid.ContainerBinaryPath + " event claude"
	cmd := firstCommand(t, root, "SessionStart")
	if cmd != wantCmd {
		t.Errorf("SessionStart command = %q, want %q (must point at container bridge path)",
			cmd, wantCmd)
	}
}

// TestRunAgentSetupHooks_DataDirIsQuoted verifies the -data-dir flag value
// is shell-quoted in the persisted hookCmd, so a non-default ROOST_DATA_DIR
// with spaces survives the agent's shell-dispatch round-trip.
func TestRunAgentSetupHooks_DataDirIsQuoted(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")

	if err := runAgentSetupHooks(
		[]string{"-settings", settings, "-data-dir", "/var/lib/agent reactor"},
		agenthook.Claude,
	); err != nil {
		t.Fatalf("runAgentSetupHooks: %v", err)
	}

	raw, _ := os.ReadFile(settings)
	var root map[string]any
	_ = json.Unmarshal(raw, &root)
	cmd := firstCommand(t, root, "SessionStart")

	want := appid.ContainerBinaryPath + " event claude -data-dir '/var/lib/agent reactor'"
	if cmd != want {
		t.Errorf("SessionStart command = %q, want %q (data-dir must be quoted)", cmd, want)
	}
}

// TestRunAgentSetupHooks_Gemini covers the Gemini dispatch path: both
// agents must work through the same shared runner.
func TestRunAgentSetupHooks_Gemini(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")

	if err := runAgentSetupHooks(
		[]string{"-settings", settings},
		agenthook.Gemini,
	); err != nil {
		t.Fatalf("runAgentSetupHooks: %v", err)
	}

	raw, _ := os.ReadFile(settings)
	var root map[string]any
	_ = json.Unmarshal(raw, &root)
	// Gemini's BeforeTool is event-list-specific — Claude's PreToolUse
	// would not be present. Asserting on BeforeTool proves we routed
	// through the Gemini Spec.
	cmd := firstCommand(t, root, "BeforeTool")
	wantCmd := appid.ContainerBinaryPath + " event gemini"
	if cmd != wantCmd {
		t.Errorf("BeforeTool command = %q, want %q", cmd, wantCmd)
	}
}

// firstCommand returns the first hook command for the given event, or "" if
// the event has no entries.
func firstCommand(t *testing.T, root map[string]any, event string) string {
	t.Helper()
	hooks, _ := root["hooks"].(map[string]any)
	entries, _ := hooks[event].([]any)
	for _, e := range entries {
		em, _ := e.(map[string]any)
		hs, _ := em["hooks"].([]any)
		for _, h := range hs {
			hm, _ := h.(map[string]any)
			if cmd, ok := hm["command"].(string); ok {
				return cmd
			}
		}
	}
	return ""
}
