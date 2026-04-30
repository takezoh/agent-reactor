package config

import (
	"testing"
)

func TestMergeSandbox_NilProject(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer", Devcontainer: DevcontainerConfig{EnvScript: "~/bin/env.sh"}}
	got := MergeSandbox(user, nil)
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer", got.Mode)
	}
	if got.Devcontainer.EnvScript != "~/bin/env.sh" {
		t.Errorf("EnvScript = %q, want ~/bin/env.sh", got.Devcontainer.EnvScript)
	}
}

func TestMergeSandbox_ModeOverride(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	project := &SandboxConfig{Mode: "direct"}
	got := MergeSandbox(user, project)
	if got.Mode != "direct" {
		t.Errorf("Mode = %q, want direct (project wins)", got.Mode)
	}
}

func TestMergeSandbox_ModeEmpty_UserWins(t *testing.T) {
	user := SandboxConfig{Mode: "devcontainer"}
	project := &SandboxConfig{} // no mode set
	got := MergeSandbox(user, project)
	if got.Mode != "devcontainer" {
		t.Errorf("Mode = %q, want devcontainer (project empty, user wins)", got.Mode)
	}
}

func TestMergeSandbox_ExtraCreateArgsConcat(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{ExtraCreateArgs: []string{"--shm-size=2g"}}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{ExtraCreateArgs: []string{"--ulimit=nofile=1024"}}}
	got := MergeSandbox(user, project)
	if len(got.Devcontainer.ExtraCreateArgs) != 2 {
		t.Errorf("ExtraCreateArgs = %v, want 2 items (user+project concat)", got.Devcontainer.ExtraCreateArgs)
	}
}

func TestMergeSandbox_ExtraCreateArgs_ProjectEmpty(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{ExtraCreateArgs: []string{"--shm-size=2g"}}}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if len(got.Devcontainer.ExtraCreateArgs) != 1 {
		t.Errorf("ExtraCreateArgs = %v, want 1 item (user only)", got.Devcontainer.ExtraCreateArgs)
	}
}

func TestMergeSandbox_EnvScriptOverride(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{EnvScript: "~/bin/roost-env.sh"}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{EnvScript: "./local-env.sh"}}
	got := MergeSandbox(user, project)
	if got.Devcontainer.EnvScript != "./local-env.sh" {
		t.Errorf("EnvScript = %q, want ./local-env.sh (project wins)", got.Devcontainer.EnvScript)
	}
}

func TestMergeSandbox_HostPathMountPrefix_ProjectOverrides(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{HostPathMountPrefix: "/mnt"}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{HostPathMountPrefix: "/data"}}
	got := MergeSandbox(user, project)
	if got.Devcontainer.HostPathMountPrefix != "/data" {
		t.Errorf("HostPathMountPrefix = %q, want /data (project wins)", got.Devcontainer.HostPathMountPrefix)
	}
}

func TestMergeSandbox_HostPathMountPrefix_ProjectEmptyUserWins(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{HostPathMountPrefix: "/mnt"}}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if got.Devcontainer.HostPathMountPrefix != "/mnt" {
		t.Errorf("HostPathMountPrefix = %q, want /mnt (project empty, user wins)", got.Devcontainer.HostPathMountPrefix)
	}
}

func TestMergeSandbox_HostPathMountPrefix_UserOnly(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{HostPathMountPrefix: "/mnt"}}
	got := MergeSandbox(user, nil)
	if got.Devcontainer.HostPathMountPrefix != "/mnt" {
		t.Errorf("HostPathMountPrefix = %q, want /mnt (nil project)", got.Devcontainer.HostPathMountPrefix)
	}
}

func TestMergeSandbox_HostPathMountPrefix_BothEmpty(t *testing.T) {
	user := SandboxConfig{}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if got.Devcontainer.HostPathMountPrefix != "" {
		t.Errorf("HostPathMountPrefix = %q, want empty", got.Devcontainer.HostPathMountPrefix)
	}
}

func TestMergeSandbox_ProxyNilProject(t *testing.T) {
	user := SandboxConfig{Proxy: ProxyConfig{Enabled: true}}
	got := MergeSandbox(user, nil)
	if !got.Proxy.Enabled {
		t.Errorf("proxy config lost on nil project: %+v", got.Proxy)
	}
}

func TestMergeSandbox_DoesNotMutateInput(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{ExtraCreateArgs: []string{"--a"}}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{ExtraCreateArgs: []string{"--b"}}}
	got := MergeSandbox(user, project)
	got.Devcontainer.ExtraCreateArgs = append(got.Devcontainer.ExtraCreateArgs, "--c")
	if len(user.Devcontainer.ExtraCreateArgs) != 1 {
		t.Errorf("user ExtraCreateArgs mutated: %v", user.Devcontainer.ExtraCreateArgs)
	}
	if len(project.Devcontainer.ExtraCreateArgs) != 1 {
		t.Errorf("project ExtraCreateArgs mutated: %v", project.Devcontainer.ExtraCreateArgs)
	}
}
