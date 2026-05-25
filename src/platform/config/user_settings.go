package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// UserSettings holds the subset of ~/.roost/settings.toml shared between the
// roost client and the orchestrator: sandbox posture, project scoping, and the
// data directory. The roost client decodes a richer superset in client/config;
// both read the same file into the same platform types, so the shared sections
// stay consistent. This keeps the orchestrator from importing client/.
type UserSettings struct {
	DataDir  string         `toml:"data_dir"`
	Sandbox  SandboxConfig  `toml:"sandbox"`
	Projects ProjectsConfig `toml:"projects"`
}

// ConfigDir returns the ~/.roost configuration directory.
func ConfigDir() string {
	return filepath.Join(ExpandPath("~"), ".roost")
}

// LoadUserSettings reads the shared subset of ~/.roost/settings.toml. A missing
// file yields defaults (direct sandbox mode); a parse or validation error is returned.
func LoadUserSettings() (UserSettings, error) {
	s := UserSettings{Sandbox: SandboxConfig{Mode: "direct"}}
	path := filepath.Join(ConfigDir(), "settings.toml")
	if _, err := toml.DecodeFile(path, &s); err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return UserSettings{}, err
	}
	if err := s.Sandbox.Validate(); err != nil {
		return UserSettings{}, err
	}
	return s, nil
}

// ResolveDataDir resolves the data directory: ROOST_DATA_DIR env, else the
// configured data_dir, else ~/.roost.
func (s UserSettings) ResolveDataDir() string {
	if v := os.Getenv("ROOST_DATA_DIR"); v != "" {
		return ExpandPath(v)
	}
	if s.DataDir != "" {
		return ExpandPath(s.DataDir)
	}
	return ConfigDir()
}
