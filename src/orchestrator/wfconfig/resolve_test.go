package wfconfig

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResolve_AppliesAllDefaults(t *testing.T) {
	cfg, err := Resolve(map[string]any{}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Polling.IntervalMS != 30000 {
		t.Errorf("IntervalMS = %d, want 30000", cfg.Polling.IntervalMS)
	}
	if cfg.Hooks.TimeoutMS != 60000 {
		t.Errorf("HooksTimeoutMS = %d, want 60000", cfg.Hooks.TimeoutMS)
	}
	if cfg.Agent.MaxConcurrentAgents != 10 {
		t.Errorf("MaxConcurrentAgents = %d, want 10", cfg.Agent.MaxConcurrentAgents)
	}
	if cfg.Agent.MaxTurns != 20 {
		t.Errorf("MaxTurns = %d, want 20", cfg.Agent.MaxTurns)
	}
	if cfg.Agent.MaxRetryBackoffMS != 300000 {
		t.Errorf("MaxRetryBackoffMS = %d, want 300000", cfg.Agent.MaxRetryBackoffMS)
	}
	if cfg.Codex.Command != "codex app-server" {
		t.Errorf("Codex.Command = %q, want %q", cfg.Codex.Command, "codex app-server")
	}
	if cfg.Codex.TurnTimeoutMS != 3600000 {
		t.Errorf("TurnTimeoutMS = %d, want 3600000", cfg.Codex.TurnTimeoutMS)
	}
	if cfg.Codex.ReadTimeoutMS != 5000 {
		t.Errorf("ReadTimeoutMS = %d, want 5000", cfg.Codex.ReadTimeoutMS)
	}
	if cfg.Codex.StallTimeoutMS != 300000 {
		t.Errorf("StallTimeoutMS = %d, want 300000", cfg.Codex.StallTimeoutMS)
	}
	wantRoot := filepath.Join(os.TempDir(), "symphony_workspaces")
	if cfg.Workspace.Root != wantRoot {
		t.Errorf("Workspace.Root = %q, want %q", cfg.Workspace.Root, wantRoot)
	}
	if len(cfg.Tracker.ActiveStates) != 2 {
		t.Errorf("ActiveStates len = %d, want 2", len(cfg.Tracker.ActiveStates))
	}
	if len(cfg.Tracker.TerminalStates) != 5 {
		t.Errorf("TerminalStates len = %d, want 5", len(cfg.Tracker.TerminalStates))
	}
}

func TestResolve_LinearTrackerEndpointDefault(t *testing.T) {
	raw := map[string]any{
		"tracker": map[string]any{"kind": "linear"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.Endpoint != "https://api.linear.app/graphql" {
		t.Errorf("Endpoint = %q, want default linear endpoint", cfg.Tracker.Endpoint)
	}
}

func TestResolve_VarExpansion_APIKey(t *testing.T) {
	t.Setenv("MY_KEY", "sk-secret")
	raw := map[string]any{
		"tracker": map[string]any{"api_key": "$MY_KEY"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.APIKey != "sk-secret" {
		t.Errorf("APIKey = %q, want %q", cfg.Tracker.APIKey, "sk-secret")
	}
}

func TestResolve_VarExpansion_OnlyAnchoredForm(t *testing.T) {
	t.Setenv("MY_KEY", "sk-secret")
	raw := map[string]any{
		"tracker": map[string]any{"api_key": "prefix-$MY_KEY"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.APIKey != "prefix-$MY_KEY" {
		t.Errorf("APIKey = %q, want literal %q", cfg.Tracker.APIKey, "prefix-$MY_KEY")
	}
}

func TestResolve_VarExpansion_EmptyEnv(t *testing.T) {
	t.Setenv("MY_KEY", "")
	raw := map[string]any{
		"tracker": map[string]any{"api_key": "$MY_KEY"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error (empty env should not be a config error): %v", err)
	}
	if cfg.Tracker.APIKey != "" {
		t.Errorf("APIKey = %q, want empty string", cfg.Tracker.APIKey)
	}
}

func TestResolve_TildeExpansion_WorkspaceRoot(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}
	raw := map[string]any{
		"workspace": map[string]any{"root": "~/wks"},
	}
	cfg, cfgErr := Resolve(raw, t.TempDir())
	if cfgErr != nil {
		t.Fatalf("unexpected error: %v", cfgErr)
	}
	want := filepath.Clean(filepath.Join(home, "wks"))
	if cfg.Workspace.Root != want {
		t.Errorf("Workspace.Root = %q, want %q", cfg.Workspace.Root, want)
	}
}

func TestResolve_WorkspaceRoot_RelativeToWorkflowDir(t *testing.T) {
	dir := t.TempDir()
	raw := map[string]any{
		"workspace": map[string]any{"root": "workspaces"},
	}
	cfg, err := Resolve(raw, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Clean(filepath.Join(dir, "workspaces"))
	if cfg.Workspace.Root != want {
		t.Errorf("Workspace.Root = %q, want %q", cfg.Workspace.Root, want)
	}
}

func TestResolve_CodexCommandPreserved(t *testing.T) {
	t.Setenv("X", "expanded")
	raw := map[string]any{
		"codex": map[string]any{"command": "my cmd $X"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Codex.Command != "my cmd $X" {
		t.Errorf("Codex.Command = %q, want literal %q", cfg.Codex.Command, "my cmd $X")
	}
}

func TestResolve_TrackerEndpointNotExpanded(t *testing.T) {
	t.Setenv("X", "expanded")
	raw := map[string]any{
		"tracker": map[string]any{"endpoint": "https://api/$X/foo"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.Endpoint != "https://api/$X/foo" {
		t.Errorf("Endpoint = %q, want literal", cfg.Tracker.Endpoint)
	}
}

func TestResolve_PerStateConcurrencyNormalized(t *testing.T) {
	raw := map[string]any{
		"agent": map[string]any{
			"max_concurrent_agents_by_state": map[string]any{
				"In Progress": 3,
				"todo":        1,
				"bad":         -1,
				"x":           "abc",
			},
		},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := cfg.Agent.MaxConcurrentAgentsByState
	if m["in progress"] != 3 {
		t.Errorf("in progress = %d, want 3", m["in progress"])
	}
	if m["todo"] != 1 {
		t.Errorf("todo = %d, want 1", m["todo"])
	}
	if _, exists := m["bad"]; exists {
		t.Error("bad (negative) entry should be dropped")
	}
	if _, exists := m["x"]; exists {
		t.Error("x (non-int) entry should be dropped")
	}
}

func TestResolve_MaxTurnsInvalid_ReturnsValidationErr(t *testing.T) {
	raw := map[string]any{
		"agent": map[string]any{"max_turns": 0},
	}
	_, err := Resolve(raw, t.TempDir())
	if !errors.Is(err, ErrConfigValidation) {
		t.Errorf("err = %v, want ErrConfigValidation", err)
	}
}

func TestResolve_HookTimeoutInvalid_ReturnsValidationErr(t *testing.T) {
	raw := map[string]any{
		"hooks": map[string]any{"timeout_ms": 0},
	}
	_, err := Resolve(raw, t.TempDir())
	if !errors.Is(err, ErrConfigValidation) {
		t.Errorf("err = %v, want ErrConfigValidation", err)
	}
}

func TestResolve_CoerceFromString(t *testing.T) {
	raw := map[string]any{
		"polling": map[string]any{"interval_ms": "5000"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Polling.IntervalMS != 5000 {
		t.Errorf("IntervalMS = %d, want 5000", cfg.Polling.IntervalMS)
	}
}

func TestResolve_CoerceFailure_ReturnsCoerceErr(t *testing.T) {
	raw := map[string]any{
		"polling": map[string]any{"interval_ms": "abc"},
	}
	_, err := Resolve(raw, t.TempDir())
	if !errors.Is(err, ErrConfigCoerce) {
		t.Errorf("err = %v, want ErrConfigCoerce", err)
	}
}

func TestResolve_UnknownKeysIgnored(t *testing.T) {
	raw := map[string]any{
		"extra":   1,
		"another": "value",
	}
	_, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Errorf("unknown keys should be ignored, got error: %v", err)
	}
}

func TestResolve_WorkspaceRoot_AbsolutePreserved(t *testing.T) {
	absPath := filepath.Join(t.TempDir(), "abs_workspaces")
	raw := map[string]any{
		"workspace": map[string]any{"root": absPath},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workspace.Root != filepath.Clean(absPath) {
		t.Errorf("Workspace.Root = %q, want %q", cfg.Workspace.Root, absPath)
	}
}

func TestResolve_VarExpansion_WorkspaceRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WS_ROOT", dir)
	raw := map[string]any{
		"workspace": map[string]any{"root": "$WS_ROOT"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(cfg.Workspace.Root, dir) {
		t.Errorf("Workspace.Root = %q, want prefix %q", cfg.Workspace.Root, dir)
	}
}

// SPEC §5.3.1 / §6.4: terminal_states default is exactly these five values,
// including both "Cancelled" (British) and "Canceled" (American) spellings.
func TestResolve_TerminalStatesDefault_MatchesSPEC(t *testing.T) {
	cfg, err := Resolve(map[string]any{}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Closed", "Cancelled", "Canceled", "Duplicate", "Done"}
	got := cfg.Tracker.TerminalStates
	if !slices.Equal(got, want) {
		t.Errorf("TerminalStates = %v, want %v", got, want)
	}
}

// SPEC §5.3.6: stall_timeout_ms <= 0 is valid and means "stall detection
// disabled"; it must not fail validation.
func TestResolve_StallTimeoutZero_Allowed(t *testing.T) {
	raw := map[string]any{
		"codex": map[string]any{"stall_timeout_ms": 0},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("stall_timeout_ms=0 should be valid (disabled), got: %v", err)
	}
	if cfg.Codex.StallTimeoutMS != 0 {
		t.Errorf("StallTimeoutMS = %d, want 0", cfg.Codex.StallTimeoutMS)
	}
}

// SPEC §5.3.6: approval_policy / thread_sandbox / turn_sandbox_policy are
// pass-through Codex config values, kept verbatim as strings.
func TestResolve_CodexPassthroughFields_Captured(t *testing.T) {
	raw := map[string]any{
		"codex": map[string]any{
			"approval_policy":     "on-request",
			"thread_sandbox":      "workspace-write",
			"turn_sandbox_policy": "read-only",
		},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Codex.ApprovalPolicy != "on-request" {
		t.Errorf("ApprovalPolicy = %q, want %q", cfg.Codex.ApprovalPolicy, "on-request")
	}
	if cfg.Codex.ThreadSandbox != "workspace-write" {
		t.Errorf("ThreadSandbox = %q, want %q", cfg.Codex.ThreadSandbox, "workspace-write")
	}
	if cfg.Codex.TurnSandboxPolicy != "read-only" {
		t.Errorf("TurnSandboxPolicy = %q, want %q", cfg.Codex.TurnSandboxPolicy, "read-only")
	}
}

// SPEC §5.3.1: absent api_key falls back to the canonical env var LINEAR_API_KEY.
func TestResolve_LinearAPIKey_CanonicalEnvFallback(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "sk-fallback")
	raw := map[string]any{
		"tracker": map[string]any{"kind": "linear"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.APIKey != "sk-fallback" {
		t.Errorf("APIKey = %q, want %q (canonical env fallback)", cfg.Tracker.APIKey, "sk-fallback")
	}
}

// SPEC §5.3.1: api_key that expands to empty also falls back to LINEAR_API_KEY.
func TestResolve_LinearAPIKey_EmptyResolvesToCanonicalEnv(t *testing.T) {
	t.Setenv("MY_KEY", "")
	t.Setenv("LINEAR_API_KEY", "sk-canonical")
	raw := map[string]any{
		"tracker": map[string]any{"kind": "linear", "api_key": "$MY_KEY"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.APIKey != "sk-canonical" {
		t.Errorf("APIKey = %q, want %q (canonical env fallback after empty expansion)", cfg.Tracker.APIKey, "sk-canonical")
	}
}

// SPEC §5.3.1: an explicit api_key takes precedence over LINEAR_API_KEY.
func TestResolve_LinearAPIKey_ExplicitKeyWins(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "sk-canonical")
	raw := map[string]any{
		"tracker": map[string]any{"kind": "linear", "api_key": "sk-explicit"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.APIKey != "sk-explicit" {
		t.Errorf("APIKey = %q, want %q (explicit key should win)", cfg.Tracker.APIKey, "sk-explicit")
	}
}

func TestResolve_NonLinearTracker_NoCanonicalEnvFallback(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "sk-should-not-appear")
	raw := map[string]any{
		"tracker": map[string]any{"kind": "github"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.APIKey != "" {
		t.Errorf("APIKey = %q, want empty (non-linear tracker must not use LINEAR_API_KEY)", cfg.Tracker.APIKey)
	}
}

// SPEC §5.3.1: when LINEAR_API_KEY is also absent, fallback yields empty string without error.
func TestResolve_LinearAPIKey_FallbackEnvAbsent(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "")
	raw := map[string]any{
		"tracker": map[string]any{"kind": "linear"},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracker.APIKey != "" {
		t.Errorf("APIKey = %q, want empty when LINEAR_API_KEY is unset", cfg.Tracker.APIKey)
	}
}

// SPEC §6.1: hook scripts are arbitrary shell command strings and must not be
// rewritten at config time, even when the whole value is a single $VAR token.
func TestResolve_HookScriptNotExpanded(t *testing.T) {
	t.Setenv("DEPLOY", "should-not-leak")
	raw := map[string]any{
		"hooks": map[string]any{
			"after_create": "$DEPLOY",
			"before_run":   "npm install && echo $DEPLOY",
		},
	}
	cfg, err := Resolve(raw, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hooks.AfterCreate != "$DEPLOY" {
		t.Errorf("AfterCreate = %q, want literal %q", cfg.Hooks.AfterCreate, "$DEPLOY")
	}
	if cfg.Hooks.BeforeRun != "npm install && echo $DEPLOY" {
		t.Errorf("BeforeRun = %q, want literal", cfg.Hooks.BeforeRun)
	}
}
