package config

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

type sandboxCacheEntry struct {
	resolved     SandboxConfig
	settingsPath string
	mtime        time.Time
}

// SandboxResolver resolves the effective SandboxConfig for a project directory
// by merging user-scope and project-scope settings. Results are cached by the
// settings file's mtime. Safe for concurrent use.
type SandboxResolver struct {
	user  SandboxConfig
	mu    sync.Mutex
	cache map[string]sandboxCacheEntry
}

// NewSandboxResolver creates a resolver with the given user-scope sandbox config.
func NewSandboxResolver(user SandboxConfig) *SandboxResolver {
	return &SandboxResolver{
		user:  user,
		cache: make(map[string]sandboxCacheEntry),
	}
}

// ResolveProjectScope returns the raw project-scope SandboxConfig for projectPath
// without merging with user config. Returns nil when no project settings are found
// or the file has no [sandbox] table.
func (r *SandboxResolver) ResolveProjectScope(projectPath string) *SandboxConfig {
	if projectPath == "" {
		return nil
	}
	settingsPath := findProjectSettings(projectPath)
	if settingsPath == "" {
		return nil
	}
	proj, err := LoadProjectFrom(settingsPath)
	if err != nil || proj == nil {
		return nil
	}
	return proj.Sandbox
}

// Resolve returns the effective SandboxConfig for projectPath, merging user
// and project scopes. An empty projectPath or absent settings file returns the
// user config unchanged. Parse errors fall back to user config with a warning.
func (r *SandboxResolver) Resolve(projectPath string) SandboxConfig {
	if projectPath == "" {
		return r.user
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.cache[projectPath]; ok && entry.settingsPath != "" {
		info, err := os.Stat(entry.settingsPath)
		if err == nil && info.ModTime().Equal(entry.mtime) {
			return entry.resolved
		}
	}

	settingsPath := findProjectSettings(projectPath)
	if settingsPath == "" {
		r.cache[projectPath] = sandboxCacheEntry{resolved: r.user}
		return r.user
	}

	info, statErr := os.Stat(settingsPath)
	proj, err := LoadProjectFrom(settingsPath)
	resolved := r.user
	if err != nil {
		slog.Error("sandbox resolver: failed to load project settings", "path", settingsPath, "err", err)
	} else {
		resolved = MergeSandbox(r.user, proj.Sandbox)
	}

	var mtime time.Time
	if statErr == nil {
		mtime = info.ModTime()
	}
	r.cache[projectPath] = sandboxCacheEntry{
		resolved:     resolved,
		settingsPath: settingsPath,
		mtime:        mtime,
	}
	return resolved
}
