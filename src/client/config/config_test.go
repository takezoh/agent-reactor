package config

import (
	"os"
	"path/filepath"
	"testing"

	platformconfig "github.com/takezoh/agent-reactor/platform/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Pane.SessionName != "arc" {
		t.Errorf("SessionName = %q, want %q", cfg.Pane.SessionName, "arc")
	}
	if cfg.Monitor.PollIntervalMs != 1000 {
		t.Errorf("PollIntervalMs = %d, want 1000", cfg.Monitor.PollIntervalMs)
	}
	if cfg.Session.DefaultCommand != "shell" {
		t.Errorf("DefaultCommand = %q, want %q", cfg.Session.DefaultCommand, "shell")
	}
	if len(cfg.Session.Commands) != 1 || cfg.Session.Commands[0] != "shell" {
		t.Errorf("Commands = %v, want [shell]", cfg.Session.Commands)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if len(cfg.Session.PushCommands) != 1 || cfg.Session.PushCommands[0] != "shell" {
		t.Errorf("PushCommands = %v, want [shell]", cfg.Session.PushCommands)
	}
	if cfg.Terminal.ScrollbackLines != 10000 {
		t.Errorf("Terminal.ScrollbackLines = %d, want 10000", cfg.Terminal.ScrollbackLines)
	}
}

// TestLoadFrom_TerminalScrollback pins the TOML wiring for the new
// `[terminal] scrollback_lines = N` knob — late-joining Web UI clients see
// up to this many scrolled-off rows on subscribe.
func TestLoadFrom_TerminalScrollback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[terminal]
scrollback_lines = 500
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Terminal.ScrollbackLines != 500 {
		t.Errorf("Terminal.ScrollbackLines = %d, want 500", cfg.Terminal.ScrollbackLines)
	}
}

func TestLoadFrom_PushCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[session]
push_commands = ["shell", "claude"]
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Session.PushCommands) != 2 {
		t.Fatalf("PushCommands = %v, want [shell, claude]", cfg.Session.PushCommands)
	}
	if cfg.Session.PushCommands[0] != "shell" || cfg.Session.PushCommands[1] != "claude" {
		t.Errorf("PushCommands = %v, want [shell, claude]", cfg.Session.PushCommands)
	}
}

func TestLoadFrom_LogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[log]
level = "debug"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := platformconfig.ExpandPath("~/foo")
	want := filepath.Join(home, "foo")
	if got != want {
		t.Errorf("ExpandPath(~/foo) = %q, want %q", got, want)
	}
	if got := platformconfig.ExpandPath("/abs/path"); got != "/abs/path" {
		t.Errorf("ExpandPath(/abs/path) = %q, want /abs/path", got)
	}
}

func TestListProjects(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "proj-a"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "proj-b"), 0o755)
	os.MkdirAll(filepath.Join(tmp, ".hidden"), 0o755)
	os.WriteFile(filepath.Join(tmp, "README"), []byte("hi"), 0o644)

	cfg := &Config{Projects: platformconfig.ProjectsConfig{ProjectRoots: []string{tmp}}}
	projects := cfg.ListProjects()
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2; got %v", len(projects), projects)
	}
	names := map[string]bool{}
	for _, p := range projects {
		names[filepath.Base(p)] = true
	}
	if !names["proj-a"] || !names["proj-b"] {
		t.Errorf("expected proj-a and proj-b, got %v", projects)
	}
}

func TestListProjects_WithProjectPaths(t *testing.T) {
	tmp := t.TempDir()
	direct := filepath.Join(tmp, "direct-proj")
	os.MkdirAll(direct, 0o755)
	nonexistent := filepath.Join(tmp, "does-not-exist")

	cfg := &Config{Projects: platformconfig.ProjectsConfig{ProjectPaths: []string{direct, nonexistent}}}
	projects := cfg.ListProjects()
	if len(projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1; got %v", len(projects), projects)
	}
	if projects[0] != direct {
		t.Errorf("projects[0] = %q, want %q", projects[0], direct)
	}
}

func TestListProjects_RootsAndPaths(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "roots", "proj-a"), 0o755)
	direct := filepath.Join(tmp, "direct-proj")
	os.MkdirAll(direct, 0o755)

	cfg := &Config{Projects: platformconfig.ProjectsConfig{
		ProjectRoots: []string{filepath.Join(tmp, "roots")},
		ProjectPaths: []string{direct},
	}}
	projects := cfg.ListProjects()
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2; got %v", len(projects), projects)
	}
}

func TestResolveDataDir_Explicit(t *testing.T) {
	t.Setenv("ROOST_DATA_DIR", "")
	cfg := &Config{DataDir: "/tmp/data"}
	if got := cfg.ResolveDataDir(); got != "/tmp/data" {
		t.Errorf("ResolveDataDir() = %q, want /tmp/data", got)
	}
}

func TestResolveDataDir_Fallback(t *testing.T) {
	t.Setenv("ROOST_DATA_DIR", "")
	cfg := &Config{}
	want := ConfigDirPath()
	if got := cfg.ResolveDataDir(); got != want {
		t.Errorf("ResolveDataDir() = %q, want %q", got, want)
	}
}

func TestResolveDataDir_EnvOverride(t *testing.T) {
	t.Setenv("ROOST_DATA_DIR", "/foo")
	cfg := &Config{DataDir: "/bar"}
	if got := cfg.ResolveDataDir(); got != "/foo" {
		t.Errorf("ResolveDataDir() = %q, want /foo", got)
	}
}

func TestResolveDataDir_EnvExpand(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ROOST_DATA_DIR", "~/x")
	cfg := &Config{}
	want := home + "/x"
	if got := cfg.ResolveDataDir(); got != want {
		t.Errorf("ResolveDataDir() = %q, want %q", got, want)
	}
}

func TestLoadFrom_Missing(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/settings.toml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pane.SessionName != "arc" {
		t.Fatalf("expected defaults, got session_name=%s", cfg.Pane.SessionName)
	}
}

func TestLoadFrom_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[tmux]
session_name = "custom"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pane.SessionName != "custom" {
		t.Fatalf("expected custom, got %s", cfg.Pane.SessionName)
	}
	if cfg.Monitor.PollIntervalMs != 1000 {
		t.Fatalf("expected default 1000, got %d", cfg.Monitor.PollIntervalMs)
	}
}

func TestLoadFrom_DriversSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[drivers.claude]
show_thinking = true
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	claude, ok := cfg.Drivers["claude"]
	if !ok {
		t.Fatal("expected drivers.claude section")
	}
	if claude["show_thinking"] != true {
		t.Errorf("show_thinking = %v, want true", claude["show_thinking"])
	}
}

func TestLoadFrom_FeaturesEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[features.enabled]
example-feature = true
another = false
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Features.Enabled["example-feature"] != true {
		t.Errorf("features.enabled[example-feature] = %v, want true", cfg.Features.Enabled["example-feature"])
	}
	if cfg.Features.Enabled["another"] != false {
		t.Errorf("features.enabled[another] = %v, want false", cfg.Features.Enabled["another"])
	}
}

func TestLoadFrom_FeaturesEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[tmux]
session_name = "test"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Features.Enabled) != 0 {
		t.Errorf("expected empty Features.Enabled, got %v", cfg.Features.Enabled)
	}
}

func TestLoadFrom_DriversEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`[tmux]
session_name = "test"
`), 0o644)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Drivers) != 0 {
		t.Errorf("expected empty Drivers, got %v", cfg.Drivers)
	}
}

func TestLoadProjectFrom_GCPConfig_enableUserAccount_returnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`
[sandbox.proxy.gcp]
account = "user@example.com"
active  = "proj-x"
enable_user_account = true
`), 0o644)

	_, err := LoadProjectFrom(path)
	if err == nil {
		t.Fatal("expected error for deprecated enable_user_account = true, got nil")
	}
}

func TestLoadProjectFrom_GCPConfig_userAccountMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`
[sandbox.proxy.gcp]
account = "user@example.com"
active  = "proj-x"
`), 0o644)

	proj, err := LoadProjectFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if proj.Sandbox == nil {
		t.Fatal("expected Sandbox to be non-nil")
	}
	gcp := proj.Sandbox.Proxy.GCP
	if gcp.Account != "user@example.com" {
		t.Errorf("Account = %q, want %q", gcp.Account, "user@example.com")
	}
	if gcp.Active != "proj-x" {
		t.Errorf("Active = %q, want %q", gcp.Active, "proj-x")
	}
	if gcp.ServiceAccount != "" {
		t.Errorf("ServiceAccount should be empty, got %q", gcp.ServiceAccount)
	}
}

func TestLoadProjectFrom_GCPConfig_SAMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`
[sandbox.proxy.gcp]
account         = "user@example.com"
active          = "proj-a"
service_account = "sa@proj.iam.gserviceaccount.com"
projects        = ["proj-a", "proj-b"]
`), 0o644)

	proj, err := LoadProjectFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	gcp := proj.Sandbox.Proxy.GCP
	if gcp.ServiceAccount != "sa@proj.iam.gserviceaccount.com" {
		t.Errorf("ServiceAccount = %q", gcp.ServiceAccount)
	}
	if gcp.Active != "proj-a" {
		t.Errorf("Active = %q, want %q", gcp.Active, "proj-a")
	}
	if len(gcp.Projects) != 2 {
		t.Errorf("Projects = %v, want 2 entries", gcp.Projects)
	}
}

func TestLoadProjectFrom_MCPProxy_server(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")
	os.WriteFile(path, []byte(`
[sandbox.proxy.mcp_proxy.servers.observability]
command = "npx"
args    = ["-y", "@example/obs-mcp"]
allow   = ["list_*"]
deny    = ["delete_*"]

[sandbox.proxy.mcp_proxy.servers.observability.env]
GOOGLE_APPLICATION_CREDENTIALS = "~/.config/gcloud/application_default_credentials.json"
`), 0o644)

	proj, err := LoadProjectFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if proj.Sandbox == nil {
		t.Fatal("expected Sandbox to be non-nil")
	}
	servers := proj.Sandbox.Proxy.MCPProxy.Servers
	if len(servers) != 1 {
		t.Fatalf("expected 1 MCP server, got %d", len(servers))
	}
	s, ok := servers["observability"]
	if !ok {
		t.Fatal("expected key 'observability' in servers map")
	}
	if s.Command != "npx" {
		t.Errorf("Command = %q, want npx", s.Command)
	}
	if len(s.Args) != 2 || s.Args[0] != "-y" {
		t.Errorf("Args = %v, unexpected", s.Args)
	}
	if len(s.Allow) != 1 || s.Allow[0] != "list_*" {
		t.Errorf("Allow = %v, unexpected", s.Allow)
	}
	if len(s.Deny) != 1 || s.Deny[0] != "delete_*" {
		t.Errorf("Deny = %v, unexpected", s.Deny)
	}
	if cred, ok := s.Env["GOOGLE_APPLICATION_CREDENTIALS"]; !ok || cred == "" {
		t.Errorf("Env GOOGLE_APPLICATION_CREDENTIALS missing or empty")
	}
}

func TestSandboxConfig_Validate_Isolation(t *testing.T) {
	cases := []struct {
		isolation string
		wantErr   bool
	}{
		{"", false},
		{"project", false},
		{"shared", false},
		{"cluster", true},
	}
	for _, tc := range cases {
		s := platformconfig.SandboxConfig{Isolation: tc.isolation}
		err := s.Validate()
		if tc.wantErr && err == nil {
			t.Errorf("isolation=%q: expected error, got nil", tc.isolation)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("isolation=%q: unexpected error: %v", tc.isolation, err)
		}
	}
}

// TestResolveDevcontainerPrefix pins the resolution order env →
// sandbox.devcontainer.name_prefix → "" so scripts/run-dev.sh can isolate its
// docker namespace from the user's TUI daemon purely via env, while a config
// override is honored when env is unset.
func TestResolveDevcontainerPrefix(t *testing.T) {
	const envVar = "ROOST_DEVCONTAINER_PREFIX"

	t.Run("env wins over config", func(t *testing.T) {
		t.Setenv(envVar, "reactor-dev")
		cfg := DefaultConfig()
		cfg.Sandbox.Devcontainer.NamePrefix = "configured"
		if got := cfg.ResolveDevcontainerPrefix(); got != "reactor-dev" {
			t.Errorf("ResolveDevcontainerPrefix = %q, want reactor-dev (env)", got)
		}
	})

	t.Run("config used when env empty", func(t *testing.T) {
		t.Setenv(envVar, "")
		cfg := DefaultConfig()
		cfg.Sandbox.Devcontainer.NamePrefix = "configured"
		if got := cfg.ResolveDevcontainerPrefix(); got != "configured" {
			t.Errorf("ResolveDevcontainerPrefix = %q, want configured", got)
		}
	})

	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv(envVar, "")
		cfg := DefaultConfig()
		if got := cfg.ResolveDevcontainerPrefix(); got != "" {
			t.Errorf("ResolveDevcontainerPrefix = %q, want \"\" (devcontainer pkg applies its own default)", got)
		}
	})
}
