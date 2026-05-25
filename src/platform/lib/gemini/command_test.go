package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHelp(t *testing.T) {
	for _, a := range []string{"help", "-h", "--help"} {
		if err := Run([]string{a}); err != nil {
			t.Errorf("Run(%q) err: %v", a, err)
		}
	}
}

func TestRunMissingSubcommand(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Error("expected error")
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	if err := Run([]string{"nope"}); err == nil {
		t.Error("expected error")
	}
}

func TestResolveSettingsPathFromEnv(t *testing.T) {
	t.Setenv("GEMINI_SETTINGS_PATH", "/custom/g.json")
	got, err := resolveSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/g.json" {
		t.Errorf("got %q", got)
	}
}

func TestRunSetupRegistersHooksAndMCP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	t.Setenv("GEMINI_SETTINGS_PATH", path)
	if err := RunSetup(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if _, ok := s["hooks"]; !ok {
		t.Errorf("hooks missing: %v", s)
	}
	mcp, _ := s["mcpServers"].(map[string]any)
	if _, ok := mcp["roost-peers"]; !ok {
		t.Errorf("mcpServers.roost-peers missing: %v", s)
	}
	// Idempotent
	if err := RunSetup(); err != nil {
		t.Fatal(err)
	}
}

func TestAddHookEntryDedup(t *testing.T) {
	hooks := map[string]any{}
	if !addHookEntry(hooks, "SessionStart", "cmd") {
		t.Errorf("first add should return true")
	}
	if addHookEntry(hooks, "SessionStart", "cmd") {
		t.Errorf("second add should return false (dedup)")
	}
}

func TestHasCommand(t *testing.T) {
	entry := map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": "x"},
		},
	}
	if !hasCommand(entry, "x") {
		t.Errorf("expected true")
	}
	if hasCommand(entry, "y") {
		t.Errorf("expected false")
	}
	if hasCommand("not a map", "x") {
		t.Errorf("non-map should be false")
	}
	if hasCommand(map[string]any{"hooks": []any{"not a map"}}, "x") {
		t.Errorf("non-map hook should be false")
	}
}
