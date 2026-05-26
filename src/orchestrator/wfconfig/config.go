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
	Server    ServerConfig
}

// ServerConfig holds §13.7 HTTP observability server settings.
// Port 0 means the server is disabled unless overridden by the CLI --port flag.
type ServerConfig struct {
	Port int    // TCP port; 0 = disabled, positive = listen on that port, ephemeral via net.Listen
	Bind string // bind address, default 127.0.0.1 (loopback)
}

// TrackerConfig holds §5.3.1 tracker settings.
type TrackerConfig struct {
	Kind           string
	Endpoint       string
	APIKey         string
	ProjectSlugs   []string
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
	// Pass-through Codex config values (§5.3.6). Kept as raw strings; the
	// targeted Codex app-server version defines the valid set. Empty means
	// unset (use the app-server's own default).
	ApprovalPolicy    string
	ThreadSandbox     string
	TurnSandboxPolicy string
}

var (
	ErrConfigValidation = errors.New("wfconfig: validation error")
	ErrConfigCoerce     = errors.New("wfconfig: type coercion error")
)
