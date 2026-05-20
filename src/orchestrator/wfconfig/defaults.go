package wfconfig

import (
	"os"
	"path/filepath"
)

// applyDefaults fills fields absent from raw with SPEC §6.4 defaults.
// Only applies a default when the corresponding key is missing from raw.
func applyDefaults(c *Config, raw map[string]any) {
	tracker, _ := raw["tracker"].(map[string]any)
	polling, _ := raw["polling"].(map[string]any)
	workspace, _ := raw["workspace"].(map[string]any)
	hooks, _ := raw["hooks"].(map[string]any)
	agent, _ := raw["agent"].(map[string]any)
	codex, _ := raw["codex"].(map[string]any)

	applyTrackerDefaults(c, tracker)
	if _, ok := polling["interval_ms"]; !ok {
		if c.Polling.IntervalMS == 0 {
			c.Polling.IntervalMS = 30000
		}
	}
	if _, ok := workspace["root"]; !ok {
		if c.Workspace.Root == "" {
			c.Workspace.Root = filepath.Join(os.TempDir(), "symphony_workspaces")
		}
	}
	if _, ok := hooks["timeout_ms"]; !ok {
		if c.Hooks.TimeoutMS == 0 {
			c.Hooks.TimeoutMS = 60000
		}
	}
	applyAgentDefaults(c, agent)
	applyCodexDefaults(c, codex)
}

func applyTrackerDefaults(c *Config, tracker map[string]any) {
	if _, ok := tracker["endpoint"]; !ok {
		if c.Tracker.Kind == "linear" && c.Tracker.Endpoint == "" {
			c.Tracker.Endpoint = "https://api.linear.app/graphql"
		}
	}
	if _, ok := tracker["active_states"]; !ok {
		if len(c.Tracker.ActiveStates) == 0 {
			c.Tracker.ActiveStates = []string{"Todo", "In Progress"}
		}
	}
	if _, ok := tracker["terminal_states"]; !ok {
		if len(c.Tracker.TerminalStates) == 0 {
			c.Tracker.TerminalStates = []string{"Done", "Canceled", "Duplicate", "Closed", "Completed"}
		}
	}
}

func applyAgentDefaults(c *Config, agent map[string]any) {
	if _, ok := agent["max_concurrent_agents"]; !ok {
		if c.Agent.MaxConcurrentAgents == 0 {
			c.Agent.MaxConcurrentAgents = 10
		}
	}
	if _, ok := agent["max_turns"]; !ok {
		if c.Agent.MaxTurns == 0 {
			c.Agent.MaxTurns = 20
		}
	}
	if _, ok := agent["max_retry_backoff_ms"]; !ok {
		if c.Agent.MaxRetryBackoffMS == 0 {
			c.Agent.MaxRetryBackoffMS = 300000
		}
	}
}

func applyCodexDefaults(c *Config, codex map[string]any) {
	if _, ok := codex["command"]; !ok {
		if c.Codex.Command == "" {
			c.Codex.Command = "codex app-server"
		}
	}
	if _, ok := codex["turn_timeout_ms"]; !ok {
		if c.Codex.TurnTimeoutMS == 0 {
			c.Codex.TurnTimeoutMS = 3600000
		}
	}
	if _, ok := codex["read_timeout_ms"]; !ok {
		if c.Codex.ReadTimeoutMS == 0 {
			c.Codex.ReadTimeoutMS = 5000
		}
	}
	if _, ok := codex["stall_timeout_ms"]; !ok {
		if c.Codex.StallTimeoutMS == 0 {
			c.Codex.StallTimeoutMS = 300000
		}
	}
}
