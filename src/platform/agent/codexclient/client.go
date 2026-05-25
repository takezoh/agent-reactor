package codexclient

import (
	"encoding/json"

	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

// Initialize performs the JSON-RPC handshake: `initialize` request followed by
// an `initialized` notification.
func Initialize(c *Conn) error {
	if _, err := c.Request(codexschema.MethodInitialize, map[string]any{
		"clientInfo":   map[string]any{"name": "roost", "version": "0"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		return err
	}
	return c.Notify(codexschema.MethodInitialized, map[string]any{})
}

// ThreadOptions holds optional per-thread parameters for thread/start (SPEC §10.2).
// Empty string fields are omitted from the wire params.
type ThreadOptions struct {
	ApprovalPolicy string // codex.approval_policy (e.g. "never", "on-request")
	SandboxMode    string // codex.thread_sandbox (e.g. "danger-full-access", "workspace-write")
	ServiceName    string // thread label: "<identifier>: <title>"
}

// TurnOptions holds optional per-turn parameters for turn/start (SPEC §10.2).
// Empty string fields are omitted from the wire params.
type TurnOptions struct {
	ApprovalPolicy string // codex.approval_policy
	SandboxPolicy  string // codex.turn_sandbox_policy; sent as {"type": "<value>"}
}

// StartThread sends a `thread/start` request and returns the new thread id.
// dynamicTools advertises client-side tools (SPEC §10.5) for the thread; pass
// nil to advertise none. The agent invokes them via `item/tool/call`.
// opts carries §10.2 approval/sandbox policy and the thread title.
func StartThread(c *Conn, cwd string, dynamicTools []any, opts ThreadOptions) (string, error) {
	params := map[string]any{}
	if cwd != "" {
		params["cwd"] = cwd
	}
	if len(dynamicTools) > 0 {
		params["dynamicTools"] = dynamicTools
	}
	if opts.ApprovalPolicy != "" {
		params["approvalPolicy"] = opts.ApprovalPolicy
	}
	if opts.SandboxMode != "" {
		params["sandbox"] = opts.SandboxMode
	}
	if opts.ServiceName != "" {
		params["serviceName"] = opts.ServiceName
	}
	res, err := c.Request(codexschema.MethodThreadStart, params)
	if err != nil {
		return "", err
	}
	var p struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(res, &p); err != nil {
		return "", err
	}
	return p.Thread.ID, nil
}

// ResumeThread sends a `thread/resume` request and returns the raw result.
func ResumeThread(c *Conn, threadID, startDir string) (json.RawMessage, error) {
	params := map[string]any{"threadId": threadID}
	if startDir != "" {
		params["cwd"] = startDir
	}
	return c.Request(codexschema.MethodThreadResume, params)
}

// StartTurn sends a `turn/start` notification to begin a new turn.
// opts carries §10.2 approval/sandbox policy for the turn.
func StartTurn(c *Conn, threadID, startDir string, stdin []byte, opts TurnOptions) error {
	params := map[string]any{}
	if threadID != "" {
		params["threadId"] = threadID
	}
	if startDir != "" {
		params["cwd"] = startDir
	}
	if len(stdin) > 0 {
		params["message"] = string(stdin)
	}
	if opts.ApprovalPolicy != "" {
		params["approvalPolicy"] = opts.ApprovalPolicy
	}
	if opts.SandboxPolicy != "" {
		params["sandboxPolicy"] = map[string]any{"type": opts.SandboxPolicy}
	}
	return c.Notify(codexschema.MethodTurnStart, params)
}
