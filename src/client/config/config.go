package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	platformconfig "github.com/takezoh/agent-roost/platform/config"
)

type Config struct {
	DataDir       string                        `toml:"data_dir"`
	Theme         string                        `toml:"theme"`
	Log           LogConfig                     `toml:"log"`
	Tmux          TmuxConfig                    `toml:"tmux"`
	Monitor       MonitorConfig                 `toml:"monitor"`
	Session       SessionConfig                 `toml:"session"`
	Projects      platformconfig.ProjectsConfig `toml:"projects"`
	Driver        CommonDriverConfig            `toml:"driver"`
	Drivers       map[string]map[string]any     `toml:"drivers"`
	Features      FeaturesConfig                `toml:"features"`
	Notifications NotificationsConfig           `toml:"notifications"`
	Sandbox       platformconfig.SandboxConfig  `toml:"sandbox"`
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

// ResolveAlias expands a command string through the alias map.
func (s SessionConfig) ResolveAlias(command string) string {
	command = strings.TrimSpace(command)
	if expanded, ok := s.Aliases[command]; ok {
		return expanded
	}
	return command
}

func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := cfg.Notifications.Validate(); err != nil {
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
		Projects: platformconfig.ProjectsConfig{},
		Sandbox: platformconfig.SandboxConfig{
			Mode: "direct",
		},
	}
}

func ConfigDirPath() string {
	return filepath.Join(platformconfig.ExpandPath("~"), ".roost")
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
