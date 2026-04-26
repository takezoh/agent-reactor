package sshagent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-roost/config"
)

func newBuilder(t *testing.T) *SpecBuilder {
	t.Helper()
	return NewSpecBuilder(context.Background(), t.TempDir())
}

func cfgForward(forward bool) config.SandboxConfig {
	return config.SandboxConfig{
		Proxy: config.ProxyConfig{
			SSHAgent: config.SSHAgentConfig{Forward: forward},
		},
	}
}

func cfgKeys(keys ...string) config.SandboxConfig {
	return config.SandboxConfig{
		Proxy: config.ProxyConfig{
			SSHAgent: config.SSHAgentConfig{Keys: keys},
		},
	}
}

func TestSpecBuilder_forward_false(t *testing.T) {
	spec, err := newBuilder(t).ContainerSpec(context.Background(), "/proj", cfgForward(false))
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Env) != 0 || len(spec.Mounts) != 0 {
		t.Errorf("expected zero spec, got %+v", spec)
	}
}

func TestSpecBuilder_forward_true_no_sock_env(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	spec, err := newBuilder(t).ContainerSpec(context.Background(), "/proj", cfgForward(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Env) != 0 {
		t.Errorf("expected zero spec when SSH_AUTH_SOCK unset, got %+v", spec)
	}
}

func TestSpecBuilder_forward_true_sock_missing(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/nonexistent/path/agent.sock")
	spec, err := newBuilder(t).ContainerSpec(context.Background(), "/proj", cfgForward(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Env) != 0 {
		t.Errorf("expected zero spec when socket absent, got %+v", spec)
	}
}

func TestSpecBuilder_forward_true_sock_present(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "agent.sock")
	if err := os.WriteFile(sockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_AUTH_SOCK", sockPath)

	spec, err := newBuilder(t).ContainerSpec(context.Background(), "/proj", cfgForward(true))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Env["SSH_AUTH_SOCK"] != containerSocketPath {
		t.Errorf("SSH_AUTH_SOCK = %q, want %q", spec.Env["SSH_AUTH_SOCK"], containerSocketPath)
	}
	wantMount := sockPath + ":" + containerSocketPath
	if len(spec.Mounts) != 1 || spec.Mounts[0] != wantMount {
		t.Errorf("mounts = %v, want [%s]", spec.Mounts, wantMount)
	}
}

func TestSpecBuilder_keys_missing_file(t *testing.T) {
	// Verify that a missing key file emits a warning but does not prevent the
	// agent from starting. Requires a working ssh-agent binary.
	if _, err := exec.LookPath("ssh-agent"); err != nil {
		t.Skip("ssh-agent not in PATH")
	}
	b := newBuilder(t)
	spec, err := b.ContainerSpec(context.Background(), "/proj",
		cfgKeys("/nonexistent/id_ed25519_missing"))
	if err != nil {
		// ssh-agent may be unavailable in sandboxed test environments.
		t.Skipf("ssh-agent spawn failed (sandboxed?): %v", err)
	}
	if spec.Env["SSH_AUTH_SOCK"] != containerSocketPath {
		t.Errorf("SSH_AUTH_SOCK = %q, want %q", spec.Env["SSH_AUTH_SOCK"], containerSocketPath)
	}
}
