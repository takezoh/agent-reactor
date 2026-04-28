package devcontainer

import (
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

	got := translateWorkDir(project+"/backend", project, spec.workspaceTarget())
	want := "/workspaces/myapp/backend"
	if got != want {
		t.Errorf("workDir = %q, want %q", got, want)
	}
	_ = cs
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
	mustContain("/workspaces/myapp")
	// workspace mount should be present as --mount arg value
	found := false
	for _, a := range args {
		if a == "type=bind,source=/workspace/myapp,target=/workspaces/myapp,consistency=cached" {
			found = true
		}
	}
	if !found {
		t.Errorf("workspace mount not found in args: %v", args)
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

	got, _, err := m.BuildLaunchCommand(inst, plan, nil)
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
