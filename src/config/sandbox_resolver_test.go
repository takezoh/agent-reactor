package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxResolver_EmptyProject(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	r := NewSandboxResolver(user)
	got := r.Resolve("")
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer (empty project returns user config)", got.Mode)
	}
}

func TestSandboxResolver_NoSettingsFile(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	r := NewSandboxResolver(user)
	got := r.Resolve(t.TempDir()) // no .roost/settings.toml
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer (absent settings returns user config)", got.Mode)
	}
}

func TestSandboxResolver_ProjectOverridesMode(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	dir := t.TempDir()
	roostDir := filepath.Join(dir, ".roost")
	os.MkdirAll(roostDir, 0o755)
	os.WriteFile(filepath.Join(roostDir, "settings.toml"), []byte(`[sandbox]
mode = "direct"
`), 0o644)

	r := NewSandboxResolver(user)
	got := r.Resolve(dir)
	if got.Mode != "direct" {
		t.Errorf("Mode = %q, want direct (project overrides)", got.Mode)
	}
}

func TestSandboxResolver_ProjectNoSandboxSection(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	dir := t.TempDir()
	roostDir := filepath.Join(dir, ".roost")
	os.MkdirAll(roostDir, 0o755)
	os.WriteFile(filepath.Join(roostDir, "settings.toml"), []byte(`[workspace]
name = "myproject"
`), 0o644)

	r := NewSandboxResolver(user)
	got := r.Resolve(dir)
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer (no sandbox section → user config)", got.Mode)
	}
}

func TestSandboxResolver_CacheHit(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	dir := t.TempDir()
	roostDir := filepath.Join(dir, ".roost")
	os.MkdirAll(roostDir, 0o755)
	path := filepath.Join(roostDir, "settings.toml")
	os.WriteFile(path, []byte(`[sandbox]
mode = "direct"
`), 0o644)

	r := NewSandboxResolver(user)
	got1 := r.Resolve(dir)
	got2 := r.Resolve(dir)
	if got1.Mode != got2.Mode {
		t.Errorf("inconsistent results: %q vs %q", got1.Mode, got2.Mode)
	}
}

func TestSandboxResolver_ParseError_FallsBackToUser(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	dir := t.TempDir()
	roostDir := filepath.Join(dir, ".roost")
	os.MkdirAll(roostDir, 0o755)
	os.WriteFile(filepath.Join(roostDir, "settings.toml"), []byte("invalid toml :::"), 0o644)

	r := NewSandboxResolver(user)
	got := r.Resolve(dir)
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer (parse error falls back to user)", got.Mode)
	}
}

func TestSandboxResolver_SSHAgentKeysProjectOverride(t *testing.T) {
	user := SandboxConfig{
		Proxy: ProxyConfig{
			SSHAgent: SSHAgentConfig{Keys: []string{"~/.ssh/id_ed25519_default"}},
		},
	}
	dir := t.TempDir()
	roostDir := filepath.Join(dir, ".roost")
	os.MkdirAll(roostDir, 0o755)
	os.WriteFile(filepath.Join(roostDir, "settings.toml"), []byte(`[sandbox.proxy.ssh_agent]
keys = ["~/.ssh/id_ed25519_project"]
`), 0o644)

	r := NewSandboxResolver(user)
	got := r.Resolve(dir)
	if len(got.Proxy.SSHAgent.Keys) != 1 || got.Proxy.SSHAgent.Keys[0] != "~/.ssh/id_ed25519_project" {
		t.Errorf("SSHAgent.Keys = %v, want [~/.ssh/id_ed25519_project] (project replaces)", got.Proxy.SSHAgent.Keys)
	}
}

