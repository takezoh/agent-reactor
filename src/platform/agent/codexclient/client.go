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

// ResumeThread sends a `thread/resume` request and returns the raw result.
func ResumeThread(c *Conn, threadID, startDir string) (json.RawMessage, error) {
	params := map[string]any{"threadId": threadID}
	if startDir != "" {
		params["cwd"] = startDir
	}
	return c.Request(codexschema.MethodThreadResume, params)
}

// StartTurn sends a `turn/start` notification to begin a new turn.
func StartTurn(c *Conn, threadID, startDir string, stdin []byte) error {
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
	return c.Notify(codexschema.MethodTurnStart, params)
}
