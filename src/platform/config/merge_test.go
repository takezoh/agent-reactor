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
	user := SandboxConfig{Proxy: ProxyConfig{AWSProfiles: []string{"prod"}}}
	got := MergeSandbox(user, nil)
	if len(got.Proxy.AWSProfiles) != 1 || got.Proxy.AWSProfiles[0] != "prod" {
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

func TestMergeMCPServers_projectAppends(t *testing.T) {
	user := map[string]MCPProxyServer{"obs": {Command: "obs-mcp", Allow: []string{"list_*"}}}
	project := map[string]MCPProxyServer{"fs": {Command: "fs-mcp", Allow: []string{"read_*"}}}
	got := mergeMCPServerMap(user, project)
	if len(got) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(got))
	}
}

func TestMergeMCPServers_projectOverridesOnSameAlias(t *testing.T) {
	user := map[string]MCPProxyServer{"obs": {Command: "old-mcp", Allow: []string{"list_*"}}}
	project := map[string]MCPProxyServer{"obs": {Command: "new-mcp", Allow: []string{"get_*"}}}
	got := mergeMCPServerMap(user, project)
	if len(got) != 1 {
		t.Fatalf("expected 1 server after override, got %d", len(got))
	}
	if got["obs"].Command != "new-mcp" {
		t.Errorf("Command = %q, want new-mcp to win", got["obs"].Command)
	}
	if len(got["obs"].Allow) != 1 || got["obs"].Allow[0] != "get_*" {
		t.Errorf("Allow = %v, want project allow to win", got["obs"].Allow)
	}
}

func TestMergeSandbox_HostExecOverlay_Concat(t *testing.T) {
	user := SandboxConfig{Proxy: ProxyConfig{HostExec: HostExecConfig{Overlay: []OverlayEntry{{Target: "bin/gcloud"}}}}}
	project := &SandboxConfig{Proxy: ProxyConfig{HostExec: HostExecConfig{Overlay: []OverlayEntry{{Target: "tools/aws"}}}}}
	got := MergeSandbox(user, project)
	if len(got.Proxy.HostExec.Overlay) != 2 {
		t.Fatalf("Overlay = %v, want 2 entries", got.Proxy.HostExec.Overlay)
	}
}

func TestMergeSandbox_HostExecOverlay_DedupProjectWins(t *testing.T) {
	user := SandboxConfig{Proxy: ProxyConfig{HostExec: HostExecConfig{Overlay: []OverlayEntry{
		{Target: "bin/gcloud", Allow: []string{"*"}},
		{Target: "tools/aws"},
	}}}}
	project := &SandboxConfig{Proxy: ProxyConfig{HostExec: HostExecConfig{Overlay: []OverlayEntry{
		{Target: "bin/gcloud", Allow: []string{"config *"}},
	}}}}
	got := MergeSandbox(user, project)
	if len(got.Proxy.HostExec.Overlay) != 2 {
		t.Fatalf("Overlay = %v, want 2 entries", got.Proxy.HostExec.Overlay)
	}
	// project entry for bin/gcloud should win
	var gcEntry OverlayEntry
	for _, e := range got.Proxy.HostExec.Overlay {
		if e.Target == "bin/gcloud" {
			gcEntry = e
		}
	}
	if len(gcEntry.Allow) != 1 || gcEntry.Allow[0] != "config *" {
		t.Errorf("bin/gcloud Allow = %v, want [config *] (project wins)", gcEntry.Allow)
	}
}

func TestMergeSandbox_HostExecOverlay_NilProject(t *testing.T) {
	user := SandboxConfig{Proxy: ProxyConfig{HostExec: HostExecConfig{Overlay: []OverlayEntry{{Target: "bin/gcloud"}}}}}
	got := MergeSandbox(user, nil)
	if len(got.Proxy.HostExec.Overlay) != 1 || got.Proxy.HostExec.Overlay[0].Target != "bin/gcloud" {
		t.Errorf("Overlay = %v, want [{Target:bin/gcloud}]", got.Proxy.HostExec.Overlay)
	}
}

func TestMergeSandbox_IsolationOverride(t *testing.T) {
	user := SandboxConfig{Isolation: "shared"}
	project := &SandboxConfig{Isolation: "project"}
	got := MergeSandbox(user, project)
	if got.Isolation != "project" {
		t.Errorf("Isolation = %q, want project (project wins)", got.Isolation)
	}
}

func TestMergeSandbox_IsolationEmpty_UserWins(t *testing.T) {
	user := SandboxConfig{Isolation: "shared"}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if got.Isolation != "shared" {
		t.Errorf("Isolation = %q, want shared (project empty, user wins)", got.Isolation)
	}
}

func TestMergeSandbox_DevcontainerPath_ProjectOverrides(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{Path: "/user/dc"}}
	project := &SandboxConfig{Devcontainer: DevcontainerConfig{Path: "/project/dc"}}
	got := MergeSandbox(user, project)
	if got.Devcontainer.Path != "/project/dc" {
		t.Errorf("Path = %q, want /project/dc (project wins)", got.Devcontainer.Path)
	}
}

func TestMergeSandbox_DevcontainerPath_ProjectEmpty_UserWins(t *testing.T) {
	user := SandboxConfig{Devcontainer: DevcontainerConfig{Path: "/user/dc"}}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if got.Devcontainer.Path != "/user/dc" {
		t.Errorf("Path = %q, want /user/dc (project empty, user wins)", got.Devcontainer.Path)
	}
}

func TestMergeMCPServers_nilProject(t *testing.T) {
	user := map[string]MCPProxyServer{"obs": {Command: "obs-mcp"}}
	got := mergeMCPServerMap(user, nil)
	if len(got) != 1 {
		t.Errorf("mergeMCPServerMap with nil project should return user servers, got %v", got)
	}
	if _, ok := got["obs"]; !ok {
		t.Error("expected key 'obs' in result")
	}
}
