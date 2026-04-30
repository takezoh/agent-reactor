package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DataDir       string                    `toml:"data_dir"`
	Theme         string                    `toml:"theme"`
	Log           LogConfig                 `toml:"log"`
	Tmux          TmuxConfig                `toml:"tmux"`
	Monitor       MonitorConfig             `toml:"monitor"`
	Session       SessionConfig             `toml:"session"`
	Projects      ProjectsConfig            `toml:"projects"`
	Driver        CommonDriverConfig        `toml:"driver"`
	Drivers       map[string]map[string]any `toml:"drivers"`
	Features      FeaturesConfig            `toml:"features"`
	Notifications NotificationsConfig       `toml:"notifications"`
	Sandbox       SandboxConfig             `toml:"sandbox"`
}

// SandboxConfig controls how agent processes are isolated.
// mode = "direct" runs agents with no extra sandboxing (default).
// mode = "devcontainer" runs each project via @devcontainers/cli.
type SandboxConfig struct {
	Mode         string             `toml:"mode"`
	Devcontainer DevcontainerConfig `toml:"devcontainer"`
	Proxy        ProxyConfig        `toml:"proxy"`
}

// IsSandboxed reports whether the sandbox mode is an active isolation backend.
// Both "" and "direct" mean no sandboxing.
func (s SandboxConfig) IsSandboxed() bool {
	return s.Mode != "" && s.Mode != "direct"
}

// Validate rejects unknown sandbox modes at startup.
func (s SandboxConfig) Validate() error {
	switch s.Mode {
	case "", "direct", "devcontainer":
		return nil
	default:
		return fmt.Errorf("sandbox.mode=%q is unknown; valid values: direct, devcontainer", s.Mode)
	}
}

// DevcontainerConfig holds settings for the devcontainer sandbox mode.
type DevcontainerConfig struct {
	// ExtraCreateArgs are appended verbatim to "docker create".
	ExtraCreateArgs []string `toml:"extra_create_args"`

	// EnvScript is a path to a script that prints KEY=VALUE lines (dotenv format)
	// to stdout. It receives the project path as its first argument.
	EnvScript string `toml:"env_script"`

	// AllowProjectEnvScript lists project paths whose project-scope env_script
	// is permitted to run.
	AllowProjectEnvScript []string `toml:"allow_project_env_script"`

	// HostPathMountPrefix, when non-empty, makes the auto-mounted project workspace
	// appear at "<prefix><host-path>" inside the container instead of the default
	// "<host-path>" (host-mirroring). Has no effect if devcontainer.json explicitly
	// sets workspaceFolder or workspaceMount. Must be an absolute path or empty.
	HostPathMountPrefix string `toml:"host_path_mount_prefix"`
}

// ProxyConfig enables roost's in-process credential injection proxy.
// When Enabled, credential env vars are injected into each container.
// The proxy runs in-process; no external daemon is required.
type ProxyConfig struct {
	Enabled     bool           `toml:"enabled"`
	AWSProfiles []string       `toml:"aws_profiles"` // AWS profile names to expose in the container via credential_process
	GCP         GCPConfig      `toml:"gcp"`
	SSHAgent    SSHAgentConfig `toml:"ssh_agent"`
	WinExec     WinExecConfig  `toml:"win_exec"`
}

// WinExecConfig controls the WSL2 Windows exe broker.
// A non-empty AllowedExes activates the host-side broker, which listens on a
// per-project Unix socket and forwards exec requests from the container to
// allowlisted Windows binaries via the WSL2 /init interop layer. Ignored on
// non-WSL2 hosts.
type WinExecConfig struct {
	AllowedExes []string          `toml:"allowed_exes"` // exe basenames that may be executed (e.g. "code.exe"); empty = disabled
	Resolve     map[string]string `toml:"resolve"`      // exe name → absolute Windows path; unlisted names use Windows PATH
}

// SSHAgentConfig controls SSH agent injection into containers.
// An ephemeral ssh-agent is spawned with only the listed keys loaded.
type SSHAgentConfig struct {
	Keys []string `toml:"keys"`
}

// GCPConfig holds per-project gcloud CLI credential settings.
// ServiceAccount and Projects must both be set to enable GCP credential injection.
// roost impersonates ServiceAccount on the host to obtain a scope-limited access token;
// the container never receives the OAuth refresh token or a full-scope user token.
type GCPConfig struct {
	ServiceAccount string   `toml:"service_account"` // SA email to impersonate (required)
	Account        string   `toml:"account"`         // host gcloud principal (optional; defaults to current gcloud auth)
	Projects       []string `toml:"projects"`        // GCP project IDs available in container; first entry is the active default
}

// CommonDriverConfig holds settings that apply to all drivers.
type CommonDriverConfig struct {
	SummarizeCommand string `toml:"summarize_command"`
	Pager            string `toml:"pager"`
}

// FeaturesConfig holds the runtime feature-flag table from the TOML config.
// Each key in Enabled is a [features.Flag] identifier; true enables the flag.
type FeaturesConfig struct {
	Enabled map[string]bool `toml:"enabled"`
}

// LogConfig controls slog handler verbosity. Level values: "debug", "info",
// "warn", "error". Unknown / empty values fall back to info in logger.Init.
type LogConfig struct {
	Level string `toml:"level"`
}

type TmuxConfig struct {
	SessionName         string `toml:"session_name"`
	Prefix              string `toml:"prefix"`
	PaneRatioHorizontal int    `toml:"pane_ratio_horizontal"`
	PaneRatioVertical   int    `toml:"pane_ratio_vertical"`
}

type MonitorConfig struct {
	PollIntervalMs     int `toml:"poll_interval_ms"`
	FastPollIntervalMs int `toml:"fast_poll_interval_ms"`
	IdleThresholdSec   int `toml:"idle_threshold_sec"`
}

type SessionConfig struct {
	AutoName       bool              `toml:"auto_name"`
	DefaultCommand string            `toml:"default_command"`
	Commands       []string          `toml:"commands"`
	PushCommands   []string          `toml:"push_commands"`
	Aliases        map[string]string `toml:"aliases"`
}

// ResolveAlias expands a command string through the alias map. Unknown
// commands are returned unchanged. Aliases are matched against the entire
// trimmed input string, not parsed tokens, so "clw" maps but "clw foo" does
// not (matching shell alias semantics where the alias name is the first word).
func (s SessionConfig) ResolveAlias(command string) string {
	command = strings.TrimSpace(command)
	if expanded, ok := s.Aliases[command]; ok {
		return expanded
	}
	return command
}

type ProjectsConfig struct {
	ProjectRoots []string `toml:"project_roots"`
	ProjectPaths []string `toml:"project_paths"`
}

func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Notifications.Validate(); err != nil {
		return nil, err
	}
	cfg.Driver.Pager = resolvePager(cfg.Driver.Pager)
	return cfg, nil
}

// resolvePager returns the effective pager command. Priority:
//  1. value from config (if non-empty)
//  2. $PAGER environment variable
//  3. "less" as the universal fallback
func resolvePager(configured string) string {
	if configured != "" {
		return configured
	}
	if p := os.Getenv("PAGER"); p != "" {
		return p
	}
	return "less"
}

func Load() (*Config, error) {
	return LoadFrom(filepath.Join(ConfigDirPath(), "settings.toml"))
}

func DefaultConfig() *Config {
	return &Config{
		Theme: "default",
		Log:   LogConfig{Level: "info"},
		Tmux: TmuxConfig{
			SessionName:         "roost",
			Prefix:              "C-b",
			PaneRatioHorizontal: 75,
			PaneRatioVertical:   75,
		},
		Monitor: MonitorConfig{
			PollIntervalMs:     1000,
			FastPollIntervalMs: 100,
			IdleThresholdSec:   30,
		},
		Session: SessionConfig{
			AutoName:       true,
			DefaultCommand: "shell",
			Commands:       []string{"shell"},
			PushCommands:   []string{"shell"},
		},
		Projects: ProjectsConfig{},
		Sandbox: SandboxConfig{
			Mode: "direct",
		},
	}
}

func ConfigDirPath() string {
	return filepath.Join(ExpandPath("~"), ".roost")
}

func EnsureConfigDir() string {
	dir := ConfigDirPath()
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func (c *Config) ListProjects() []string {
	var projects []string
	for _, root := range c.Projects.ProjectRoots {
		root = ExpandPath(root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				projects = append(projects, filepath.Join(root, e.Name()))
			}
		}
	}
	for _, p := range c.Projects.ProjectPaths {
		p = ExpandPath(p)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			projects = append(projects, p)
		}
	}
	return projects
}

func (c *Config) ResolveDataDir() string {
	if v := os.Getenv("ROOST_DATA_DIR"); v != "" {
		return ExpandPath(v)
	}
	if c.DataDir != "" {
		return ExpandPath(c.DataDir)
	}
	return ConfigDirPath()
}

func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}
