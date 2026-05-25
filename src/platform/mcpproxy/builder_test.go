package mcpproxy

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/platform/config"
)

func TestSpecBuilderBasics(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	cfg := Config{
		RunBase:           filepath.Join(dir, "run"),
		ContainerSockPath: "/tmp/in/mcp.sock",
		ContainerBinPath:  "/usr/local/bin/roost",
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{}
	})
	if b.Name() != "mcpproxy" {
		t.Errorf("Name = %q", b.Name())
	}
	if b.Routes() != nil {
		t.Errorf("Routes should be nil")
	}
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.RunBase); err != nil {
		t.Errorf("RunBase not created: %v", err)
	}
}

func TestSpecBuilderEmptyServers(t *testing.T) {
	ctx := t.Context()
	b := NewSpecBuilder(ctx, Config{}, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{}
	})
	spec, err := b.ContainerSpec(ctx, "/proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Mounts) != 0 || len(spec.Env) != 0 {
		t.Errorf("expected empty spec, got %+v", spec)
	}
}

func TestSpecBuilderWithServers(t *testing.T) {
	ctx := t.Context()
	t.Setenv("TMPDIR", "/tmp")
	runBase := t.TempDir()
	cfg := Config{
		RunBase:           runBase,
		ContainerSockPath: "/tmp/incontainer/mcp.sock",
		ContainerBinPath:  "/bin/roost",
		WorkspaceFolderFor: func(p string) string {
			return "/workspace"
		},
	}
	b := NewSpecBuilder(ctx, cfg, func(string) config.MCPProxyConfig {
		return config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{
			"obs": {Command: "true"},
		}}
	})
	spec, err := b.ContainerSpec(ctx, "/myproj")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Env["ROOST_MCP_SOCK"] != cfg.ContainerSockPath {
		t.Errorf("env ROOST_MCP_SOCK = %q", spec.Env["ROOST_MCP_SOCK"])
	}
	if len(spec.Mounts) != 2 {
		t.Errorf("expected 2 mounts, got %d: %+v", len(spec.Mounts), spec.Mounts)
	}
	// Second call must be idempotent (broker already exists).
	if _, err := b.ContainerSpec(ctx, "/myproj"); err != nil {
		t.Fatal(err)
	}
}

func TestCompileServerEmptyCommand(t *testing.T) {
	_, err := compileServer("x", "", nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestCompileServerEnvOverride(t *testing.T) {
	t.Setenv("FOO", "baseline")
	srv, err := compileServer("x", "true", []string{"-v"}, map[string]string{"FOO": "override", "NEW": "v"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	foundOverride := false
	foundNew := false
	for _, kv := range srv.env {
		if kv == "FOO=override" {
			foundOverride = true
		}
		if kv == "FOO=baseline" {
			t.Errorf("baseline FOO leaked despite override")
		}
		if kv == "NEW=v" {
			foundNew = true
		}
	}
	if !foundOverride || !foundNew {
		t.Errorf("env missing overrides: %v", srv.env)
	}
}

func TestCompileServers(t *testing.T) {
	cfg := config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{
		"a": {Command: "true"},
		"b": {Command: "true"},
	}}
	got, err := compileServers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestCompileServersError(t *testing.T) {
	cfg := config.MCPProxyConfig{Servers: map[string]config.MCPProxyServer{
		"a": {Command: ""},
	}}
	if _, err := compileServers(cfg); err == nil {
		t.Error("expected error")
	}
}

func TestExitCode(t *testing.T) {
	if got := exitCode("p", "a", nil); got != 0 {
		t.Errorf("nil err: %d", got)
	}
	if got := exitCode("p", "a", errors.New("boom")); got != 1 {
		t.Errorf("plain err: %d", got)
	}
	// Real ExitError via /bin/false
	cmd := exec.Command("false")
	err := cmd.Run()
	if got := exitCode("p", "a", err); got == 0 {
		t.Errorf("exit error should be non-zero, got %d", got)
	}
}
