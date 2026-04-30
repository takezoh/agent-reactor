package runtime

import (
	"testing"

	sandboxdc "github.com/takezoh/agent-roost/sandbox/devcontainer"
)

func TestResolveWorkspaceFallback(t *testing.T) {
	cases := []struct {
		projectPath string
		prefix      string
		want        string
	}{
		{"/home/u/proj", "", "/home/u/proj"},
		{"/home/u/proj", "/mnt", "/mnt/home/u/proj"},
		{"/home/u/proj", "/mnt/", "/mnt/home/u/proj"}, // trailing slash normalised
		{"", "", ""},
		{"", "/mnt", "/mnt"},
	}
	for _, tc := range cases {
		got := resolveWorkspaceFallback(tc.projectPath, tc.prefix)
		if got != tc.want {
			t.Errorf("resolveWorkspaceFallback(%q, %q) = %q, want %q", tc.projectPath, tc.prefix, got, tc.want)
		}
	}
}

func TestBuildMounts_RegistersWorkspaceAndRunDir(t *testing.T) {
	ms := buildMounts("/host/myapp", "/workspaces/myapp", "/host/run", nil)
	if len(ms) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(ms), ms)
	}
	if ms[0].Host != "/host/myapp" || ms[0].Container != "/workspaces/myapp" {
		t.Errorf("workspace mount = %+v, want host=/host/myapp container=/workspaces/myapp", ms[0])
	}
	if ms[1].Host != "/host/run" || ms[1].Container != ContainerRunDir {
		t.Errorf("run dir mount = %+v, want host=/host/run container=%s", ms[1], ContainerRunDir)
	}
}

// Regression guard: when devcontainer.json omits workspaceFolder, buildMounts
// must still receive a non-empty container target (via WorkspaceTarget fallback)
// so pathmap can translate hook payload paths back to host. Empty container
// target would silently break TRANSCRIPT/EVENTS routing.
func TestBuildMounts_RejectsEmptyWorkspaceContainer(t *testing.T) {
	ms := buildMounts("/host/myapp", "", "/host/run", nil)
	for _, m := range ms {
		if m.Host == "/host/myapp" && m.Container == "" {
			t.Fatalf("empty container target leaked into pathmap: %+v", ms)
		}
	}
}

// Regression guard for the empty-TRANSCRIPT bug: user-declared bind mounts
// (e.g. ~/.claude/projects from extra_create_args) must end up in pathmap so
// transcript_path translation succeeds. Without these mounts, hook payloads
// get cleared at the IPC boundary and the TRANSCRIPT tab stays empty.
func TestBuildMounts_IncludesUserBindMounts(t *testing.T) {
	binds := []sandboxdc.BindMount{
		{Source: "/home/take/.claude/projects", Target: "/home/ubuntu/.claude/projects"},
		{Source: "/home/take/.claude/sessions", Target: "/home/ubuntu/.claude/sessions"},
	}
	ms := buildMounts("/host/myapp", "/workspaces/myapp", "/host/run", binds)

	want := map[string]string{
		"/home/take/.claude/projects": "/home/ubuntu/.claude/projects",
		"/home/take/.claude/sessions": "/home/ubuntu/.claude/sessions",
	}
	for hostP, containerP := range want {
		found := false
		for _, m := range ms {
			if m.Host == hostP && m.Container == containerP {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected mount host=%q container=%q not found in %+v", hostP, containerP, ms)
		}
	}
}

// When both host and container workspace paths are empty, no workspace mount
// should be emitted; only the run-dir mount remains.
func TestBuildMounts_OmitsWorkspaceWhenBothPathsEmpty(t *testing.T) {
	ms := buildMounts("", "", "/host/run", nil)
	if len(ms) != 1 {
		t.Fatalf("len = %d, want 1 (run dir only): %+v", len(ms), ms)
	}
	if ms[0].Host != "/host/run" {
		t.Errorf("expected only run dir mount, got %+v", ms)
	}
}

// When hostRunDir is empty (e.g. dataDir not configured), run-dir mount is skipped.
func TestBuildMounts_OmitsRunDirWhenEmpty(t *testing.T) {
	ms := buildMounts("/host/myapp", "/workspaces/myapp", "", nil)
	for _, m := range ms {
		if m.Container == ContainerRunDir {
			t.Errorf("run dir mount must be omitted when hostRunDir is empty: %+v", ms)
		}
	}
}

// Workspace and run-dir mounts that would be emitted twice (once from defaults,
// once from user binds) must be deduplicated.
func TestBuildMounts_DeduplicatesWorkspaceAndRunDir(t *testing.T) {
	binds := []sandboxdc.BindMount{
		{Source: "/host/myapp", Target: "/workspaces/myapp"}, // duplicate of workspace
		{Source: "/host/run", Target: ContainerRunDir},       // duplicate of run dir
		{Source: "/home/take/.claude/projects", Target: "/home/ubuntu/.claude/projects"},
	}
	ms := buildMounts("/host/myapp", "/workspaces/myapp", "/host/run", binds)
	if len(ms) != 3 {
		t.Fatalf("len = %d, want 3 (ws + run + claude/projects): %+v", len(ms), ms)
	}
}
