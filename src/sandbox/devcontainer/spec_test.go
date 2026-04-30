package devcontainer

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveContainerEnvPlaceholders_ThreeLayers(t *testing.T) {
	imageEnv := map[string]string{
		"PATH": "/usr/bin:/bin",
		"HOME": "/root",
	}
	spec := &DevcontainerSpec{
		ContainerEnv: map[string]string{
			// $VAR form
			"PATH": "/opt/shims:$PATH",
			// ${VAR} form
			"MYPATH": "${PATH}:/extra",
			// ${containerEnv:VAR} form (legacy)
			"MYPATH2": "${containerEnv:PATH}:/legacy",
			// undefined var → empty
			"UNDEF": "$UNDEFINED_VAR_XYZ",
		},
		RemoteEnv: map[string]string{
			// L3 should see resolved ContainerEnv (not image PATH)
			"REMOTE_PATH": "$PATH",
		},
	}
	spec.ResolveContainerEnvPlaceholders(imageEnv)

	cases := []struct {
		key  string
		env  map[string]string
		want string
	}{
		{"PATH", spec.ContainerEnv, "/opt/shims:/usr/bin:/bin"},
		{"MYPATH", spec.ContainerEnv, "/usr/bin:/bin:/extra"},
		{"MYPATH2", spec.ContainerEnv, "/usr/bin:/bin:/legacy"},
		{"UNDEF", spec.ContainerEnv, ""},
		// RemoteEnv PATH resolves against L1∪resolved-L2 (containerEnv PATH wins)
		{"REMOTE_PATH", spec.RemoteEnv, "/opt/shims:/usr/bin:/bin"},
	}
	for _, c := range cases {
		if got := c.env[c.key]; got != c.want {
			t.Errorf("%s = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestLoadSpec_ImageField(t *testing.T) {
	dir := setupProjectDC(t, `{"image":"myproject:dev"}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "myproject:dev" {
		t.Errorf("Image = %q, want myproject:dev", spec.Image)
	}
}

func TestLoadSpec_BuildName(t *testing.T) {
	dir := setupProjectDC(t, `{"build":{"dockerfile":"Dockerfile","name":"myproject:dev"}}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "myproject:dev" {
		t.Errorf("Image = %q, want myproject:dev", spec.Image)
	}
}

func TestLoadSpec_ImagePrecedenceOverBuildName(t *testing.T) {
	dir := setupProjectDC(t, `{"image":"top:v1","build":{"name":"build:v2"}}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "top:v1" {
		t.Errorf("Image = %q, want top:v1 (image: takes precedence)", spec.Image)
	}
}

func TestLoadSpec_MissingImage_Error(t *testing.T) {
	dir := setupProjectDC(t, `{"containerEnv":{"FOO":"bar"}}`)
	_, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if !errors.Is(err, ErrMissingImage) {
		t.Errorf("expected ErrMissingImage, got %v", err)
	}
}

func TestLoadSpec_BuildWithoutName_Error(t *testing.T) {
	dir := setupProjectDC(t, `{"build":{"dockerfile":"Dockerfile"}}`)
	_, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if !errors.Is(err, ErrMissingImage) {
		t.Errorf("expected ErrMissingImage, got %v", err)
	}
}

func TestWorkspaceTarget_FallbackWhenWorkspaceFolderUnset(t *testing.T) {
	s := &DevcontainerSpec{ProjectPath: "/host/myapp"}
	if got, want := s.WorkspaceTarget(), "/host/myapp"; got != want {
		t.Errorf("WorkspaceTarget() = %q, want %q", got, want)
	}
}

func TestWorkspaceTarget_UsesOverlayFallback(t *testing.T) {
	s := &DevcontainerSpec{ProjectPath: "/host/myapp", WorkspaceFolderFallback: "/mnt/host/myapp"}
	if got, want := s.WorkspaceTarget(), "/mnt/host/myapp"; got != want {
		t.Errorf("WorkspaceTarget() = %q, want %q", got, want)
	}
}

func TestWorkspaceTarget_WorkspaceFolderWinsOverFallback(t *testing.T) {
	s := &DevcontainerSpec{
		ProjectPath:             "/host/myapp",
		WorkspaceFolder:         "/custom/ws",
		WorkspaceFolderFallback: "/mnt/host/myapp",
	}
	if got, want := s.WorkspaceTarget(), "/custom/ws"; got != want {
		t.Errorf("WorkspaceTarget() = %q, want %q (workspaceFolder must win)", got, want)
	}
}

func TestWorkspaceTarget_UsesWorkspaceFolderWhenSet(t *testing.T) {
	s := &DevcontainerSpec{ProjectPath: "/host/myapp", WorkspaceFolder: "/custom/ws"}
	if got, want := s.WorkspaceTarget(), "/custom/ws"; got != want {
		t.Errorf("WorkspaceTarget() = %q, want %q", got, want)
	}
}

func TestParseMountSpec_BindLong(t *testing.T) {
	src, tgt, ok := parseMountSpec("type=bind,source=/host/x,target=/container/x,readonly")
	if !ok || src != "/host/x" || tgt != "/container/x" {
		t.Errorf("parseMountSpec = (%q,%q,%v), want (/host/x,/container/x,true)", src, tgt, ok)
	}
}

func TestParseMountSpec_BindAliases(t *testing.T) {
	src, tgt, ok := parseMountSpec("type=bind,src=/host/x,dst=/container/x")
	if !ok || src != "/host/x" || tgt != "/container/x" {
		t.Errorf("parseMountSpec aliases = (%q,%q,%v)", src, tgt, ok)
	}
}

func TestParseMountSpec_VolumeSkipped(t *testing.T) {
	_, _, ok := parseMountSpec("type=volume,source=myvol,target=/data")
	if ok {
		t.Errorf("non-bind types must be skipped")
	}
}

func TestParseMountSpec_ShortForm(t *testing.T) {
	src, tgt, ok := parseMountSpec("/host/x:/container/x:ro")
	if !ok || src != "/host/x" || tgt != "/container/x" {
		t.Errorf("short form = (%q,%q,%v), want (/host/x,/container/x,true)", src, tgt, ok)
	}
}

func TestBindMounts_IncludesWorkspaceAndExtraCreateArgs(t *testing.T) {
	s := &DevcontainerSpec{
		ProjectPath: "/host/myapp",
		ExtraCreateArgs: []string{
			"--mount", "type=bind,source=/home/take/.claude/projects,target=/home/ubuntu/.claude/projects",
			"-v", "/home/take/.claude/sessions:/home/ubuntu/.claude/sessions:ro",
			"--shm-size=2g", // non-mount arg should be ignored
		},
	}
	got := s.BindMounts()

	wantPairs := map[string]string{
		"/host/myapp":                 "/host/myapp", // workspace mount fallback: host-mirrored
		"/home/take/.claude/projects": "/home/ubuntu/.claude/projects",
		"/home/take/.claude/sessions": "/home/ubuntu/.claude/sessions",
	}
	for src, tgt := range wantPairs {
		found := false
		for _, b := range got {
			if b.Source == src && b.Target == tgt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected BindMount{%q -> %q} not in %+v", src, tgt, got)
		}
	}
}

func TestBindMounts_HandlesDevcontainerJSONMounts(t *testing.T) {
	s := &DevcontainerSpec{
		ProjectPath: "/host/myapp",
		Mounts:      []string{"type=bind,source=/host/cache,target=/cache"},
	}
	got := s.BindMounts()
	found := false
	for _, b := range got {
		if b.Source == "/host/cache" && b.Target == "/cache" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("devcontainer.json mounts not picked up: %+v", got)
	}
}

func TestBindMounts_SkipsVolumes(t *testing.T) {
	s := &DevcontainerSpec{
		ProjectPath: "/host/myapp",
		ExtraCreateArgs: []string{
			"--mount", "type=volume,source=myvol,target=/data",
		},
	}
	got := s.BindMounts()
	for _, b := range got {
		if b.Target == "/data" {
			t.Errorf("volume mount leaked: %+v", got)
		}
	}
}
