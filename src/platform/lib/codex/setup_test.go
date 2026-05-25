package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterMCPServer_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	added, err := RegisterMCPServer(path, "/usr/local/bin/roost")
	if err != nil {
		t.Fatalf("RegisterMCPServer: %v", err)
	}
	if !added {
		t.Error("expected added=true for new file")
	}

	data, _ := os.ReadFile(path)
	var servers map[string]any
	json.Unmarshal(data, &servers)

	entry, ok := servers["roost-peers"].(map[string]any)
	if !ok {
		t.Fatal("roost-peers entry missing")
	}
	if entry["command"] != "/usr/local/bin/roost" {
		t.Errorf("command = %v, want /usr/local/bin/roost", entry["command"])
	}
	args, _ := entry["args"].([]any)
	if len(args) == 0 || args[0] != "peers-mcp" {
		t.Errorf("args = %v, want [peers-mcp]", args)
	}
}

func TestRegisterMCPServer_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	added1, err := RegisterMCPServer(path, "/usr/local/bin/roost")
	if err != nil {
		t.Fatalf("first RegisterMCPServer: %v", err)
	}
	if !added1 {
		t.Error("expected added=true on first call")
	}

	added2, err := RegisterMCPServer(path, "/usr/local/bin/roost")
	if err != nil {
		t.Fatalf("second RegisterMCPServer: %v", err)
	}
	if added2 {
		t.Error("expected added=false on second call (already registered)")
	}
}

func TestRegisterMCPServer_PreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(`{"other-tool":{"command":"other","args":["run"]}}`), 0o644)

	_, err := RegisterMCPServer(path, "/usr/local/bin/roost")
	if err != nil {
		t.Fatalf("RegisterMCPServer: %v", err)
	}

	data, _ := os.ReadFile(path)
	var servers map[string]any
	json.Unmarshal(data, &servers)

	if _, ok := servers["other-tool"]; !ok {
		t.Error("existing other-tool entry was removed")
	}
	if _, ok := servers["roost-peers"]; !ok {
		t.Error("roost-peers was not written")
	}
}
