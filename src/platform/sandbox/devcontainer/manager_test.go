package devcontainer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/platform/sandbox"
)

func TestBuildLaunchCommand_subdir(t *testing.T) {
	const project = "/home/take/code/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
	}
	cs := &ContainerState{containerID: "abc123", spec: spec}

	got := translateWorkDir(project+"/backend", project, spec.WorkspaceTarget())
	want := "/workspaces/myapp/backend"
	if got != want {
		t.Errorf("workDir = %q, want %q", got, want)
	}
	_ = cs
}

func TestResolveWorkDir_FrameCtxOverride(t *testing.T) {
	// frameCtx.WorkDir takes priority over every other source.
	spec := &DevcontainerSpec{
		ProjectPath:     "/workspace/fintech",
		Isolation:       IsolationShared,
		WorkspaceFolder: "/workspace/fintech",
	}
	got := resolveWorkDir(spec, "/workspace/credproxy", "/some/other", "/other/project")
	if got != "/workspace/credproxy" {
		t.Errorf("workDir = %q, want /workspace/credproxy", got)
	}
}

func TestResolveWorkDir_Shared_FallsBackToPlanStartDir(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:     "/workspace/fintech",
		Isolation:       IsolationShared,
		WorkspaceFolder: "/workspace/fintech",
	}
	got := resolveWorkDir(spec, "", "/workspace/credproxy", "/workspace/credproxy")
	if got != "/workspace/credproxy" {
		t.Errorf("workDir = %q, want /workspace/credproxy", got)
	}
}

func TestResolveWorkDir_Shared_EmptyAll_FallsBackToWorkspaceTarget(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:     "/workspace/fintech",
		Isolation:       IsolationShared,
		WorkspaceFolder: "/workspace/fintech",
	}
	got := resolveWorkDir(spec, "", "", "/workspace/credproxy")
	if got != "/workspace/fintech" {
		t.Errorf("workDir = %q, want /workspace/fintech", got)
	}
}

func TestResolveWorkDir_Project_TranslatesHostPath(t *testing.T) {
	const project = "/home/take/code/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		Isolation:       IsolationProject,
		WorkspaceFolder: "/workspaces/myapp",
	}
	got := resolveWorkDir(spec, "", project+"/backend", project)
	if got != "/workspaces/myapp/backend" {
		t.Errorf("workDir = %q, want /workspaces/myapp/backend", got)
	}
}

func TestContainerState_WorkspaceTarget(t *testing.T) {
	t.Run("fallback when WorkspaceFolder empty", func(t *testing.T) {
		cs := &ContainerState{spec: &DevcontainerSpec{ProjectPath: "/workspace/myapp"}}
		if got := cs.WorkspaceTarget(); got != "/workspace/myapp" {
			t.Errorf("WorkspaceTarget = %q, want /workspace/myapp", got)
		}
	})
	t.Run("uses WorkspaceFolder when set", func(t *testing.T) {
		cs := &ContainerState{spec: &DevcontainerSpec{
			ProjectPath:     "/workspace/myapp",
			WorkspaceFolder: "/custom/ws",
		}}
		if got := cs.WorkspaceTarget(); got != "/custom/ws" {
			t.Errorf("WorkspaceTarget = %q, want /custom/ws", got)
		}
	})
	t.Run("nil ContainerState", func(t *testing.T) {
		var cs *ContainerState
		if got := cs.WorkspaceTarget(); got != "" {
			t.Errorf("WorkspaceTarget = %q, want empty for nil", got)
		}
	})
}

func TestDevcontainerSpec_buildCreateArgs_defaults(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:  "/workspace/myapp",
		ContainerEnv: map[string]string{"FOO": "bar"},
	}
	args := spec.BuildCreateArgs("roost-abc123:latest")

	mustContain := func(needle string) {
		t.Helper()
		for _, a := range args {
			if a == needle {
				return
			}
		}
		t.Errorf("args missing %q: %v", needle, args)
	}

	mustContain("roost-abc123:latest")
	mustContain("roost-managed=1")
	mustContain("roost-project=/workspace/myapp")
	mustContain("FOO=bar")
	// -w should set the workspace target as default cwd (replaces Dockerfile WORKDIR).
	mustContain("-w")
	mustContain("/workspace/myapp")
	// workspace mount should be present as --mount arg value (host-mirrored default)
	found := false
	for _, a := range args {
		if a == "type=bind,source=/workspace/myapp,target=/workspace/myapp,consistency=cached" {
			found = true
		}
	}
	if !found {
		t.Errorf("workspace mount not found in args: %v", args)
	}
}

// assertArgBeforeImage verifies that an arg matching pred appears before image in args.
func assertArgBeforeImage(t *testing.T, args []string, image string, pred func(string) bool) {
	t.Helper()
	imageIdx := slices.Index(args, image)
	if imageIdx < 0 {
		t.Fatalf("image %q not found in args: %v", image, args)
	}
	argIdx := slices.IndexFunc(args, pred)
	if argIdx < 0 {
		t.Fatalf("expected arg not found in args: %v", args)
	}
	if argIdx >= imageIdx {
		t.Errorf("arg[%d]=%q must appear before image[%d]=%q; args: %v", argIdx, args[argIdx], imageIdx, image, args)
	}
}

func TestDevcontainerSpec_buildCreateArgs_extraCreateArgsBeforeImage(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:     "/workspace/myapp",
		ContainerEnv:    map[string]string{},
		ExtraCreateArgs: []string{"--mount", "type=bind,source=/home/take/.roost,target=/home/ubuntu/.roost,readonly"},
	}
	args := spec.BuildCreateArgs("myimage:latest")
	assertArgBeforeImage(t, args, "myimage:latest", func(a string) bool {
		return strings.Contains(a, ".roost")
	})
}

func TestSpecOverlay_ExtraCreateArgs(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:  "/workspace/myapp",
		ContainerEnv: map[string]string{},
	}
	spec.Apply(SpecOverlay{ExtraCreateArgs: []string{"--shm-size=2g"}})
	if len(spec.ExtraCreateArgs) != 1 || spec.ExtraCreateArgs[0] != "--shm-size=2g" {
		t.Errorf("ExtraCreateArgs = %v, want [--shm-size=2g]", spec.ExtraCreateArgs)
	}
	args := spec.BuildCreateArgs("img:latest")
	assertArgBeforeImage(t, args, "img:latest", func(a string) bool { return a == "--shm-size=2g" })
}

func TestBuildLaunchCommand_streamDirectCommand(t *testing.T) {
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "ctr42", spec: spec},
	}
	m := &Manager{}
	plan := sandbox.LaunchSpec{
		StartDir: project,
		Command:  "codex resume thr_123 --remote unix:///opt/roost/run/codex-foo.sock",
	}

	got, _, err := m.BuildLaunchCommand(inst, plan, sandbox.FrameContext{}, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	if !strings.Contains(got, "codex resume thr_123 --remote unix:///opt/roost/run/codex-foo.sock") {
		t.Errorf("expected direct codex remote command, got: %s", got)
	}
}

func TestBuildLaunchCommand_shellUsesLoginShell(t *testing.T) {
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
		RemoteUser:      "ubuntu",
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "abc123", spec: spec},
	}
	m := &Manager{}
	plan := sandbox.LaunchSpec{StartDir: project, Command: "shell"}

	got, _, err := m.BuildLaunchCommand(inst, plan, sandbox.FrameContext{}, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	if strings.Contains(got, "/bin/bash") {
		t.Errorf("command must not hardcode /bin/bash: %s", got)
	}
	if !strings.Contains(got, "getent passwd") {
		t.Errorf("command must look up login shell via getent passwd: %s", got)
	}
	if !strings.Contains(got, "sh -c ") {
		t.Errorf("command must wrap snippet in sh -c: %s", got)
	}
}

func TestBuildLaunchCommand_MergesFrameCtxEnv(t *testing.T) {
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "ctr1", spec: spec},
	}
	m := &Manager{}
	plan := sandbox.LaunchSpec{StartDir: project, Command: "bash"}
	frameCtx := sandbox.FrameContext{Env: map[string]string{"AWS_PROFILE": "prod"}}

	got, _, err := m.BuildLaunchCommand(inst, plan, frameCtx, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	if !strings.Contains(got, "AWS_PROFILE=prod") {
		t.Errorf("frameCtx.Env not in command: %s", got)
	}
}

// Regression guard for the shared-mode core bug: two frames from different
// projects must produce two independent docker exec command strings even
// though they share the same Instance / DevcontainerSpec. The spec carries
// only the first project's path (ProjectPath/WorkspaceFolder), so a leak
// would show up as both commands landing in the spec's project.
func TestBuildLaunchCommand_SharedInstance_PerFrameIsolation(t *testing.T) {
	// One shared container, spec pinned to "first project" (= whichever frame
	// triggered the container creation).
	spec := &DevcontainerSpec{
		ProjectPath:     "/workspace/first-project",
		Isolation:       IsolationShared,
		WorkspaceFolder: "/workspace/first-project",
		ContainerEnv:    map[string]string{},
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: "/workspace/first-project",
		Internal:    &ContainerState{containerID: "shared-ctr", spec: spec},
	}
	m := &Manager{}

	// Frame A: /workspace/agent-roost
	planA := sandbox.LaunchSpec{Command: "bash"}
	ctxA := sandbox.FrameContext{
		FrameID: "frame-a",
		WorkDir: "/workspace/agent-roost",
		Env: map[string]string{
			"ROOST_FRAME_ID":     "frame-a",
			"ROOST_SOCKET_TOKEN": "tok-aaa",
		},
	}
	cmdA, _, err := m.BuildLaunchCommand(inst, planA, ctxA, nil)
	if err != nil {
		t.Fatalf("frame A: %v", err)
	}

	// Frame B: /workspace/credproxy (different project, same Instance)
	planB := sandbox.LaunchSpec{Command: "bash"}
	ctxB := sandbox.FrameContext{
		FrameID: "frame-b",
		WorkDir: "/workspace/credproxy",
		Env: map[string]string{
			"ROOST_FRAME_ID":     "frame-b",
			"ROOST_SOCKET_TOKEN": "tok-bbb",
		},
	}
	cmdB, _, err := m.BuildLaunchCommand(inst, planB, ctxB, nil)
	if err != nil {
		t.Fatalf("frame B: %v", err)
	}

	// -w must reflect each frame's project, NOT the spec's first-project path.
	if !strings.Contains(cmdA, "-w '/workspace/agent-roost'") {
		t.Errorf("frame A: -w must point to /workspace/agent-roost; got: %s", cmdA)
	}
	if strings.Contains(cmdA, "-w '/workspace/first-project'") {
		t.Errorf("frame A: spec project leaked into -w; got: %s", cmdA)
	}
	if !strings.Contains(cmdB, "-w '/workspace/credproxy'") {
		t.Errorf("frame B: -w must point to /workspace/credproxy; got: %s", cmdB)
	}
	if strings.Contains(cmdB, "-w '/workspace/first-project'") {
		t.Errorf("frame B: spec project leaked into -w; got: %s", cmdB)
	}

	// Per-frame env must not cross-contaminate.
	if !strings.Contains(cmdA, "ROOST_FRAME_ID=frame-a") || strings.Contains(cmdA, "frame-b") {
		t.Errorf("frame A env leak; got: %s", cmdA)
	}
	if !strings.Contains(cmdA, "ROOST_SOCKET_TOKEN=tok-aaa") || strings.Contains(cmdA, "tok-bbb") {
		t.Errorf("frame A token leak; got: %s", cmdA)
	}
	if !strings.Contains(cmdB, "ROOST_FRAME_ID=frame-b") || strings.Contains(cmdB, "frame-a") {
		t.Errorf("frame B env leak; got: %s", cmdB)
	}
	if !strings.Contains(cmdB, "ROOST_SOCKET_TOKEN=tok-bbb") || strings.Contains(cmdB, "tok-aaa") {
		t.Errorf("frame B token leak; got: %s", cmdB)
	}

	// Both must docker exec into the SAME container id (sanity: shared mode).
	if !strings.Contains(cmdA, "shared-ctr") || !strings.Contains(cmdB, "shared-ctr") {
		t.Errorf("both frames must target same container; A=%s B=%s", cmdA, cmdB)
	}
}

func TestBuildLaunchCommand_FrameCtxEnvWinsOnConflict(t *testing.T) {
	// docker exec applies the last `-e KEY=VAL` so the order we emit is what
	// determines who wins. spec → frameCtx → env; later entries override.
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		RemoteEnv:       map[string]string{"AWS_PROFILE": "default"},
		WorkspaceFolder: "/workspaces/myapp",
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "ctr1", spec: spec},
	}
	m := &Manager{}
	plan := sandbox.LaunchSpec{StartDir: project, Command: "bash"}
	frameCtx := sandbox.FrameContext{Env: map[string]string{"AWS_PROFILE": "prod"}}

	got, _, err := m.BuildLaunchCommand(inst, plan, frameCtx, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	// Both must appear, frameCtx (prod) must appear AFTER spec (default) so
	// docker treats frameCtx as the winning value.
	specIdx := strings.Index(got, "AWS_PROFILE=default")
	ctxIdx := strings.Index(got, "AWS_PROFILE=prod")
	if specIdx < 0 || ctxIdx < 0 {
		t.Fatalf("expected both AWS_PROFILE entries; got: %s", got)
	}
	if ctxIdx <= specIdx {
		t.Errorf("frameCtx.Env must appear after spec.RemoteEnv; spec=%d ctx=%d in %s", specIdx, ctxIdx, got)
	}
}

func TestBuildLaunchCommand_RemoteEnv(t *testing.T) {
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		RemoteEnv:       map[string]string{"MY_VAR": "hello", "PATH": "/extra/bin:/usr/bin"},
		WorkspaceFolder: "/workspaces/myapp",
	}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "ctr123", spec: spec},
	}
	m := &Manager{}
	plan := sandbox.LaunchSpec{StartDir: project, Command: "bash"}

	got, _, err := m.BuildLaunchCommand(inst, plan, sandbox.FrameContext{}, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	for _, want := range []string{"MY_VAR=hello", "PATH=/extra/bin:/usr/bin"} {
		if !strings.Contains(got, want) {
			t.Errorf("command missing %q: %s", want, got)
		}
	}
}

func TestResolveContainerEnvPlaceholders(t *testing.T) {
	imageEnv := map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/bin",
		"TERM": "xterm",
	}

	t.Run("containerEnv placeholder resolved from image", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{
				"PATH": "/home/ubuntu/.local/bin:${containerEnv:PATH}",
			},
			RemoteEnv: map[string]string{},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		got := spec.ContainerEnv["PATH"]
		want := "/home/ubuntu/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/bin"
		if got != want {
			t.Errorf("ContainerEnv[PATH] = %q, want %q", got, want)
		}
	})

	t.Run("remoteEnv sees imageEnv union containerEnv (containerEnv wins)", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{
				"PATH": "/mise/shims:/usr/bin",
			},
			RemoteEnv: map[string]string{
				"PATH": "/extra:${containerEnv:PATH}",
				"TERM": "${containerEnv:TERM}",
			},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		if got, want := spec.RemoteEnv["PATH"], "/extra:/mise/shims:/usr/bin"; got != want {
			t.Errorf("RemoteEnv[PATH] = %q, want %q", got, want)
		}
		if got, want := spec.RemoteEnv["TERM"], "xterm"; got != want {
			t.Errorf("RemoteEnv[TERM] = %q, want %q", got, want)
		}
	})

	t.Run("undefined var becomes empty string", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{"X": "${containerEnv:UNDEFINED}"},
			RemoteEnv:    map[string]string{},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		if got := spec.ContainerEnv["X"]; got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("empty imageEnv (ImageEnv probe failed) preserves explicit containerEnv paths in remoteEnv", func(t *testing.T) {
		// Simulates the ImageEnv-failure fallback: called with empty map instead of being skipped.
		// containerEnv PATH has google-cloud-sdk explicitly listed before ${containerEnv:PATH}.
		// remoteEnv PATH references ${containerEnv:PATH}.
		// After resolution, remoteEnv PATH must include google-cloud-sdk even without image baseline.
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{
				"PATH": "/home/ubuntu/.local/share/google-cloud-sdk/bin:/usr/local/bin:${containerEnv:PATH}",
			},
			RemoteEnv: map[string]string{
				"PATH": "/home/ubuntu/.local/share/mise/shims:${containerEnv:PATH}",
			},
		}
		spec.ResolveContainerEnvPlaceholders(map[string]string{}) // empty = no image baseline
		if !strings.Contains(spec.RemoteEnv["PATH"], "google-cloud-sdk") {
			t.Errorf("RemoteEnv[PATH] = %q, must contain google-cloud-sdk after empty-baseline resolve", spec.RemoteEnv["PATH"])
		}
		if !strings.Contains(spec.ContainerEnv["PATH"], "google-cloud-sdk") {
			t.Errorf("ContainerEnv[PATH] = %q, must contain google-cloud-sdk", spec.ContainerEnv["PATH"])
		}
	})

	t.Run("no placeholder is unchanged", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ContainerEnv: map[string]string{"FOO": "bar"},
			RemoteEnv:    map[string]string{"BAZ": "qux"},
		}
		spec.ResolveContainerEnvPlaceholders(imageEnv)
		if spec.ContainerEnv["FOO"] != "bar" {
			t.Errorf("ContainerEnv[FOO] changed unexpectedly")
		}
		if spec.RemoteEnv["BAZ"] != "qux" {
			t.Errorf("RemoteEnv[BAZ] changed unexpectedly")
		}
	})
}

func TestBuildLaunchCommand_PreExec(t *testing.T) {
	const project = "/workspace/myapp"
	base := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
	}
	m := &Manager{}

	t.Run("preExec wraps command with bash -lc", func(t *testing.T) {
		spec := *base
		spec.PreExec = "mise trust 2>/dev/null || true"
		inst := &sandbox.Instance[*ContainerState]{
			ProjectPath: project,
			Internal:    &ContainerState{containerID: "abc123", spec: &spec},
		}
		got, _, err := m.BuildLaunchCommand(inst, sandbox.LaunchSpec{StartDir: project, Command: "bash"}, sandbox.FrameContext{}, nil)
		if err != nil {
			t.Fatalf("BuildLaunchCommand error: %v", err)
		}
		if !strings.Contains(got, "bash -lc") {
			t.Errorf("expected bash -lc wrapper, got: %s", got)
		}
		if !strings.Contains(got, "mise trust") {
			t.Errorf("expected mise trust in command, got: %s", got)
		}
		if !strings.Contains(got, "exec bash") {
			t.Errorf("expected exec bash in command, got: %s", got)
		}
	})

	t.Run("no preExec leaves command unchanged", func(t *testing.T) {
		spec := *base
		inst := &sandbox.Instance[*ContainerState]{
			ProjectPath: project,
			Internal:    &ContainerState{containerID: "abc123", spec: &spec},
		}
		got, _, err := m.BuildLaunchCommand(inst, sandbox.LaunchSpec{StartDir: project, Command: "bash"}, sandbox.FrameContext{}, nil)
		if err != nil {
			t.Fatalf("BuildLaunchCommand error: %v", err)
		}
		if strings.Contains(got, "bash -lc") {
			t.Errorf("unexpected bash -lc wrapper without PreExec: %s", got)
		}
		if !strings.HasSuffix(got, " bash") {
			t.Errorf("expected command to end with 'bash', got: %s", got)
		}
	})

	t.Run("preExec with shell command retains login shell lookup", func(t *testing.T) {
		spec := *base
		spec.PreExec = "mise trust 2>/dev/null || true"
		inst := &sandbox.Instance[*ContainerState]{
			ProjectPath: project,
			Internal:    &ContainerState{containerID: "abc123", spec: &spec},
		}
		got, _, err := m.BuildLaunchCommand(inst, sandbox.LaunchSpec{StartDir: project, Command: "shell"}, sandbox.FrameContext{}, nil)
		if err != nil {
			t.Fatalf("BuildLaunchCommand error: %v", err)
		}
		if !strings.Contains(got, "mise trust") {
			t.Errorf("expected mise trust in shell command, got: %s", got)
		}
		if !strings.Contains(got, "getent passwd") {
			t.Errorf("expected login shell lookup in shell command, got: %s", got)
		}
	})
}

func TestSpecOverlay_PreExecFallback(t *testing.T) {
	t.Run("overlay PreExec used when spec is empty", func(t *testing.T) {
		s := &DevcontainerSpec{}
		s.Apply(SpecOverlay{PreExec: "mise trust 2>/dev/null || true"})
		if s.PreExec != "mise trust 2>/dev/null || true" {
			t.Errorf("expected overlay PreExec, got %q", s.PreExec)
		}
	})

	t.Run("spec PreExec wins over overlay", func(t *testing.T) {
		s := &DevcontainerSpec{PreExec: "custom-hook"}
		s.Apply(SpecOverlay{PreExec: "mise trust 2>/dev/null || true"})
		if s.PreExec != "custom-hook" {
			t.Errorf("expected spec PreExec to win, got %q", s.PreExec)
		}
	})
}

func TestApplyPathOverlayPrependsToUserPath(t *testing.T) {
	// The hostexec overlay injects "shims:$PATH". It must NOT overwrite the
	// user's explicit containerEnv/remoteEnv PATH (e.g. one containing google-cloud-sdk).
	// It should prepend the shims prefix and preserve the user's value.
	userPath := "/home/ubuntu/.local/share/google-cloud-sdk/bin:/usr/local/bin:${containerEnv:PATH}"
	remotePath := "/home/ubuntu/.local/share/mise/shims:${containerEnv:PATH}"
	s := &DevcontainerSpec{
		ContainerEnv: map[string]string{"PATH": userPath},
		RemoteEnv:    map[string]string{"PATH": remotePath},
	}
	shimsDir := "/opt/roost/run/hostexec-shims"
	s.Apply(SpecOverlay{Env: map[string]string{"PATH": shimsDir + ":$PATH"}})

	if got := s.ContainerEnv["PATH"]; got != shimsDir+":"+userPath {
		t.Errorf("ContainerEnv[PATH] = %q, want shims prepended to user PATH %q", got, shimsDir+":"+userPath)
	}
	if got := s.RemoteEnv["PATH"]; got != shimsDir+":"+remotePath {
		t.Errorf("RemoteEnv[PATH] = %q, want shims prepended to remote PATH %q", got, shimsDir+":"+remotePath)
	}
	// google-cloud-sdk must survive in ContainerEnv
	if !strings.Contains(s.ContainerEnv["PATH"], "google-cloud-sdk") {
		t.Errorf("ContainerEnv[PATH] lost google-cloud-sdk: %s", s.ContainerEnv["PATH"])
	}
}

func TestApplyPathOverlayUsedAsIsWhenNoExistingPath(t *testing.T) {
	s := &DevcontainerSpec{}
	s.Apply(SpecOverlay{Env: map[string]string{"PATH": "/opt/shims:$PATH"}})
	if got := s.ContainerEnv["PATH"]; got != "/opt/shims:$PATH" {
		t.Errorf("ContainerEnv[PATH] = %q, want /opt/shims:$PATH when no existing PATH", got)
	}
}

func TestApplyNonPathOverlayOverridesExisting(t *testing.T) {
	// Non-PATH vars (no ":$VAR" suffix) must override, not prepend.
	s := &DevcontainerSpec{
		ContainerEnv: map[string]string{"SSH_AUTH_SOCK": "/old/agent.sock"},
		RemoteEnv:    map[string]string{"SSH_AUTH_SOCK": "/old/agent.sock"},
	}
	s.Apply(SpecOverlay{Env: map[string]string{"SSH_AUTH_SOCK": "/opt/roost/run/agent.sock"}})
	if got := s.ContainerEnv["SSH_AUTH_SOCK"]; got != "/opt/roost/run/agent.sock" {
		t.Errorf("ContainerEnv[SSH_AUTH_SOCK] = %q, want override", got)
	}
	if got := s.RemoteEnv["SSH_AUTH_SOCK"]; got != "/opt/roost/run/agent.sock" {
		t.Errorf("RemoteEnv[SSH_AUTH_SOCK] = %q, want override", got)
	}
}

func TestApplyMergesOverlayEnvIntoBothContainerAndRemoteEnv(t *testing.T) {
	s := &DevcontainerSpec{
		ContainerEnv: map[string]string{"EXISTING": "yes"},
		RemoteEnv:    map[string]string{"USER_VAR": "from-dc-json"},
	}
	s.Apply(SpecOverlay{Env: map[string]string{"SSH_AUTH_SOCK": "/opt/roost/run/agent.sock", "FOO": "bar"}})

	if got := s.ContainerEnv["SSH_AUTH_SOCK"]; got != "/opt/roost/run/agent.sock" {
		t.Errorf("ContainerEnv[SSH_AUTH_SOCK] = %q, want /opt/roost/run/agent.sock", got)
	}
	if got := s.ContainerEnv["FOO"]; got != "bar" {
		t.Errorf("ContainerEnv[FOO] = %q, want bar", got)
	}
	if got := s.RemoteEnv["SSH_AUTH_SOCK"]; got != "/opt/roost/run/agent.sock" {
		t.Errorf("RemoteEnv[SSH_AUTH_SOCK] = %q, want /opt/roost/run/agent.sock", got)
	}
	if got := s.RemoteEnv["FOO"]; got != "bar" {
		t.Errorf("RemoteEnv[FOO] = %q, want bar", got)
	}
	// pre-existing keys are preserved
	if got := s.ContainerEnv["EXISTING"]; got != "yes" {
		t.Errorf("ContainerEnv[EXISTING] = %q, want yes", got)
	}
	if got := s.RemoteEnv["USER_VAR"]; got != "from-dc-json" {
		t.Errorf("RemoteEnv[USER_VAR] = %q, want from-dc-json", got)
	}
}

func TestApplyOverlayEnvAppearsInBuildLaunchCommand(t *testing.T) {
	const project = "/workspace/myapp"
	spec := &DevcontainerSpec{
		ProjectPath:     project,
		ContainerEnv:    map[string]string{},
		RemoteEnv:       map[string]string{},
		WorkspaceFolder: "/workspaces/myapp",
	}
	spec.Apply(SpecOverlay{Env: map[string]string{"SSH_AUTH_SOCK": "/opt/roost/run/agent.sock"}})

	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    &ContainerState{containerID: "ctr999", spec: spec},
	}
	m := &Manager{}
	plan := sandbox.LaunchSpec{StartDir: project, Command: "claude"}

	got, _, err := m.BuildLaunchCommand(inst, plan, sandbox.FrameContext{}, nil)
	if err != nil {
		t.Fatalf("BuildLaunchCommand error: %v", err)
	}
	if !strings.Contains(got, "SSH_AUTH_SOCK=/opt/roost/run/agent.sock") {
		t.Errorf("docker exec command missing SSH_AUTH_SOCK: %s", got)
	}
}

func TestApplyOverlayPostCreateAppendsToExtraPostCreate(t *testing.T) {
	t.Run("overlay PostCreate stored in ExtraPostCreate", func(t *testing.T) {
		s := &DevcontainerSpec{}
		s.Apply(SpecOverlay{PostCreate: []string{"sh", "-c", "echo hello"}})
		if len(s.ExtraPostCreate) != 1 {
			t.Fatalf("ExtraPostCreate len = %d, want 1", len(s.ExtraPostCreate))
		}
		if got := s.ExtraPostCreate[0]; len(got) != 3 || got[2] != "echo hello" {
			t.Errorf("ExtraPostCreate[0] = %v, want [sh -c echo hello]", got)
		}
	})

	t.Run("base PostCreate is not modified", func(t *testing.T) {
		s := &DevcontainerSpec{PostCreate: []string{"bash", "-lc", "npm install"}}
		s.Apply(SpecOverlay{PostCreate: []string{"sh", "-c", "roost setup"}})
		if len(s.PostCreate) != 3 {
			t.Errorf("PostCreate modified, got len %d", len(s.PostCreate))
		}
		if len(s.ExtraPostCreate) != 1 {
			t.Errorf("ExtraPostCreate len = %d, want 1", len(s.ExtraPostCreate))
		}
	})

	t.Run("empty overlay PostCreate skipped", func(t *testing.T) {
		s := &DevcontainerSpec{}
		s.Apply(SpecOverlay{PostCreate: nil})
		if len(s.ExtraPostCreate) != 0 {
			t.Errorf("ExtraPostCreate should be empty, got %v", s.ExtraPostCreate)
		}
	})
}

func TestResolveContainerEnvPlaceholders_DeduplicatesPath(t *testing.T) {
	// Full layered scenario: Dockerfile ENV, user containerEnv with ${containerEnv:PATH},
	// remoteEnv with ${containerEnv:PATH}, and overlay shims prepended.
	// After resolution the same directory must appear only once in each env.
	imageEnv := map[string]string{
		"PATH": "/home/ubuntu/.local/bin:/home/ubuntu/.local/share/mise/shims:/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:/usr/local/bin:/usr/bin:/bin",
	}
	spec := &DevcontainerSpec{
		ContainerEnv: map[string]string{
			"PATH": "/opt/roost/run/hostexec-shims:/home/ubuntu/.local/bin:/home/ubuntu/.local/share/mise/shims:/home/ubuntu/.local/share/google-cloud-sdk/bin:/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:${containerEnv:PATH}",
		},
		RemoteEnv: map[string]string{
			"PATH": "/opt/roost/run/hostexec-shims:/home/ubuntu/.local/share/mise/shims:${containerEnv:PATH}",
		},
	}
	spec.ResolveContainerEnvPlaceholders(imageEnv)

	checkNoDuplicates := func(t *testing.T, label, val string) {
		t.Helper()
		seen := map[string]int{}
		for _, seg := range strings.Split(val, ":") {
			seen[seg]++
		}
		for seg, n := range seen {
			if n > 1 {
				t.Errorf("%s: %q appears %d times in PATH: %s", label, seg, n, val)
			}
		}
	}
	checkNoDuplicates(t, "ContainerEnv", spec.ContainerEnv["PATH"])
	checkNoDuplicates(t, "RemoteEnv", spec.RemoteEnv["PATH"])

	if !strings.Contains(spec.RemoteEnv["PATH"], "google-cloud-sdk") {
		t.Errorf("RemoteEnv[PATH] missing google-cloud-sdk: %s", spec.RemoteEnv["PATH"])
	}
}

func TestContainerName_project(t *testing.T) {
	s := &DevcontainerSpec{ProjectPath: "/workspace/myapp", Isolation: IsolationProject}
	name := s.ContainerName()
	if name == "roost-shared" {
		t.Errorf("ContainerName for project isolation must not be roost-shared")
	}
	if name[:6] != "roost-" {
		t.Errorf("ContainerName = %q, want roost-<hash>", name)
	}
}

func TestContainerName_shared(t *testing.T) {
	s := &DevcontainerSpec{ProjectPath: "/workspace/myapp", Isolation: IsolationShared}
	if got := s.ContainerName(); got != "roost-shared" {
		t.Errorf("ContainerName = %q, want roost-shared", got)
	}
}

func TestIsShared(t *testing.T) {
	t.Run("shared", func(t *testing.T) {
		cs := &ContainerState{spec: &DevcontainerSpec{Isolation: IsolationShared}}
		if !cs.IsShared() {
			t.Error("expected IsShared() true")
		}
	})
	t.Run("project", func(t *testing.T) {
		cs := &ContainerState{spec: &DevcontainerSpec{Isolation: IsolationProject}}
		if cs.IsShared() {
			t.Error("expected IsShared() false")
		}
	})
	t.Run("nil", func(t *testing.T) {
		var cs *ContainerState
		if cs.IsShared() {
			t.Error("expected IsShared() false for nil")
		}
	})
}

// Regression guard for the "shared container never stops" symptom: shared
// containers must docker stop on DestroyInstance (kept around for reuse) while
// project containers docker rm (per-project lifecycle).
// withMockDockerStack swaps every Manager docker indirection at once. Tests
// drive the full ensureContainer flow without invoking real docker.
func withMockDockerStack(t *testing.T, m dockerStackMocks) {
	t.Helper()
	saved := struct {
		start  func(context.Context, string) error
		stop   func(context.Context, string) error
		rm     func(context.Context, string) error
		create func(context.Context, []string) (string, error)
		find   func(context.Context, string) (*ContainerInfo, error)
		shared func(context.Context) (*ContainerInfo, error)
		image  func(context.Context, string) (map[string]string, error)
		post   func(context.Context, string, string, []string)
	}{
		startContainerFn, stopContainerFn, removeContainerFn,
		createContainerFn, findContainerFn, findSharedContainerFn,
		imageEnvFn, runPostCreateFn,
	}
	t.Cleanup(func() {
		startContainerFn = saved.start
		stopContainerFn = saved.stop
		removeContainerFn = saved.rm
		createContainerFn = saved.create
		findContainerFn = saved.find
		findSharedContainerFn = saved.shared
		imageEnvFn = saved.image
		runPostCreateFn = saved.post
	})
	if m.start != nil {
		startContainerFn = m.start
	}
	if m.stop != nil {
		stopContainerFn = m.stop
	}
	if m.remove != nil {
		removeContainerFn = m.remove
	}
	if m.create != nil {
		createContainerFn = m.create
	}
	if m.find != nil {
		findContainerFn = m.find
	}
	if m.findShared != nil {
		findSharedContainerFn = m.findShared
	}
	if m.imageEnv != nil {
		imageEnvFn = m.imageEnv
	}
	if m.postCreate != nil {
		runPostCreateFn = m.postCreate
	}
}

type dockerStackMocks struct {
	start      func(context.Context, string) error
	stop       func(context.Context, string) error
	remove     func(context.Context, string) error
	create     func(context.Context, []string) (string, error)
	find       func(context.Context, string) (*ContainerInfo, error)
	findShared func(context.Context) (*ContainerInfo, error)
	imageEnv   func(context.Context, string) (map[string]string, error)
	postCreate func(context.Context, string, string, []string)
}

// setupTestSpec writes a minimal devcontainer.json so loadSpec succeeds.
func setupTestSpec(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dcDir := filepath.Join(dir, ".devcontainer")
	if err := os.MkdirAll(dcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"),
		[]byte(`{"image":"test:latest"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEnsureInstance_CreateNew_ProjectMode(t *testing.T) {
	project := setupTestSpec(t)
	created := false
	withMockDockerStack(t, dockerStackMocks{
		find: func(_ context.Context, _ string) (*ContainerInfo, error) {
			return nil, nil // no existing container
		},
		imageEnv: func(_ context.Context, _ string) (map[string]string, error) {
			return map[string]string{}, nil
		},
		create: func(_ context.Context, _ []string) (string, error) {
			created = true
			return "new-ctr-id", nil
		},
		start:      func(_ context.Context, _ string) error { return nil },
		postCreate: func(_ context.Context, _, _ string, _ []string) {},
	})
	m := New(nil)
	inst, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{})
	if err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	if !created {
		t.Errorf("CreateContainer was not called")
	}
	if inst == nil || inst.Internal == nil {
		t.Fatalf("Instance/Internal not populated")
	}
	if inst.Internal.ContainerID() != "new-ctr-id" {
		t.Errorf("ContainerID = %q, want new-ctr-id", inst.Internal.ContainerID())
	}
}

func TestEnsureInstance_ReuseExisting_ProjectMode(t *testing.T) {
	project := setupTestSpec(t)
	startCalled := false
	withMockDockerStack(t, dockerStackMocks{
		find: func(_ context.Context, _ string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: "existing", State: "exited"}, nil
		},
		imageEnv: func(_ context.Context, _ string) (map[string]string, error) {
			return map[string]string{}, nil
		},
		start: func(_ context.Context, id string) error {
			startCalled = true
			if id != "existing" {
				t.Errorf("StartContainer called with %q, want existing", id)
			}
			return nil
		},
		create: func(_ context.Context, _ []string) (string, error) {
			t.Errorf("CreateContainer must not be called when reusing")
			return "", nil
		},
	})
	m := New(nil)
	if _, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{}); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	if !startCalled {
		t.Errorf("StartContainer was not called on existing exited container")
	}
}

// Cold start contract: if a previous daemon crashed (no graceful shutdown),
// the leftover container must be discarded so the new launch re-runs
// postCreate (sockbridge / app-server bootstrap) on a fresh container.
func TestEnsureInstance_ColdStartDiscardsExistingContainer(t *testing.T) {
	project := setupTestSpec(t)
	var removed string
	var created bool
	withMockDockerStack(t, dockerStackMocks{
		find: func(_ context.Context, _ string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: "stale", State: "running"}, nil
		},
		imageEnv: func(_ context.Context, _ string) (map[string]string, error) {
			return map[string]string{}, nil
		},
		remove: func(_ context.Context, id string) error {
			removed = id
			return nil
		},
		create: func(_ context.Context, _ []string) (string, error) {
			created = true
			return "new-ctr", nil
		},
		start:      func(_ context.Context, _ string) error { return nil },
		postCreate: func(_ context.Context, _, _ string, _ []string) {},
	})
	m := New(nil)
	if _, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{ColdStart: true}); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	if removed != "stale" {
		t.Errorf("cold start: existing container must be removed; got removed=%q", removed)
	}
	if !created {
		t.Errorf("cold start: a fresh container must be created after the discard")
	}
}

// Cold start without an existing container: no remove call, straight to create.
func TestEnsureInstance_ColdStartNoExistingGoesStraightToCreate(t *testing.T) {
	project := setupTestSpec(t)
	var removeCalled bool
	var created bool
	withMockDockerStack(t, dockerStackMocks{
		find: func(_ context.Context, _ string) (*ContainerInfo, error) { return nil, nil },
		imageEnv: func(_ context.Context, _ string) (map[string]string, error) {
			return map[string]string{}, nil
		},
		remove: func(_ context.Context, _ string) error {
			removeCalled = true
			return nil
		},
		create: func(_ context.Context, _ []string) (string, error) {
			created = true
			return "new-ctr", nil
		},
		start:      func(_ context.Context, _ string) error { return nil },
		postCreate: func(_ context.Context, _, _ string, _ []string) {},
	})
	m := New(nil)
	if _, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{ColdStart: true}); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	if removeCalled {
		t.Errorf("cold start with no existing container must not call remove")
	}
	if !created {
		t.Errorf("create must be called")
	}
}

// Warm start path: existing container must be reused (no discard).
func TestEnsureInstance_WarmStartReusesExistingContainer(t *testing.T) {
	project := setupTestSpec(t)
	var removeCalled, createCalled bool
	withMockDockerStack(t, dockerStackMocks{
		find: func(_ context.Context, _ string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: "warm-ctr", State: "running"}, nil
		},
		imageEnv: func(_ context.Context, _ string) (map[string]string, error) {
			return map[string]string{}, nil
		},
		remove: func(_ context.Context, _ string) error { removeCalled = true; return nil },
		create: func(_ context.Context, _ []string) (string, error) {
			createCalled = true
			return "", nil
		},
		start: func(_ context.Context, _ string) error { return nil },
	})
	m := New(nil)
	if _, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{}); err != nil {
		t.Fatalf("EnsureInstance: %v", err)
	}
	if removeCalled {
		t.Errorf("warm start must not destroy the existing container")
	}
	if createCalled {
		t.Errorf("warm start must not create a fresh container; existing was usable")
	}
}

func TestEnsureInstance_ImageEnvFailureIsNonFatal(t *testing.T) {
	project := setupTestSpec(t)
	withMockDockerStack(t, dockerStackMocks{
		find: func(_ context.Context, _ string) (*ContainerInfo, error) { return nil, nil },
		imageEnv: func(_ context.Context, _ string) (map[string]string, error) {
			return nil, fmt.Errorf("image not found locally")
		},
		create:     func(_ context.Context, _ []string) (string, error) { return "id", nil },
		start:      func(_ context.Context, _ string) error { return nil },
		postCreate: func(_ context.Context, _, _ string, _ []string) {},
	})
	m := New(nil)
	if _, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{}); err != nil {
		t.Errorf("imageEnv error must be non-fatal, got %v", err)
	}
}

func TestEnsureInstance_FindContainerError(t *testing.T) {
	project := setupTestSpec(t)
	withMockDockerStack(t, dockerStackMocks{
		find: func(_ context.Context, _ string) (*ContainerInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		},
	})
	m := New(nil)
	_, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{})
	if err == nil || !strings.Contains(err.Error(), "find container") {
		t.Errorf("expected find-container error, got: %v", err)
	}
}

func TestEnsureInstance_CachedSecondCall(t *testing.T) {
	project := setupTestSpec(t)
	createCalls := 0
	withMockDockerStack(t, dockerStackMocks{
		find:     func(_ context.Context, _ string) (*ContainerInfo, error) { return nil, nil },
		imageEnv: func(_ context.Context, _ string) (map[string]string, error) { return map[string]string{}, nil },
		create: func(_ context.Context, _ []string) (string, error) {
			createCalls++
			return "id", nil
		},
		start:      func(_ context.Context, _ string) error { return nil },
		postCreate: func(_ context.Context, _, _ string, _ []string) {},
	})
	m := New(nil)
	for i := 0; i < 3; i++ {
		if _, err := m.EnsureInstance(context.Background(), project, "", sandbox.StartOptions{}); err != nil {
			t.Fatalf("EnsureInstance #%d: %v", i, err)
		}
	}
	if createCalls != 1 {
		t.Errorf("CreateContainer called %d times across 3 EnsureInstance calls, want 1", createCalls)
	}
}

// Shutdown 仕様: shared container も含めてすべて破棄する。Cold start で
// 必ず新しい container が作られるよう、shutdown は資源を完全に解放する。
// detach 経路 (EffDetachClient) は DestroyInstance を呼ばないので、warm
// restart 用に container を残したい場合は detach を使う。
func TestDestroyInstance_SharedRemoved(t *testing.T) {
	stopID, rmID := "", ""
	origStop, origRm := stopContainerFn, removeContainerFn
	t.Cleanup(func() {
		stopContainerFn = origStop
		removeContainerFn = origRm
	})
	stopContainerFn = func(_ context.Context, id string) error { stopID = id; return nil }
	removeContainerFn = func(_ context.Context, id string) error { rmID = id; return nil }

	m := &Manager{containers: map[string]*ContainerState{
		SharedContainerKey: {containerID: "shared-id", spec: &DevcontainerSpec{Isolation: IsolationShared}},
	}}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: "/workspace/myapp",
		Internal:    m.containers[SharedContainerKey],
	}
	if err := m.DestroyInstance(context.Background(), inst); err != nil {
		t.Fatalf("DestroyInstance: %v", err)
	}
	if rmID != "shared-id" {
		t.Errorf("rm called with %q, want shared-id", rmID)
	}
	if stopID != "" {
		t.Errorf("stop must NOT be called on shutdown; got %q", stopID)
	}
	if _, ok := m.containers[SharedContainerKey]; ok {
		t.Errorf("containers[__shared__] still present after Destroy")
	}
}

func TestDestroyInstance_ProjectCallsRemoveNotStop(t *testing.T) {
	stopID, rmID := "", ""
	origStop, origRm := stopContainerFn, removeContainerFn
	t.Cleanup(func() {
		stopContainerFn = origStop
		removeContainerFn = origRm
	})
	stopContainerFn = func(_ context.Context, id string) error { stopID = id; return nil }
	removeContainerFn = func(_ context.Context, id string) error { rmID = id; return nil }

	const project = "/workspace/myapp"
	m := &Manager{containers: map[string]*ContainerState{
		project: {containerID: "proj-id", spec: &DevcontainerSpec{Isolation: IsolationProject}},
	}}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: project,
		Internal:    m.containers[project],
	}
	if err := m.DestroyInstance(context.Background(), inst); err != nil {
		t.Fatalf("DestroyInstance: %v", err)
	}
	if rmID != "proj-id" {
		t.Errorf("rm called with %q, want proj-id", rmID)
	}
	if stopID != "" {
		t.Errorf("stop must NOT be called for project container; got %q", stopID)
	}
	if _, ok := m.containers[project]; ok {
		t.Errorf("containers[%q] still present after Destroy", project)
	}
}

func TestDestroyInstance_EmptyContainerIDIsNoop(t *testing.T) {
	stopCalled, rmCalled := false, false
	origStop, origRm := stopContainerFn, removeContainerFn
	t.Cleanup(func() {
		stopContainerFn = origStop
		removeContainerFn = origRm
	})
	stopContainerFn = func(_ context.Context, _ string) error { stopCalled = true; return nil }
	removeContainerFn = func(_ context.Context, _ string) error { rmCalled = true; return nil }

	m := &Manager{containers: map[string]*ContainerState{
		"/p": {containerID: "", spec: &DevcontainerSpec{Isolation: IsolationProject}}, // never started
	}}
	inst := &sandbox.Instance[*ContainerState]{
		ProjectPath: "/p",
		Internal:    m.containers["/p"],
	}
	if err := m.DestroyInstance(context.Background(), inst); err != nil {
		t.Fatalf("DestroyInstance: %v", err)
	}
	if stopCalled || rmCalled {
		t.Errorf("docker should not be called when containerID is empty")
	}
}

// staleBindMountErr returns the exact error string Docker Desktop produces when
// its WSL bind-mount cache loses a file-mount source. Tests reuse this so the
// regression assertions stay bound to the real failure mode.
func staleBindMountErr(id string) error {
	return fmt.Errorf("docker start %s: exit status 1\nError response from daemon: "+
		"failed to create task for container: failed to create shim task: "+
		"OCI runtime create failed: runc create failed: "+
		"unable to start container process: error during container init: "+
		`error mounting "/run/desktop/mnt/host/wsl/docker-desktop-bind-mounts/Ubuntu-22.04/abc123" `+
		`to rootfs at "/home/ubuntu/.claude.json": `+
		`mount src=..., dst=..., flags=MS_BIND|MS_REC: no such file or directory`, id)
}

// withMockDockerFns swaps the package-level docker indirections for the duration
// of a test. The originals are restored on t.Cleanup so other tests are unaffected.
func withMockDockerFns(t *testing.T, start func(ctx context.Context, id string) error, remove func(ctx context.Context, id string) error) {
	t.Helper()
	origStart, origRm := startContainerFn, removeContainerFn
	t.Cleanup(func() {
		startContainerFn = origStart
		removeContainerFn = origRm
	})
	if start != nil {
		startContainerFn = start
	}
	if remove != nil {
		removeContainerFn = remove
	}
}

// Reproduces the user's "frame won't start after roost restart" bug:
// after a clean shutdown, docker start of the existing roost-shared fails
// with an OCI mount error because Docker Desktop's WSL bind-mount cache lost
// the source path for ~/.claude.json. tryReuseElseRecreate must catch that
// specific failure, remove the broken container, and tell ensureContainer to
// recreate. Any other reuse failure must propagate unchanged.
func TestContainerState_Getters(t *testing.T) {
	t.Run("nil safe", func(t *testing.T) {
		var cs *ContainerState
		if cs.WorkspaceFolder() != "" || cs.BindMounts() != nil || cs.ContainerID() != "" ||
			cs.PreExec() != "" || cs.EffectiveUser() != "" {
			t.Errorf("nil ContainerState getters must return zero values")
		}
	})
	t.Run("nil spec safe", func(t *testing.T) {
		cs := &ContainerState{} // spec is nil
		if cs.WorkspaceFolder() != "" || cs.BindMounts() != nil ||
			cs.PreExec() != "" || cs.EffectiveUser() != "" {
			t.Errorf("ContainerState with nil spec must return zero values")
		}
	})
	t.Run("populated spec", func(t *testing.T) {
		spec := &DevcontainerSpec{
			ProjectPath:     "/workspace/myapp",
			WorkspaceFolder: "/workspaces/myapp",
			PreExec:         "source .env",
			RemoteUser:      "ubuntu",
		}
		cs := &ContainerState{containerID: "id123", spec: spec}
		if got := cs.WorkspaceFolder(); got != "/workspaces/myapp" {
			t.Errorf("WorkspaceFolder = %q", got)
		}
		if got := cs.ContainerID(); got != "id123" {
			t.Errorf("ContainerID = %q", got)
		}
		if got := cs.PreExec(); got != "source .env" {
			t.Errorf("PreExec = %q", got)
		}
		if got := cs.EffectiveUser(); got != "ubuntu" {
			t.Errorf("EffectiveUser = %q", got)
		}
		// BindMounts goes through spec; with no extra mounts it returns the
		// workspace fallback as a single entry.
		if binds := cs.BindMounts(); len(binds) == 0 {
			t.Errorf("BindMounts should include at least the workspace mount")
		}
	})
}

func TestManager_New(t *testing.T) {
	called := 0
	overlay := func(string, string, string) (SpecOverlay, error) { called++; return SpecOverlay{}, nil }
	m := New(overlay)
	if m == nil {
		t.Fatalf("New returned nil")
	}
	if m.containers == nil {
		t.Errorf("New must initialize the containers map")
	}
	// Verify the overlay function was stored by exercising it through loadSpec
	// surrogate: just invoke the function via the manager field directly.
	if m.overlayFn == nil {
		t.Fatalf("overlayFn not stored")
	}
	if _, err := m.overlayFn("", "", ""); err != nil {
		t.Errorf("overlay invocation: %v", err)
	}
	if called != 1 {
		t.Errorf("overlay called %d times, want 1", called)
	}
}

func TestAcquireReleaseFrame_RefCount(t *testing.T) {
	m := &Manager{}
	inst := &sandbox.Instance[*ContainerState]{Internal: &ContainerState{}}

	// First two frames: ref-count grows, ReleaseFrame returns false (no destroy).
	m.AcquireFrame(inst)
	m.AcquireFrame(inst)
	if got := m.ReleaseFrame(inst); got {
		t.Errorf("ReleaseFrame on 2 acquires: got destroy=true after first release")
	}
	// Last frame: ReleaseFrame returns true so the caller knows to DestroyInstance.
	if got := m.ReleaseFrame(inst); !got {
		t.Errorf("ReleaseFrame at refCount==0: got destroy=false, want true")
	}
}

func TestReleaseFrame_NeverGoesNegative(t *testing.T) {
	// Defensive: callers should never Release without Acquire, but if they do
	// (e.g. duplicate cleanup), the call must still report destroy=true rather
	// than wedging the container forever or panicking.
	m := &Manager{}
	inst := &sandbox.Instance[*ContainerState]{Internal: &ContainerState{}}
	if got := m.ReleaseFrame(inst); !got {
		t.Errorf("ReleaseFrame from refCount=0: got %v, want true (treat as destroyable)", got)
	}
	if got := m.ReleaseFrame(inst); !got {
		t.Errorf("ReleaseFrame from refCount<0: got %v, want true (idempotent destroy)", got)
	}
}

func TestAcquireReleaseFrame_ConcurrentSafe(t *testing.T) {
	// AcquireFrame / ReleaseFrame guard the count with cs.mu. The shared
	// container in particular has frames from multiple projects acquiring /
	// releasing in parallel; if the mutex regressed we'd see a refCount that
	// drops to zero (and triggers DestroyInstance) while frames still hold it.
	m := &Manager{}
	inst := &sandbox.Instance[*ContainerState]{Internal: &ContainerState{}}

	const n = 100
	done := make(chan bool, n*2)
	for i := 0; i < n; i++ {
		go func() { m.AcquireFrame(inst); done <- true }()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	if inst.Internal.refCount != n {
		t.Fatalf("after %d concurrent acquires: refCount=%d, want %d", n, inst.Internal.refCount, n)
	}
	for i := 0; i < n; i++ {
		go func() { _ = m.ReleaseFrame(inst); done <- true }()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	if inst.Internal.refCount != 0 {
		t.Errorf("after %d concurrent releases: refCount=%d, want 0", n, inst.Internal.refCount)
	}
}

func TestTryReuseElseRecreate_StaleBindMount(t *testing.T) {
	withMockDockerFns(t,
		func(_ context.Context, id string) error {
			return staleBindMountErr(id) // reuseContainer's docker start fails this way
		},
		func(_ context.Context, _ string) error {
			return nil // remove succeeds
		},
	)
	m := &Manager{containers: map[string]*ContainerState{}}
	ctr := &ContainerInfo{ID: "abc123", State: "exited"}
	spec := &DevcontainerSpec{ProjectPath: "/workspace/myapp", Isolation: IsolationShared}

	recreate, err := m.tryReuseElseRecreate(context.Background(), SharedContainerKey, ctr, spec)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !recreate {
		t.Errorf("recreate=false; auto-recover did not trigger on stale bind-mount")
	}
	// reuseContainer may have stored an entry just before the start error
	// surfaced. After recovery, the stale entry must be gone so createContainer
	// can populate it fresh.
	if _, ok := m.containers[SharedContainerKey]; ok {
		t.Errorf("stale containers[__shared__] entry was not cleared")
	}
}

func TestTryReuseElseRecreate_PropagatesUnrelatedError(t *testing.T) {
	otherErr := fmt.Errorf("docker start abc: permission denied")
	withMockDockerFns(t,
		func(_ context.Context, _ string) error { return otherErr },
		func(_ context.Context, _ string) error {
			t.Errorf("RemoveContainer must NOT be called for non-stale failures")
			return nil
		},
	)
	m := &Manager{containers: map[string]*ContainerState{}}
	ctr := &ContainerInfo{ID: "abc", State: "exited"}
	spec := &DevcontainerSpec{ProjectPath: "/workspace/myapp"}

	recreate, err := m.tryReuseElseRecreate(context.Background(), "/workspace/myapp", ctr, spec)
	if recreate {
		t.Errorf("recreate=true; non-stale errors must not trigger recover")
	}
	if err == nil {
		t.Errorf("expected propagated error, got nil")
	}
}

func TestTryReuseElseRecreate_NoErrorWhenReuseSucceeds(t *testing.T) {
	rmCalled := false
	withMockDockerFns(t,
		func(_ context.Context, _ string) error { return nil }, // reuse OK
		func(_ context.Context, _ string) error {
			rmCalled = true
			return nil
		},
	)
	m := &Manager{containers: map[string]*ContainerState{}}
	ctr := &ContainerInfo{ID: "abc", State: "exited"}
	spec := &DevcontainerSpec{ProjectPath: "/workspace/myapp"}

	recreate, err := m.tryReuseElseRecreate(context.Background(), "/workspace/myapp", ctr, spec)
	if err != nil || recreate {
		t.Errorf("clean reuse: got recreate=%v err=%v", recreate, err)
	}
	if rmCalled {
		t.Errorf("RemoveContainer must not be called when reuse succeeds")
	}
	// reuseContainer must have populated the in-memory entry.
	if _, ok := m.containers["/workspace/myapp"]; !ok {
		t.Errorf("containers entry not populated after successful reuse")
	}
}

func TestTryReuseElseRecreate_RemoveFailurePropagates(t *testing.T) {
	rmErr := fmt.Errorf("docker rm abc: permission denied")
	withMockDockerFns(t,
		func(_ context.Context, id string) error { return staleBindMountErr(id) },
		func(_ context.Context, _ string) error { return rmErr },
	)
	m := &Manager{containers: map[string]*ContainerState{}}
	ctr := &ContainerInfo{ID: "abc", State: "exited"}
	spec := &DevcontainerSpec{}

	recreate, err := m.tryReuseElseRecreate(context.Background(), SharedContainerKey, ctr, spec)
	if recreate {
		t.Errorf("recreate=true when remove failed; caller would skip createContainer with bad state")
	}
	if err == nil || !strings.Contains(err.Error(), "recover after stale bind-mount") {
		t.Errorf("expected recover-error wrap, got: %v", err)
	}
}

func TestIsStaleBindMountError(t *testing.T) {
	staleSample := fmt.Errorf("docker start abc: exit status 1\nError response from daemon: " +
		"failed to create task for container: failed to create shim task: " +
		"OCI runtime create failed: runc create failed: " +
		"unable to start container process: error during container init: " +
		`error mounting "/run/desktop/mnt/host/wsl/docker-desktop-bind-mounts/Ubuntu-22.04/1f37ac35" ` +
		`to rootfs at "/home/ubuntu/.claude.json": ` +
		`mount src=..., dst=..., flags=MS_BIND|MS_REC: no such file or directory`)
	if !isStaleBindMountError(staleSample) {
		t.Errorf("expected stale-bind-mount detection on docker desktop OCI mount error")
	}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", fmt.Errorf("docker start abc: permission denied"), false},
		{"oci create no mounting", fmt.Errorf("OCI runtime create failed: some other failure"), false},
		{"mounting but not OCI", fmt.Errorf("error mounting /foo: no such file or directory"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isStaleBindMountError(tc.err); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMountConfigurationHash_Deterministic(t *testing.T) {
	spec := &DevcontainerSpec{ExtraWorkspaces: []BindMount{
		{Source: "/a", Target: "/a"},
		{Source: "/b", Target: "/b"},
	}}
	h1 := spec.MountConfigurationHash()
	h2 := spec.MountConfigurationHash()
	if h1 != h2 || h1 == "" {
		t.Errorf("non-deterministic or empty hash: %q vs %q", h1, h2)
	}
}

func TestMountConfigurationHash_OrderIndependent(t *testing.T) {
	a := &DevcontainerSpec{ExtraWorkspaces: []BindMount{
		{Source: "/a", Target: "/a"},
		{Source: "/b", Target: "/b"},
	}}
	b := &DevcontainerSpec{ExtraWorkspaces: []BindMount{
		{Source: "/b", Target: "/b"},
		{Source: "/a", Target: "/a"},
	}}
	if a.MountConfigurationHash() != b.MountConfigurationHash() {
		t.Errorf("order should not affect hash: %q vs %q",
			a.MountConfigurationHash(), b.MountConfigurationHash())
	}
}

func TestMountConfigurationHash_DifferentSets(t *testing.T) {
	a := &DevcontainerSpec{ExtraWorkspaces: []BindMount{{Source: "/a", Target: "/a"}}}
	b := &DevcontainerSpec{ExtraWorkspaces: []BindMount{{Source: "/b", Target: "/b"}}}
	if a.MountConfigurationHash() == b.MountConfigurationHash() {
		t.Errorf("different sets must hash differently")
	}
}

// Regression for "codex frame fails to connect to remote app server after a
// roost binary upgrade": the host-side stream backend now writes its socket
// under ~/.roost/run/__shared__/ while a pre-existing roost-shared container
// still has /opt/roost/run bind-mounted to the older random-hash directory
// (~/.roost/run/<hash>/). The Manager's mount-hash label hasn't changed —
// because it only covered ExtraWorkspaces — so the stale container is
// reused, the in-container sockbridge talks to the old host path, and codex
// CLI gets "failed to connect to remote app server".
//
// MountConfigurationHash must include every container-create-time mount
// (workspace + run-dir + proxy / devcontainer.json mounts) so changing the
// run-dir source path flips the label and triggers an auto-recreate.
func TestMountConfigurationHash_DetectsRunDirMountChange(t *testing.T) {
	specOld := &DevcontainerSpec{
		ExtraWorkspaces: []BindMount{
			{Source: "/workspace/a", Target: "/workspace/a"},
		},
		Mounts: []string{
			"type=bind,source=/home/take/.roost/run/4342aed7adbf,target=/opt/roost/run",
		},
	}
	specNew := &DevcontainerSpec{
		ExtraWorkspaces: []BindMount{
			{Source: "/workspace/a", Target: "/workspace/a"},
		},
		Mounts: []string{
			"type=bind,source=/home/take/.roost/run/__shared__,target=/opt/roost/run",
		},
	}
	if got := specOld.MountConfigurationHash(); got == specNew.MountConfigurationHash() {
		t.Errorf("run-dir mount drift must flip the hash; got identical %q", got)
	}
}

// Adding an arbitrary mount (e.g. credproxy AWS sock) must also change the
// hash so an upgrade that introduces a new bind-mount triggers recreate.
func TestMountConfigurationHash_DetectsAddedMount(t *testing.T) {
	baseMounts := []string{"type=bind,source=/host/run,target=/opt/roost/run"}
	specOld := &DevcontainerSpec{Mounts: baseMounts}
	specNew := &DevcontainerSpec{Mounts: append(
		append([]string{}, baseMounts...),
		"type=bind,source=/host/aws.sock,target=/opt/aws.sock",
	)}
	if specOld.MountConfigurationHash() == specNew.MountConfigurationHash() {
		t.Errorf("adding a mount must change the hash")
	}
}

// Mount order in spec.Mounts is incidental — provider iteration over Go maps
// is non-deterministic — so the hash must canonicalize.
func TestMountConfigurationHash_OrderIndependentForMounts(t *testing.T) {
	mountsA := []string{
		"type=bind,source=/a,target=/x",
		"type=bind,source=/b,target=/y",
	}
	mountsB := []string{
		"type=bind,source=/b,target=/y",
		"type=bind,source=/a,target=/x",
	}
	a := &DevcontainerSpec{Mounts: mountsA}
	b := &DevcontainerSpec{Mounts: mountsB}
	if a.MountConfigurationHash() != b.MountConfigurationHash() {
		t.Errorf("mount-order should not affect hash; got %q vs %q",
			a.MountConfigurationHash(), b.MountConfigurationHash())
	}
}

// Empty spec must still produce a stable, non-empty fallback so the label
// value is always parseable.
func TestMountConfigurationHash_EmptyStable(t *testing.T) {
	a := (&DevcontainerSpec{}).MountConfigurationHash()
	b := (&DevcontainerSpec{}).MountConfigurationHash()
	if a == "" {
		t.Errorf("empty spec must produce non-empty hash")
	}
	if a != b {
		t.Errorf("empty spec hash must be stable")
	}
}

func TestMountConfigurationHash_Empty(t *testing.T) {
	spec := &DevcontainerSpec{}
	if got := spec.MountConfigurationHash(); got != "none" {
		t.Errorf("MountConfigurationHash() = %q, want \"none\"", got)
	}
}

func TestBuildCreateArgs_shared_includes_mount_hash_label(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:  "/workspace/myapp",
		Isolation:    IsolationShared,
		ContainerEnv: map[string]string{},
		ExtraWorkspaces: []BindMount{
			{Source: "/workspace/myapp", Target: "/workspace/myapp"},
		},
	}
	args := spec.BuildCreateArgs("img:latest")
	want := "roost-mount-hash=" + spec.MountConfigurationHash()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args missing %q: %v", want, args)
}

func TestBuildCreateArgs_project_omits_mount_hash_label(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:  "/workspace/myapp",
		Isolation:    IsolationProject,
		ContainerEnv: map[string]string{},
	}
	args := spec.BuildCreateArgs("img:latest")
	for _, a := range args {
		if strings.HasPrefix(a, "roost-mount-hash=") {
			t.Errorf("project mode must not emit roost-mount-hash label: %v", args)
		}
	}
}

func TestBuildCreateArgs_shared_labels(t *testing.T) {
	spec := &DevcontainerSpec{
		ProjectPath:  "/workspace/myapp",
		Isolation:    IsolationShared,
		ContainerEnv: map[string]string{},
		ExtraWorkspaces: []BindMount{
			{Source: "/workspace/myapp", Target: "/workspace/myapp"},
			{Source: "/workspace/other", Target: "/workspace/other"},
		},
	}
	args := spec.BuildCreateArgs("shared-image:latest")

	mustContain := func(needle string) {
		t.Helper()
		for _, a := range args {
			if a == needle {
				return
			}
		}
		t.Errorf("args missing %q: %v", needle, args)
	}
	mustNotContain := func(needle string) {
		t.Helper()
		for _, a := range args {
			if a == needle {
				t.Errorf("args must not contain %q: %v", needle, args)
				return
			}
		}
	}

	mustContain("roost-shared")
	mustContain("roost-managed=1")
	mustContain("roost-isolation=shared")
	mustNotContain("roost-project=/workspace/myapp")

	// ExtraWorkspaces should appear as --mount args.
	found := 0
	for _, a := range args {
		if a == "type=bind,source=/workspace/myapp,target=/workspace/myapp,consistency=cached" ||
			a == "type=bind,source=/workspace/other,target=/workspace/other,consistency=cached" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected 2 ExtraWorkspace mounts, found %d; args: %v", found, args)
	}
}

func TestDevcontainerSpec_effectiveUser(t *testing.T) {
	cases := []struct {
		name      string
		remote    string
		container string
		want      string
	}{
		{"both set", "vscode", "root", "vscode"},
		{"only container", "", "ubuntu", "ubuntu"},
		{"neither", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &DevcontainerSpec{RemoteUser: tc.remote, ContainerUser: tc.container}
			if got := s.EffectiveUser(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
