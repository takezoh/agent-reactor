package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_WinExec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`
[sandbox.proxy]
enabled = true

[sandbox.proxy.win_exec]
allowed_exes = ["code.exe", "explorer.exe"]

[sandbox.proxy.win_exec.resolve]
"code.exe" = "/mnt/c/Users/take/vscode/bin/code.exe"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	wx := cfg.Sandbox.Proxy.WinExec
	if len(wx.AllowedExes) != 2 {
		t.Errorf("AllowedExes = %v, want 2 entries", wx.AllowedExes)
	}
	if wx.Resolve["code.exe"] != "/mnt/c/Users/take/vscode/bin/code.exe" {
		t.Errorf("Resolve[code.exe] = %q, unexpected", wx.Resolve["code.exe"])
	}
}

func TestMergeWinExec_AllowedExesProjectReplaces(t *testing.T) {
	user := SandboxConfig{
		Proxy: ProxyConfig{
			WinExec: WinExecConfig{
				AllowedExes: []string{"code.exe", "explorer.exe"},
			},
		},
	}
	project := &SandboxConfig{
		Proxy: ProxyConfig{
			WinExec: WinExecConfig{
				AllowedExes: []string{"clip.exe"},
			},
		},
	}
	got := MergeSandbox(user, project)
	if len(got.Proxy.WinExec.AllowedExes) != 1 || got.Proxy.WinExec.AllowedExes[0] != "clip.exe" {
		t.Errorf("AllowedExes = %v, want [clip.exe] (project replaces)", got.Proxy.WinExec.AllowedExes)
	}
}

func TestMergeWinExec_AllowedExesUserWinsWhenProjectEmpty(t *testing.T) {
	user := SandboxConfig{
		Proxy: ProxyConfig{
			WinExec: WinExecConfig{
				AllowedExes: []string{"code.exe"},
			},
		},
	}
	project := &SandboxConfig{}
	got := MergeSandbox(user, project)
	if len(got.Proxy.WinExec.AllowedExes) != 1 || got.Proxy.WinExec.AllowedExes[0] != "code.exe" {
		t.Errorf("AllowedExes = %v, want [code.exe] (user wins)", got.Proxy.WinExec.AllowedExes)
	}
}

func TestMergeWinExec_ResolveMapMerged(t *testing.T) {
	user := SandboxConfig{
		Proxy: ProxyConfig{
			WinExec: WinExecConfig{
				Resolve: map[string]string{
					"code.exe":     "/user/path/code",
					"explorer.exe": "/user/path/explorer",
				},
			},
		},
	}
	project := &SandboxConfig{
		Proxy: ProxyConfig{
			WinExec: WinExecConfig{
				Resolve: map[string]string{
					"code.exe": "/project/path/code", // project overrides
					"op.exe":   "/project/path/op",   // project adds
				},
			},
		},
	}
	got := MergeSandbox(user, project)
	res := got.Proxy.WinExec.Resolve
	if res["code.exe"] != "/project/path/code" {
		t.Errorf("Resolve[code.exe] = %q, want /project/path/code (project wins)", res["code.exe"])
	}
	if res["explorer.exe"] != "/user/path/explorer" {
		t.Errorf("Resolve[explorer.exe] = %q, want /user/path/explorer (user key preserved)", res["explorer.exe"])
	}
	if res["op.exe"] != "/project/path/op" {
		t.Errorf("Resolve[op.exe] = %q, want /project/path/op (project added)", res["op.exe"])
	}
}

func TestMergeWinExec_DoesNotMutateInput(t *testing.T) {
	user := SandboxConfig{
		Proxy: ProxyConfig{
			WinExec: WinExecConfig{
				AllowedExes: []string{"code.exe"},
				Resolve:     map[string]string{"code.exe": "/a"},
			},
		},
	}
	project := &SandboxConfig{
		Proxy: ProxyConfig{
			WinExec: WinExecConfig{
				AllowedExes: []string{"clip.exe"},
				Resolve:     map[string]string{"clip.exe": "/b"},
			},
		},
	}
	got := MergeSandbox(user, project)
	got.Proxy.WinExec.AllowedExes = append(got.Proxy.WinExec.AllowedExes, "extra.exe")
	got.Proxy.WinExec.Resolve["new.exe"] = "/c"

	if len(user.Proxy.WinExec.AllowedExes) != 1 {
		t.Error("user AllowedExes mutated")
	}
	if len(project.Proxy.WinExec.AllowedExes) != 1 {
		t.Error("project AllowedExes mutated")
	}
	if _, ok := user.Proxy.WinExec.Resolve["new.exe"]; ok {
		t.Error("user Resolve mutated")
	}
}
