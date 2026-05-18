package devcontainer

import (
	"slices"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/sandbox"
	"github.com/takezoh/agent-roost/state"
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
	plan := state.LaunchPlan{
		Project:   project,
		StartDir:  project,
		Command:   "codex resume thr_123 --remote unix:///opt/roost/run/codex-foo.sock",
		Subsystem: state.LaunchSubsystemStream,
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
	plan := state.LaunchPlan{Project: project, StartDir: project, Command: "shell"}

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
	plan := state.LaunchPlan{Project: project, StartDir: project, Command: "bash"}
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
	planA := state.LaunchPlan{Project: "/workspace/agent-roost", Command: "bash"}
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
	planB := state.LaunchPlan{Project: "/workspace/credproxy", Command: "bash"}
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
	plan := state.LaunchPlan{Project: project, StartDir: project, Command: "bash"}
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
	plan := state.LaunchPlan{Project: project, StartDir: project, Command: "bash"}

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
		got, _, err := m.BuildLaunchCommand(inst, state.LaunchPlan{Project: project, StartDir: project, Command: "bash"}, sandbox.FrameContext{}, nil)
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
		got, _, err := m.BuildLaunchCommand(inst, state.LaunchPlan{Project: project, StartDir: project, Command: "bash"}, sandbox.FrameContext{}, nil)
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
		got, _, err := m.BuildLaunchCommand(inst, state.LaunchPlan{Project: project, StartDir: project, Command: "shell"}, sandbox.FrameContext{}, nil)
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
	plan := state.LaunchPlan{Project: project, StartDir: project, Command: "claude"}

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

func TestExtraWorkspacesHash_Deterministic(t *testing.T) {
	spec := &DevcontainerSpec{ExtraWorkspaces: []BindMount{
		{Source: "/a", Target: "/a"},
		{Source: "/b", Target: "/b"},
	}}
	h1 := spec.ExtraWorkspacesHash()
	h2 := spec.ExtraWorkspacesHash()
	if h1 != h2 || h1 == "" {
		t.Errorf("non-deterministic or empty hash: %q vs %q", h1, h2)
	}
}

func TestExtraWorkspacesHash_OrderIndependent(t *testing.T) {
	a := &DevcontainerSpec{ExtraWorkspaces: []BindMount{
		{Source: "/a", Target: "/a"},
		{Source: "/b", Target: "/b"},
	}}
	b := &DevcontainerSpec{ExtraWorkspaces: []BindMount{
		{Source: "/b", Target: "/b"},
		{Source: "/a", Target: "/a"},
	}}
	if a.ExtraWorkspacesHash() != b.ExtraWorkspacesHash() {
		t.Errorf("order should not affect hash: %q vs %q",
			a.ExtraWorkspacesHash(), b.ExtraWorkspacesHash())
	}
}

func TestExtraWorkspacesHash_DifferentSets(t *testing.T) {
	a := &DevcontainerSpec{ExtraWorkspaces: []BindMount{{Source: "/a", Target: "/a"}}}
	b := &DevcontainerSpec{ExtraWorkspaces: []BindMount{{Source: "/b", Target: "/b"}}}
	if a.ExtraWorkspacesHash() == b.ExtraWorkspacesHash() {
		t.Errorf("different sets must hash differently")
	}
}

func TestExtraWorkspacesHash_Empty(t *testing.T) {
	spec := &DevcontainerSpec{}
	if got := spec.ExtraWorkspacesHash(); got != "none" {
		t.Errorf("ExtraWorkspacesHash() = %q, want \"none\"", got)
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
	want := "roost-mount-hash=" + spec.ExtraWorkspacesHash()
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
