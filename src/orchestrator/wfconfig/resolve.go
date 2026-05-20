package wfconfig

import (
	"fmt"
	"path/filepath"
)

// Resolve converts the raw WORKFLOW.md front matter map into a typed Config.
// workflowDir is used to resolve relative workspace.root paths.
func Resolve(raw map[string]any, workflowDir string) (Config, error) {
	var c Config
	if err := decodeTracker(raw, &c); err != nil {
		return Config{}, err
	}
	if err := decodePolling(raw, &c); err != nil {
		return Config{}, err
	}
	if err := decodeWorkspace(raw, &c); err != nil {
		return Config{}, err
	}
	if err := decodeHooks(raw, &c); err != nil {
		return Config{}, err
	}
	if err := decodeAgent(raw, &c); err != nil {
		return Config{}, err
	}
	if err := decodeCodex(raw, &c); err != nil {
		return Config{}, err
	}
	applyDefaults(&c, raw)
	expandFields(&c)
	normalizeWorkspaceRoot(&c, workflowDir)
	return c, validate(&c)
}

func expandFields(c *Config) {
	c.Tracker.APIKey = expandAPIKey(c.Tracker.APIKey)
	c.Workspace.Root = expandPath(c.Workspace.Root)
	c.Hooks.AfterCreate = expandHookScript(c.Hooks.AfterCreate)
	c.Hooks.BeforeRun = expandHookScript(c.Hooks.BeforeRun)
	c.Hooks.AfterRun = expandHookScript(c.Hooks.AfterRun)
	c.Hooks.BeforeRemove = expandHookScript(c.Hooks.BeforeRemove)
}

func normalizeWorkspaceRoot(c *Config, workflowDir string) {
	if c.Workspace.Root == "" {
		return
	}
	if !filepath.IsAbs(c.Workspace.Root) {
		c.Workspace.Root = filepath.Join(workflowDir, c.Workspace.Root)
	}
	c.Workspace.Root = filepath.Clean(c.Workspace.Root)
}

func decodeTracker(raw map[string]any, c *Config) error {
	v, ok := raw["tracker"]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: tracker must be a map", ErrConfigCoerce)
	}
	var err error
	if s, ok := m["kind"]; ok {
		if c.Tracker.Kind, err = coerceString(s); err != nil {
			return fmt.Errorf("tracker.kind: %w", err)
		}
	}
	if s, ok := m["endpoint"]; ok {
		if c.Tracker.Endpoint, err = coerceString(s); err != nil {
			return fmt.Errorf("tracker.endpoint: %w", err)
		}
	}
	if s, ok := m["api_key"]; ok {
		if c.Tracker.APIKey, err = coerceString(s); err != nil {
			return fmt.Errorf("tracker.api_key: %w", err)
		}
	}
	if s, ok := m["project_slug"]; ok {
		if c.Tracker.ProjectSlug, err = coerceString(s); err != nil {
			return fmt.Errorf("tracker.project_slug: %w", err)
		}
	}
	if s, ok := m["active_states"]; ok {
		if c.Tracker.ActiveStates, err = coerceStringSlice(s); err != nil {
			return fmt.Errorf("tracker.active_states: %w", err)
		}
	}
	if s, ok := m["terminal_states"]; ok {
		if c.Tracker.TerminalStates, err = coerceStringSlice(s); err != nil {
			return fmt.Errorf("tracker.terminal_states: %w", err)
		}
	}
	return nil
}

func decodePolling(raw map[string]any, c *Config) error {
	v, ok := raw["polling"]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: polling must be a map", ErrConfigCoerce)
	}
	if s, ok := m["interval_ms"]; ok {
		n, err := coerceInt(s)
		if err != nil {
			return fmt.Errorf("polling.interval_ms: %w", err)
		}
		c.Polling.IntervalMS = n
	}
	return nil
}

func decodeWorkspace(raw map[string]any, c *Config) error {
	v, ok := raw["workspace"]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: workspace must be a map", ErrConfigCoerce)
	}
	if s, ok := m["root"]; ok {
		str, err := coerceString(s)
		if err != nil {
			return fmt.Errorf("workspace.root: %w", err)
		}
		c.Workspace.Root = str
	}
	return nil
}

func decodeHooks(raw map[string]any, c *Config) error {
	v, ok := raw["hooks"]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: hooks must be a map", ErrConfigCoerce)
	}
	var err error
	if s, ok := m["timeout_ms"]; ok {
		if c.Hooks.TimeoutMS, err = coerceInt(s); err != nil {
			return fmt.Errorf("hooks.timeout_ms: %w", err)
		}
	}
	for _, f := range []struct {
		key  string
		dest *string
	}{
		{"after_create", &c.Hooks.AfterCreate},
		{"before_run", &c.Hooks.BeforeRun},
		{"after_run", &c.Hooks.AfterRun},
		{"before_remove", &c.Hooks.BeforeRemove},
	} {
		if s, ok := m[f.key]; ok {
			if *f.dest, err = coerceString(s); err != nil {
				return fmt.Errorf("hooks.%s: %w", f.key, err)
			}
		}
	}
	return nil
}

func decodeAgent(raw map[string]any, c *Config) error {
	v, ok := raw["agent"]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: agent must be a map", ErrConfigCoerce)
	}
	var err error
	if s, ok := m["max_concurrent_agents"]; ok {
		if c.Agent.MaxConcurrentAgents, err = coerceInt(s); err != nil {
			return fmt.Errorf("agent.max_concurrent_agents: %w", err)
		}
	}
	if s, ok := m["max_concurrent_agents_by_state"]; ok {
		if c.Agent.MaxConcurrentAgentsByState, err = coercePerStateMap(s); err != nil {
			return fmt.Errorf("agent.max_concurrent_agents_by_state: %w", err)
		}
	}
	if s, ok := m["max_turns"]; ok {
		if c.Agent.MaxTurns, err = coerceInt(s); err != nil {
			return fmt.Errorf("agent.max_turns: %w", err)
		}
	}
	if s, ok := m["max_retry_backoff_ms"]; ok {
		if c.Agent.MaxRetryBackoffMS, err = coerceInt(s); err != nil {
			return fmt.Errorf("agent.max_retry_backoff_ms: %w", err)
		}
	}
	return nil
}

func decodeCodex(raw map[string]any, c *Config) error {
	v, ok := raw["codex"]
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: codex must be a map", ErrConfigCoerce)
	}
	var err error
	if s, ok := m["command"]; ok {
		if c.Codex.Command, err = coerceString(s); err != nil {
			return fmt.Errorf("codex.command: %w", err)
		}
	}
	if s, ok := m["turn_timeout_ms"]; ok {
		if c.Codex.TurnTimeoutMS, err = coerceInt(s); err != nil {
			return fmt.Errorf("codex.turn_timeout_ms: %w", err)
		}
	}
	if s, ok := m["read_timeout_ms"]; ok {
		if c.Codex.ReadTimeoutMS, err = coerceInt(s); err != nil {
			return fmt.Errorf("codex.read_timeout_ms: %w", err)
		}
	}
	if s, ok := m["stall_timeout_ms"]; ok {
		if c.Codex.StallTimeoutMS, err = coerceInt(s); err != nil {
			return fmt.Errorf("codex.stall_timeout_ms: %w", err)
		}
	}
	return nil
}
