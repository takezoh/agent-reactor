package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_SSHAgentKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`
[sandbox.proxy]
ssh_agent.keys = ["~/.ssh/id_ed25519", "~/.ssh/id_rsa"]
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sandbox.Proxy.SSHAgent.Keys) != 2 {
		t.Errorf("SSHAgent.Keys = %v, want 2 entries", cfg.Sandbox.Proxy.SSHAgent.Keys)
	}
	if cfg.Sandbox.Proxy.SSHAgent.Keys[0] != "~/.ssh/id_ed25519" {
		t.Errorf("SSHAgent.Keys[0] = %q, want ~/.ssh/id_ed25519", cfg.Sandbox.Proxy.SSHAgent.Keys[0])
	}
}
