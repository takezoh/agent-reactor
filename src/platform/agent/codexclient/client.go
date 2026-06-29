package codexclient

import (
	"encoding/json"

	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
)

// Initialize performs the JSON-RPC handshake: `initialize` request followed by
// an `initialized` notification.
func Initialize(c *Conn) error {
	if _, err := c.Request(codexschema.MethodInitialize, map[string]any{
		"clientInfo":   map[string]any{"name": "agent-reactor", "version": "0"},
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

// ResumeOptions holds optional parameters for thread/resume.
type ResumeOptions struct {
	ThreadID    string
	RolloutPath string
	Cwd         string
}

// ThreadSession is the canonical thread locator returned by thread/start and
// thread/resume. ThreadID is the live routing identity; RolloutPath is the
// durable locator.
type ThreadSession struct {
	ThreadID    string
	SessionID   string
	RolloutPath string
	Raw         json.RawMessage
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
func StartThread(c *Conn, cwd string, dynamicTools []any, opts ThreadOptions) (ThreadSession, error) {
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
		return ThreadSession{}, err
	}
	return decodeThreadSession(res)
}

// ResumeThread sends a `thread/resume` request and returns the canonical thread locator.
func ResumeThread(c *Conn, opts ResumeOptions) (ThreadSession, error) {
	params := map[string]any{}
	if opts.ThreadID != "" {
		params["threadId"] = opts.ThreadID
	}
	if opts.RolloutPath != "" {
		params["path"] = opts.RolloutPath
	}
	if opts.Cwd != "" {
		params["cwd"] = opts.Cwd
	}
	res, err := c.Request(codexschema.MethodThreadResume, params)
	if err != nil {
		return ThreadSession{}, err
	}
	return decodeThreadSession(res)
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

func decodeThreadSession(raw json.RawMessage) (ThreadSession, error) {
	var payload struct {
		Thread struct {
			ID        string `json:"id"`
			SessionID string `json:"sessionId"`
			Path      string `json:"path"`
		} `json:"thread"`
		ThreadID  string `json:"threadId"`
		SessionID string `json:"sessionId"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ThreadSession{}, err
	}
	session := ThreadSession{
		ThreadID:    payload.Thread.ID,
		SessionID:   payload.Thread.SessionID,
		RolloutPath: payload.Thread.Path,
		Raw:         raw,
	}
	if session.ThreadID == "" {
		session.ThreadID = payload.ThreadID
	}
	if session.SessionID == "" {
		session.SessionID = payload.SessionID
	}
	if session.RolloutPath == "" {
		session.RolloutPath = payload.Path
	}
	return session, nil
}
