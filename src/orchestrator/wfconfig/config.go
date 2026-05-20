// Package wfconfig resolves WORKFLOW.md front matter into a typed Config per SPEC §5.3/§6.1/§6.4.
package wfconfig

import "errors"

// Config is the fully-resolved, typed service configuration.
type Config struct {
	Tracker   TrackerConfig
	Polling   PollingConfig
	Workspace WorkspaceConfig
	Hooks     HooksConfig
	Agent     AgentConfig
	Codex     CodexConfig
}

// TrackerConfig holds §5.3.1 tracker settings.
type TrackerConfig struct {
	Kind           string
	Endpoint       string
	APIKey         string
	ProjectSlug    string
	ActiveStates   []string
	TerminalStates []string
}

// PollingConfig holds §5.3.2 polling settings.
type PollingConfig struct {
	IntervalMS int
}

// WorkspaceConfig holds §5.3.3 workspace settings.
type WorkspaceConfig struct {
	Root string // absolute path
}

// HooksConfig holds §5.3.4 hook settings.
type HooksConfig struct {
	TimeoutMS    int
	AfterCreate  string
	BeforeRun    string
	AfterRun     string
	BeforeRemove string
}

// AgentConfig holds §5.3.5 agent settings.
type AgentConfig struct {
	MaxConcurrentAgents        int
	MaxConcurrentAgentsByState map[string]int // lowercase-normalized keys
	MaxTurns                   int
	MaxRetryBackoffMS          int
}

// CodexConfig holds §5.3.6 codex settings.
type CodexConfig struct {
	Command        string // shell string, kept verbatim
	TurnTimeoutMS  int
	ReadTimeoutMS  int
	StallTimeoutMS int
}

var (
	ErrConfigValidation = errors.New("wfconfig: validation error")
	ErrConfigCoerce     = errors.New("wfconfig: type coercion error")
)
