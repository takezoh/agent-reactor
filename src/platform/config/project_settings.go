package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ProjectConfig is the platform-side subset of project settings: only the
// sandbox override. Used by SandboxResolver.
type ProjectConfig struct {
	Sandbox *SandboxConfig `toml:"sandbox"`
}

// LoadProjectFrom reads path as a project-level settings.toml and returns the
// sandbox-relevant subset. A missing file is not an error.
func LoadProjectFrom(path string) (*ProjectConfig, error) {
	cfg := &ProjectConfig{}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if cfg.Sandbox != nil {
		if err := cfg.Sandbox.Proxy.GCP.Validate(); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// findProjectSettings walks up from dir searching for .roost/settings.toml.
func findProjectSettings(dir string) string {
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, ".roost", "settings.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
