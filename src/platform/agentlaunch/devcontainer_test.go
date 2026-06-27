package agentlaunch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/takezoh/agent-reactor/platform/config"
	"github.com/takezoh/agent-reactor/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-reactor/platform/sandbox/devcontainer"
	"github.com/takezoh/credproxy/container"
)

func TestResolveWorkspaceFallback(t *testing.T) {
	cases := []struct {
		projectPath string
		prefix      string
		want        string
	}{
		{"/home/u/proj", "", "/home/u/proj"},
		{"/home/u/proj", "/mnt", "/mnt/home/u/proj"},
		{"/home/u/proj", "/mnt/", "/mnt/home/u/proj"},
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

func TestStreamSockListenPathMapsIntoRunDir(t *testing.T) {
	// The codex app-server binds at ContainerRunDir/<sock> inside the container;
	// the daemon dials the host path the run-dir bind mount exposes. Pin that the
	// listen path resolves back into the bind-mounted run dir — the invariant
	// e2ab83c established for proxy sockets, here for the stream socket.
	const runDir = "/host/run/4342aed7adbf"
	// toMounts(buildMounts(...)) mirrors exactly what Wrap stores in
	// WrappedLaunch.Mounts, so this exercises the production conversion path.
	w := WrappedLaunch{Mounts: toMounts(buildMounts("", "", runDir, nil))}

	listen := ContainerRunDir + "/codex-sess1.sock"
	got, ok := w.HostPath(listen)
	if !ok {
		t.Fatalf("HostPath(%q) not covered by the run-dir mount", listen)
	}
	if want := runDir + "/codex-sess1.sock"; got != want {
		t.Errorf("dial path = %q, want %q (must live under the bind-mounted run dir)", got, want)
	}
}

func TestBuildMounts_RejectsEmptyWorkspaceContainer(t *testing.T) {
	ms := buildMounts("/host/myapp", "", "/host/run", nil)
	for _, m := range ms {
		if m.Host == "/host/myapp" && m.Container == "" {
			t.Fatalf("empty container target leaked into pathmap: %+v", ms)
		}
	}
}

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

func TestBuildMounts_OmitsWorkspaceWhenBothPathsEmpty(t *testing.T) {
	ms := buildMounts("", "", "/host/run", nil)
	if len(ms) != 1 {
		t.Fatalf("len = %d, want 1 (run dir only): %+v", len(ms), ms)
	}
	if ms[0].Host != "/host/run" {
		t.Errorf("expected only run dir mount, got %+v", ms)
	}
}

func TestBuildMounts_OmitsRunDirWhenEmpty(t *testing.T) {
	ms := buildMounts("/host/myapp", "/workspaces/myapp", "", nil)
	for _, m := range ms {
		if m.Container == ContainerRunDir {
			t.Errorf("run dir mount must be omitted when hostRunDir is empty: %+v", ms)
		}
	}
}

func TestBuildPostCreate_MultipleSubcmds(t *testing.T) {
	bin := "/opt/agent-reactor/run/reactor-bridge"
	subcmds := []string{"setup claude", "setup codex", "setup gemini"}
	got := buildPostCreate(bin, subcmds, nil)
	if len(got) != 3 || got[0] != "bash" || got[1] != "-lc" {
		t.Fatalf("unexpected argv prefix: %v", got)
	}
	// `set -e` must be the FIRST line so a failing earlier subcmd aborts
	// the script — otherwise a Claude setup failure would be silently
	// shadowed by the subsequent Gemini setup succeeding (devcontainer up
	// reports OK while Claude hooks go missing).
	want := "set -e\n" + bin + " setup claude\n" + bin + " setup codex\n" + bin + " setup gemini"
	if got[2] != want {
		t.Errorf("script = %q, want %q", got[2], want)
	}
}

func TestBuildPostCreate_EmptySubcmds(t *testing.T) {
	got := buildPostCreate("/opt/agent-reactor/run/reactor-bridge", nil, nil)
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestBuildOverlayEnv_ContainerPaths(t *testing.T) {
	env := buildOverlayEnv(nil, container.Spec{})
	if got := env["ROOST_SOCKET"]; got != ContainerSockFilePath {
		t.Errorf("ROOST_SOCKET = %q, want %q", got, ContainerSockFilePath)
	}
	if got := env["ROOST_DATA_DIR"]; got != ContainerRunDir {
		t.Errorf("ROOST_DATA_DIR = %q, want %q", got, ContainerRunDir)
	}
}

func TestBuildMounts_DeduplicatesWorkspaceAndRunDir(t *testing.T) {
	binds := []sandboxdc.BindMount{
		{Source: "/host/myapp", Target: "/workspaces/myapp"},
		{Source: "/host/run", Target: ContainerRunDir},
		{Source: "/home/take/.claude/projects", Target: "/home/ubuntu/.claude/projects"},
	}
	ms := buildMounts("/host/myapp", "/workspaces/myapp", "/host/run", binds)
	if len(ms) != 3 {
		t.Fatalf("len = %d, want 3 (ws + run + claude/projects): %+v", len(ms), ms)
	}
}

func TestSharedWorkspaceBindMounts_EnumeratesProjects(t *testing.T) {
	root := t.TempDir()
	projA := filepath.Join(root, "proj-a")
	projB := filepath.Join(root, "proj-b")
	hidden := filepath.Join(root, ".hidden")
	for _, d := range []string{projA, projB, hidden} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	direct := t.TempDir()

	projects := config.ProjectsConfig{
		ProjectRoots: []string{root},
		ProjectPaths: []string{direct},
	}
	binds := sharedWorkspaceBindMounts(projects, "")

	bySource := map[string]string{}
	for _, b := range binds {
		bySource[b.Source] = b.Target
	}
	if _, ok := bySource[projA]; !ok {
		t.Errorf("expected proj-a in binds: %+v", binds)
	}
	if _, ok := bySource[projB]; !ok {
		t.Errorf("expected proj-b in binds: %+v", binds)
	}
	if _, ok := bySource[direct]; !ok {
		t.Errorf("expected direct in binds: %+v", binds)
	}
	if _, ok := bySource[hidden]; ok {
		t.Errorf("hidden dir must not appear in binds: %+v", binds)
	}
}

func TestSharedWorkspaceBindMounts_WithPrefix(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "myapp")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	projects := config.ProjectsConfig{ProjectRoots: []string{root}}
	binds := sharedWorkspaceBindMounts(projects, "/mnt")
	if len(binds) != 1 {
		t.Fatalf("expected 1 bind, got %v", binds)
	}
	want := "/mnt" + proj
	if binds[0].Target != want {
		t.Errorf("Target = %q, want %q", binds[0].Target, want)
	}
}

func TestSharedWorkspaceBindMounts_ProjectMode_ReturnsNothing(t *testing.T) {
	binds := sharedWorkspaceBindMounts(config.ProjectsConfig{}, "")
	if len(binds) != 0 {
		t.Errorf("expected no binds for empty config, got %v", binds)
	}
}

// TestSharedOverlay_ProxyDirMatchesRunDir pins the invariant whose violation
// broke gh/ssh inside shared containers. credproxy providers create their
// per-project sockets (hostexec.sock, agent.sock, hostexec-shims/) under
// runBase/container.ProjectRunHash(OverlayProject); the overlay bind-mounts
// ProjectRunDir(runBase, ContainerKey) to ContainerRunDir. For the shared
// container these MUST resolve to the same host directory, or the proxy
// sockets never appear under /opt/agent-reactor/run and gh/ssh fail. IsolationPlan
// defines OverlayProject as ContainerKey, so the two can no longer diverge;
// this guards a future refactor that re-splits them.
func TestSharedOverlay_ProxyDirMatchesRunDir(t *testing.T) {
	const runBase = "/data/run"
	plan := sandbox.IsolationPlan{Kind: sandbox.IsolationShared}

	bindDir := ProjectRunDir(runBase, plan.ContainerKey("/workspace/fintech"))
	proxyDir := filepath.Join(runBase, container.ProjectRunHash(plan.OverlayProject("/workspace/fintech")))

	if proxyDir != bindDir {
		t.Errorf("shared proxy socket dir %q != run-dir bind %q; gh/ssh sockets unreachable in container",
			proxyDir, bindDir)
	}
}

func TestRunDirKey(t *testing.T) {
	cases := []struct {
		name        string
		isolation   string
		projectPath string
		want        string
	}{
		{
			name:        "shared mode collapses to SharedContainerKey",
			isolation:   "shared",
			projectPath: "/workspace/agent-roost",
			want:        sandboxdc.SharedContainerKey,
		},
		{
			name:        "project mode keeps project path",
			isolation:   "project",
			projectPath: "/workspace/agent-roost",
			want:        "/workspace/agent-roost",
		},
		{
			name:        "default (no isolation) is project-mode keyed by project",
			isolation:   "",
			projectPath: "/workspace/agent-roost",
			want:        "/workspace/agent-roost",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := &DevcontainerLauncher{
				resolveSandbox: func(string) config.SandboxConfig {
					return config.SandboxConfig{Isolation: tc.isolation}
				},
				resolveProjectScope: func(string) *config.SandboxConfig { return nil },
			}
			if got := l.RunDirKey(tc.projectPath); got != tc.want {
				t.Errorf("RunDirKey(%q) = %q, want %q", tc.projectPath, got, tc.want)
			}
		})
	}
}

func TestStartOptionsFor_PropagatesIsolation(t *testing.T) {
	l := &DevcontainerLauncher{
		resolveSandbox: func(string) config.SandboxConfig {
			return config.SandboxConfig{Isolation: "shared"}
		},
		resolveProjectScope: func(string) *config.SandboxConfig { return nil },
	}
	opts := l.StartOptionsFor("/workspace/agent-roost")
	if !opts.Isolation.IsShared() {
		t.Errorf("shared isolation must produce a shared plan, got %+v", opts)
	}

	l.resolveSandbox = func(string) config.SandboxConfig { return config.SandboxConfig{} }
	opts = l.StartOptionsFor("/workspace/agent-roost")
	if opts.Isolation.IsShared() {
		t.Errorf("empty isolation must produce a project plan (default), got %+v", opts)
	}
}

func TestColdStart_PropagatesToStartOptions(t *testing.T) {
	l := &DevcontainerLauncher{
		resolveSandbox: func(string) config.SandboxConfig {
			return config.SandboxConfig{Isolation: "shared"}
		},
		resolveProjectScope: func(string) *config.SandboxConfig { return nil },
	}

	if opts := l.StartOptionsFor("/workspace/agent-roost"); opts.ColdStart {
		t.Fatalf("default StartOptions must have ColdStart=false; got %+v", opts)
	}

	l.BeginColdStart()
	if opts := l.StartOptionsFor("/workspace/agent-roost"); !opts.ColdStart {
		t.Errorf("BeginColdStart must set ColdStart=true on subsequent StartOptions; got %+v", opts)
	}
	if opts := l.StartOptionsFor("/workspace/fintech"); !opts.ColdStart {
		t.Errorf("ColdStart must be true for every project while the window is open; got %+v", opts)
	}

	l.EndColdStart()
	if opts := l.StartOptionsFor("/workspace/agent-roost"); opts.ColdStart {
		t.Errorf("EndColdStart must reset ColdStart=false; got %+v", opts)
	}
}

func TestDevcontainerLauncher_ImplementsColdStartAware(t *testing.T) {
	var _ ColdStartAware = (*DevcontainerLauncher)(nil)
}

func TestResolveStartOptions_ProjectScopeForcesProject(t *testing.T) {
	l := &DevcontainerLauncher{
		resolveSandbox: func(string) config.SandboxConfig {
			return config.SandboxConfig{Isolation: "shared"}
		},
		resolveProjectScope: func(string) *config.SandboxConfig {
			return &config.SandboxConfig{Isolation: "project"}
		},
	}
	opts := l.resolveStartOptions("/workspace/myapp")
	if opts.Isolation.IsShared() {
		t.Errorf("project-scope isolation=project must win; got a shared plan")
	}
}

func TestResolveStartOptions_ProjectScopeDevcontainerPath(t *testing.T) {
	l := &DevcontainerLauncher{
		resolveSandbox: func(string) config.SandboxConfig {
			return config.SandboxConfig{Isolation: "shared"}
		},
		resolveProjectScope: func(string) *config.SandboxConfig {
			return &config.SandboxConfig{
				Devcontainer: config.DevcontainerConfig{Path: "/some/dir"},
			}
		},
	}
	opts := l.resolveStartOptions("/workspace/myapp")
	if opts.Isolation.IsShared() {
		t.Errorf("project-scope devcontainer path must force project-mode; got a shared plan")
	}
	if opts.Isolation.DevcontainerDir == "" {
		t.Errorf("expected DevcontainerDir to be propagated from project scope")
	}
}

func stubHelperBinaries(t *testing.T) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable: %v", err)
	}
	dir := filepath.Dir(exe)
	for _, name := range []string{"reactor-bridge"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Skipf("write %s: %v", p, err)
		}
		t.Cleanup(func() { _ = os.Remove(p) })
	}
}

func TestResolveFrameContext_ProjectMode_UsesProjectPath(t *testing.T) {
	l := &DevcontainerLauncher{
		resolveSandbox:      func(string) config.SandboxConfig { return config.SandboxConfig{} },
		resolveProjectScope: func(string) *config.SandboxConfig { return nil },
	}
	ctx, err := l.ResolveFrameContext(context.Background(), "/workspace/credproxy", "frame-1")
	if err != nil {
		t.Fatalf("ResolveFrameContext: %v", err)
	}
	if ctx.FrameID != "frame-1" {
		t.Errorf("FrameID = %q, want frame-1", ctx.FrameID)
	}
}

func TestResolveFrameContext_SharedMode_DropsProject(t *testing.T) {
	var lastKey string
	l := &DevcontainerLauncher{
		resolveSandbox: func(p string) config.SandboxConfig {
			lastKey = p
			return config.SandboxConfig{Isolation: "shared"}
		},
		resolveProjectScope: func(string) *config.SandboxConfig { return nil },
	}
	_, err := l.ResolveFrameContext(context.Background(), "/workspace/fintech", "frame-1")
	if err != nil {
		t.Fatalf("ResolveFrameContext: %v", err)
	}
	// Shared mode must NOT resolve the real project (user-scope config), and the
	// key must be the SharedContainerKey so the proxy socket dir matches the
	// shared run-dir bind. Resolve maps this non-absolute sentinel to user config.
	if lastKey != sandboxdc.SharedContainerKey {
		t.Errorf("shared mode: resolveSandbox called with %q, want %q", lastKey, sandboxdc.SharedContainerKey)
	}
}

func TestFrameScopeEnv_DropsContainerScopeAndPlaceholders(t *testing.T) {
	in := map[string]string{
		"PATH":               "/opt/agent-reactor/run/hostexec-shims:$PATH",
		"ROOST_SOCKET":       "/opt/agent-reactor/run/server.sock",
		"ROOST_DATA_DIR":     "/opt/agent-reactor/run",
		"SSH_AUTH_SOCK":      "/opt/agent-reactor/run/agent.sock",
		"AWS_PROFILE":        "prod",
		"GCP_PROJECT":        "my-proj",
		"NESTED_PLACEHOLDER": "${SOME_OTHER}/bin:/usr/bin",
	}
	out := frameScopeEnv(in)

	mustKeep := []string{"AWS_PROFILE", "GCP_PROJECT"}
	for _, k := range mustKeep {
		if _, ok := out[k]; !ok {
			t.Errorf("expected %s to pass through frameScopeEnv, got %v", k, out)
		}
	}
	mustDrop := []string{"PATH", "ROOST_SOCKET", "ROOST_DATA_DIR", "SSH_AUTH_SOCK", "NESTED_PLACEHOLDER"}
	for _, k := range mustDrop {
		if _, ok := out[k]; ok {
			t.Errorf("expected %s to be dropped by frameScopeEnv, got %v", k, out)
		}
	}
}

func TestResolveFrameContext_ProjectMode_PassesProjectPath(t *testing.T) {
	var lastKey string
	l := &DevcontainerLauncher{
		resolveSandbox: func(p string) config.SandboxConfig {
			lastKey = p
			return config.SandboxConfig{}
		},
		resolveProjectScope: func(string) *config.SandboxConfig { return nil },
	}
	_, err := l.ResolveFrameContext(context.Background(), "/workspace/myapp", "frame-1")
	if err != nil {
		t.Fatalf("ResolveFrameContext: %v", err)
	}
	if lastKey != "/workspace/myapp" {
		t.Errorf("project mode: resolveSandbox called with %q, want /workspace/myapp", lastKey)
	}
}

func TestResolveFrameContext_EmptyProjectPath(t *testing.T) {
	l := &DevcontainerLauncher{
		resolveSandbox:      func(string) config.SandboxConfig { return config.SandboxConfig{} },
		resolveProjectScope: func(string) *config.SandboxConfig { return nil },
	}
	ctx, err := l.ResolveFrameContext(context.Background(), "", "frame-1")
	if err != nil {
		t.Fatalf("ResolveFrameContext: %v", err)
	}
	if ctx.FrameID != "frame-1" {
		t.Errorf("FrameID = %q", ctx.FrameID)
	}
}

func TestBuildContainerOverlay_SharedMode_UsesUserScope(t *testing.T) {
	stubHelperBinaries(t)
	var lastConfigKey string
	resolveSandbox := func(key string) config.SandboxConfig {
		lastConfigKey = key
		return config.SandboxConfig{}
	}
	dataDir := t.TempDir()
	overlay := BuildContainerOverlay(resolveSandbox, config.ProjectsConfig{}, nil, dataDir, nil)

	sharedPlan := sandbox.IsolationPlan{Kind: sandbox.IsolationShared}
	projectPlan := sandbox.IsolationPlan{Kind: sandbox.IsolationProject}

	if _, err := overlay(sharedPlan, "/workspace/fintech", "/tmp/dc"); err != nil {
		t.Fatalf("shared overlay: %v", err)
	}
	if lastConfigKey != sandboxdc.SharedContainerKey {
		t.Errorf("shared mode: resolveSandbox got %q, want %q (user scope via non-abs sentinel)", lastConfigKey, sandboxdc.SharedContainerKey)
	}

	if _, err := overlay(projectPlan, "/workspace/myapp", "/tmp/dc"); err != nil {
		t.Fatalf("project overlay: %v", err)
	}
	if lastConfigKey != "/workspace/myapp" {
		t.Errorf("project mode: resolveSandbox got %q, want /workspace/myapp", lastConfigKey)
	}
}

func TestBuildContainerOverlay_SharedMode_WorkspaceFallbackIsEmpty(t *testing.T) {
	stubHelperBinaries(t)
	resolveSandbox := func(string) config.SandboxConfig {
		return config.SandboxConfig{
			Devcontainer: config.DevcontainerConfig{HostPathMountPrefix: ""},
		}
	}
	dataDir := t.TempDir()
	overlay := BuildContainerOverlay(resolveSandbox, config.ProjectsConfig{}, nil, dataDir, nil)

	ov, err := overlay(sandbox.IsolationPlan{Kind: sandbox.IsolationShared}, "/workspace/fintech", "/tmp/dc")
	if err != nil {
		t.Fatalf("overlay: %v", err)
	}
	if ov.WorkspaceFolderFallback != "" {
		t.Errorf("shared mode WorkspaceFolderFallback = %q, want \"\"", ov.WorkspaceFolderFallback)
	}
}

func TestBuildContainerOverlay_ProjectMode_WorkspaceFallbackUsesProject(t *testing.T) {
	stubHelperBinaries(t)
	resolveSandbox := func(string) config.SandboxConfig {
		return config.SandboxConfig{
			Devcontainer: config.DevcontainerConfig{HostPathMountPrefix: "/mnt"},
		}
	}
	dataDir := t.TempDir()
	overlay := BuildContainerOverlay(resolveSandbox, config.ProjectsConfig{}, nil, dataDir, nil)

	ov, err := overlay(sandbox.IsolationPlan{Kind: sandbox.IsolationProject}, "/workspace/myapp", "/tmp/dc")
	if err != nil {
		t.Fatalf("overlay: %v", err)
	}
	if ov.WorkspaceFolderFallback != "/mnt/workspace/myapp" {
		t.Errorf("project mode WorkspaceFolderFallback = %q, want /mnt/workspace/myapp",
			ov.WorkspaceFolderFallback)
	}
}

func TestBuildProviderHooks_NonSharedProject(t *testing.T) {
	resolve := func(string) config.SandboxConfig { return config.SandboxConfig{} }
	hooks := BuildProviderHooks(resolve, config.ProjectsConfig{})

	if got := hooks.HostExecWorkspaceFolder("/home/u/proj"); got != "/home/u/proj" {
		t.Errorf("HostExecWorkspaceFolder = %q, want /home/u/proj", got)
	}
	targets := hooks.MCPWorkspaceTargets("/home/u/proj")
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %+v", len(targets), targets)
	}
	if targets[0].HostRoot != "/home/u/proj" || targets[0].ContainerWS != "/home/u/proj" {
		t.Errorf("target = %+v, want {/home/u/proj /home/u/proj}", targets[0])
	}
}

func TestBuildProviderHooks_WithPrefix(t *testing.T) {
	resolve := func(string) config.SandboxConfig {
		return config.SandboxConfig{Devcontainer: config.DevcontainerConfig{HostPathMountPrefix: "/mnt"}}
	}
	hooks := BuildProviderHooks(resolve, config.ProjectsConfig{})

	if got := hooks.HostExecWorkspaceFolder("/home/u/proj"); got != "/mnt/home/u/proj" {
		t.Errorf("HostExecWorkspaceFolder = %q, want /mnt/home/u/proj", got)
	}
	targets := hooks.MCPWorkspaceTargets("/home/u/proj")
	if len(targets) != 1 || targets[0].ContainerWS != "/mnt/home/u/proj" {
		t.Errorf("targets = %+v, want ContainerWS /mnt/home/u/proj", targets)
	}
}

// TestBuildProviderHooks_SharedFansOut is the regression for the shared
// container: the MCP overlay must produce one absolute target per bound project,
// never a single relative sentinel target that Docker rejects.
func TestBuildProviderHooks_SharedFansOut(t *testing.T) {
	root := t.TempDir()
	projA := filepath.Join(root, "proj-a")
	projB := filepath.Join(root, "proj-b")
	for _, d := range []string{projA, projB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	resolve := func(string) config.SandboxConfig { return config.SandboxConfig{} }
	hooks := BuildProviderHooks(resolve, config.ProjectsConfig{ProjectRoots: []string{root}})

	targets := hooks.MCPWorkspaceTargets(sandboxdc.SharedContainerKey)
	got := map[string]string{}
	for _, tg := range targets {
		if !filepath.IsAbs(tg.ContainerWS) {
			t.Errorf("shared target not absolute: %+v", tg)
		}
		got[tg.HostRoot] = tg.ContainerWS
	}
	if got[projA] != projA || got[projB] != projB {
		t.Errorf("expected overlays for proj-a and proj-b, got %+v", targets)
	}
}

// TestBuildProviderHooks_SharedWithPrefix covers the prefix-set shared case:
// every overlay target must be an absolute, prefixed container path (a prefix
// mis-join would yield a non-absolute target that overlayMounts silently drops).
func TestBuildProviderHooks_SharedWithPrefix(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "app")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) config.SandboxConfig {
		return config.SandboxConfig{Devcontainer: config.DevcontainerConfig{HostPathMountPrefix: "/mnt"}}
	}
	hooks := BuildProviderHooks(resolve, config.ProjectsConfig{ProjectRoots: []string{root}})

	targets := hooks.MCPWorkspaceTargets(sandboxdc.SharedContainerKey)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %+v", len(targets), targets)
	}
	if want := "/mnt" + proj; targets[0].ContainerWS != want {
		t.Errorf("ContainerWS = %q, want %q", targets[0].ContainerWS, want)
	}
	if !filepath.IsAbs(targets[0].ContainerWS) {
		t.Errorf("prefixed shared target must be absolute: %q", targets[0].ContainerWS)
	}
	if targets[0].HostRoot != proj {
		t.Errorf("HostRoot = %q, want %q", targets[0].HostRoot, proj)
	}
}

// TestSharedWorkspaceBindMounts_DedupsOnTarget verifies two project paths that
// collapse to the same container target under a prefix (trailing-slash variant)
// yield a single bind mount — docker rejects duplicate mount targets, and the
// derived MCP overlay files must not collide.
func TestSharedWorkspaceBindMounts_DedupsOnTarget(t *testing.T) {
	dir := t.TempDir()
	projects := config.ProjectsConfig{ProjectPaths: []string{dir, dir + "/"}}
	binds := sharedWorkspaceBindMounts(projects, "/mnt")
	if len(binds) != 1 {
		t.Fatalf("expected 1 deduped bind, got %d: %+v", len(binds), binds)
	}
}
