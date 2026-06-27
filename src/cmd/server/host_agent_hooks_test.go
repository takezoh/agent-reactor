package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/client/lib/agenthook"
)

// TestRegisterOneHostAgentHook_WritesHookCmd verifies the per-agent helper
// writes a hook command of the expected shape into the right settings file.
// Drives every Spec in agenthook.All so adding an agent surfaces here, not
// as silent partial coverage at runtime.
func TestRegisterOneHostAgentHook_WritesHookCmd(t *testing.T) {
	home := t.TempDir()
	exe := "/usr/bin/server"
	dataDir := "/var/lib/reactor"

	for _, spec := range agenthook.All {
		registerOneHostAgentHook(home, exe, dataDir, spec)
		settings := filepath.Join(home, spec.SettingsRel)
		cmd := firstCommandFromSettings(t, settings, spec.Events[0])
		want := exe + " event " + spec.Name + " -data-dir " + dataDir
		if cmd != want {
			t.Errorf("[%s] %s command = %q, want %q",
				spec.Name, spec.Events[0], cmd, want)
		}
	}
}

// TestRegisterOneHostAgentHook_ShellQuotesUnsafePaths ensures the hook
// command persisted to settings.json is shell-safe when exe / dataDir
// contain spaces — the path-with-spaces failure surface the deleted bash
// scripts' `printf %q` originally guarded.
func TestRegisterOneHostAgentHook_ShellQuotesUnsafePaths(t *testing.T) {
	home := t.TempDir()
	exe := "/home/Alice User/.local/bin/server"
	dataDir := "/var/lib/agent reactor"

	registerOneHostAgentHook(home, exe, dataDir, agenthook.Claude)
	settings := filepath.Join(home, agenthook.Claude.SettingsRel)
	cmd := firstCommandFromSettings(t, settings, "SessionStart")

	// The exe and dataDir must each appear as quoted single tokens so the
	// agent's downstream shell exec gets one arg per shellword.
	if !strings.Contains(cmd, "'/home/Alice User/.local/bin/server'") {
		t.Errorf("hook cmd %q does not contain quoted exe path", cmd)
	}
	if !strings.Contains(cmd, "'/var/lib/agent reactor'") {
		t.Errorf("hook cmd %q does not contain quoted data-dir", cmd)
	}
}

// TestRegisterOneHostAgentHook_EmptyDataDirOmitsFlag covers the
// "default data dir" code path where -data-dir must not appear in the
// persisted command (cleaner / easier to eyeball / matches the comment
// contract on registerHostAgentHooks).
func TestRegisterOneHostAgentHook_EmptyDataDirOmitsFlag(t *testing.T) {
	home := t.TempDir()
	registerOneHostAgentHook(home, "/usr/bin/server", "", agenthook.Claude)
	cmd := firstCommandFromSettings(t,
		filepath.Join(home, agenthook.Claude.SettingsRel), "SessionStart")
	if strings.Contains(cmd, "-data-dir") {
		t.Errorf("hook cmd %q should omit -data-dir when dataDir is empty", cmd)
	}
}

// TestAgentHookPostCreateSubcmds_MatchesAll locks the devcontainer postCreate
// subcmd list to agenthook.All — a drift between the two would mean the
// host registers an agent but the container path doesn't (or vice versa),
// the exact silent-partial-coverage failure the unified All slice exists
// to prevent.
func TestAgentHookPostCreateSubcmds_MatchesAll(t *testing.T) {
	got := agentHookPostCreateSubcmds()
	if len(got) != len(agenthook.All) {
		t.Fatalf("got %d subcmds, want %d", len(got), len(agenthook.All))
	}
	for i, spec := range agenthook.All {
		if got[i] != spec.SubcmdName {
			t.Errorf("subcmd[%d] = %q, want %q", i, got[i], spec.SubcmdName)
		}
	}
}

// firstCommandFromSettings is a tiny helper local to these tests so we
// don't reach across packages for the JSON walker. Returns the first
// hook command registered for event, or "" if absent.
func firstCommandFromSettings(t *testing.T, path, event string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
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
