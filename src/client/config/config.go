package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/takezoh/agent-reactor/platform/appid"
	platformconfig "github.com/takezoh/agent-reactor/platform/config"
)

type Config struct {
	DataDir  string                        `toml:"data_dir"`
	Theme    string                        `toml:"theme"`
	Log      LogConfig                     `toml:"log"`
	Monitor  MonitorConfig                 `toml:"monitor"`
	Session  SessionConfig                 `toml:"session"`
	Terminal TerminalConfig                `toml:"terminal"`
	Projects platformconfig.ProjectsConfig `toml:"projects"`
	Driver   CommonDriverConfig            `toml:"driver"`
	Drivers  map[string]map[string]any     `toml:"drivers"`
	Features FeaturesConfig                `toml:"features"`
	Sandbox  platformconfig.SandboxConfig  `toml:"sandbox"`
	Codex    CodexConfig                   `toml:"codex"`
	Editor   EditorConfig                  `toml:"editor"`
}

// EditorConfig holds settings for opening projects in an editor.
type EditorConfig struct {
	// Command is the editor executable to launch (e.g. "code", "code-insiders",
	// "cursor"). It may include flags: "code --reuse-window". Defaults to "code".
	Command string `toml:"command"`
	// Extensions is the list of file extensions that, if found in the project
	// root, cause the editor to open that file instead of the directory.
	Extensions []string `toml:"extensions"`
}

// CodexConfig holds settings for the Codex app-server integration.
type CodexConfig struct {
	// ReadTimeoutMs is the per-request JSON-RPC read timeout in milliseconds.
	// Zero means use the default (15 s).
	ReadTimeoutMs int `toml:"read_timeout_ms"`
}

// CommonDriverConfig holds settings that apply to all drivers.
type CommonDriverConfig struct {
	SummarizeCommand string `toml:"summarize_command"`
	Pager            string `toml:"pager"`
}

// FeaturesConfig holds the runtime feature-flag table from the TOML config.
type FeaturesConfig struct {
	Enabled map[string]bool `toml:"enabled"`
}

// LogConfig controls slog handler verbosity.
type LogConfig struct {
	Level string `toml:"level"`
}

type MonitorConfig struct {
	PollIntervalMs     int `toml:"poll_interval_ms"`
	FastPollIntervalMs int `toml:"fast_poll_interval_ms"`
	IdleThresholdSec   int `toml:"idle_threshold_sec"`
}

type SessionConfig struct {
	AutoName       bool     `toml:"auto_name"`
	DefaultCommand string   `toml:"default_command"`
	Commands       []string `toml:"commands"`
	PushCommands   []string `toml:"push_commands"`
}

// TerminalConfig holds knobs that govern the server-side terminal emulator.
type TerminalConfig struct {
	// ScrollbackLines bounds the VT scrollback buffer per session, in lines.
	// A late-joining Web UI client receives this buffer as the first seed
	// frame on subscribe so it can scroll up through history printed before
	// it attached. Zero leaves the underlying emulator's default in place.
	ScrollbackLines int `toml:"scrollback_lines"`
	// FontFamily overrides the Web UI terminal font (a CSS font-family value,
	// e.g. "HackGen Console NF"). Surfaced to the browser over
	// GET /api/session-config and applied to the xterm.js grid. Empty leaves
	// the xterm.js built-in monospace default in place.
	FontFamily string `toml:"font_family"`
	// FontSize sets the Web UI terminal font size in CSS px. Surfaced over
	// GET /api/session-config. Zero leaves the xterm.js built-in default
	// (15px) in place.
	FontSize int `toml:"font_size"`
}

func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := cfg.Sandbox.Validate(); err != nil {
		return nil, err
	}
	cfg.Driver.Pager = resolvePager(cfg.Driver.Pager)
	return cfg, nil
}

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
		Terminal: TerminalConfig{
			ScrollbackLines: 10000,
		},
		Editor: EditorConfig{
			Command:    "code",                      // "code" is the VS Code CLI binary
			Extensions: []string{".code-workspace"}, // VS Code workspace file extension
		},
		Projects: platformconfig.ProjectsConfig{},
		Sandbox: platformconfig.SandboxConfig{
			Mode: "direct",
		},
	}
}

func ConfigDirPath() string {
	return filepath.Join(platformconfig.ExpandPath("~"), appid.DotDir)
}

func EnsureConfigDir() string {
	dir := ConfigDirPath()
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func (c *Config) ListProjects() []string {
	return c.Projects.ListProjects()
}

func (c *Config) ResolveDataDir() string {
	if v := os.Getenv("ROOST_DATA_DIR"); v != "" {
		return platformconfig.ExpandPath(v)
	}
	if c.DataDir != "" {
		return platformconfig.ExpandPath(c.DataDir)
	}
	return ConfigDirPath()
}

// ResolveDevcontainerPrefix returns the docker container/label prefix used by
// this daemon. Resolution order: env ROOST_DEVCONTAINER_PREFIX → config
// sandbox.devcontainer.name_prefix → "" (the devcontainer package falls back
// to its DefaultNamePrefix). Use this when constructing the devcontainer
// Manager so a peer daemon (e.g. scripts/run-dev.sh) can isolate its docker
// namespace from a primary daemon without editing the TOML.
func (c *Config) ResolveDevcontainerPrefix() string {
	if v := os.Getenv("ROOST_DEVCONTAINER_PREFIX"); v != "" {
		return v
	}
	return c.Sandbox.Devcontainer.NamePrefix
}
