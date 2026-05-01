package mcpproxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/config"
)

func TestWriteMCPJSON_noProject(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "mcp.json")
	servers := map[string]config.MCPProxyServer{
		"obs": {Command: "npx"},
	}
	if err := writeMCPJSON(out, filepath.Join(dir, "nonexistent.json"), servers, "/bin/roost"); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	raw, _ := os.ReadFile(out)
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(doc.MCPServers))
	}
	if _, ok := doc.MCPServers["obs"]; !ok {
		t.Error("expected obs entry")
	}
}

func TestWriteMCPJSON_mergesProject(t *testing.T) {
	dir := t.TempDir()
	projectMCP := filepath.Join(dir, "project.mcp.json")
	os.WriteFile(projectMCP, []byte(`{"mcpServers":{"existing":{"command":"other"},"obs":{"command":"old"}}}`), 0o644)

	out := filepath.Join(dir, "mcp.json")
	servers := map[string]config.MCPProxyServer{
		"obs": {Command: "npx"},
	}
	if err := writeMCPJSON(out, projectMCP, servers, "/bin/roost"); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	raw, _ := os.ReadFile(out)
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.MCPServers) != 2 {
		t.Fatalf("expected 2 servers, got %d: %v", len(doc.MCPServers), doc.MCPServers)
	}
	if _, ok := doc.MCPServers["existing"]; !ok {
		t.Error("project entry 'existing' should be preserved")
	}
	// obs should be the shim, not the old entry
	var obs struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(doc.MCPServers["obs"], &obs); err != nil {
		t.Fatal(err)
	}
	if obs.Command != "/bin/roost" {
		t.Errorf("obs.command = %q, want shim /bin/roost", obs.Command)
	}
}

func TestWriteMCPJSON_idempotent(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "mcp.json")
	servers := map[string]config.MCPProxyServer{"obs": {Command: "npx"}}

	if err := writeMCPJSON(out, filepath.Join(dir, "none"), servers, "/bin/roost"); err != nil {
		t.Fatal(err)
	}
	info1, _ := os.Stat(out)

	if err := writeMCPJSON(out, filepath.Join(dir, "none"), servers, "/bin/roost"); err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(out)

	if info1.ModTime() != info2.ModTime() {
		t.Error("second write should be skipped when content is unchanged")
	}
}
