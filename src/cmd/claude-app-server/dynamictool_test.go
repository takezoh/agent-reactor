package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseToolCall ---

func TestParseToolCall_valid(t *testing.T) {
	text := "I'll call the tool.\n```tool_call\n{\"tool\":\"linear_graphql\",\"arguments\":{\"query\":\"{ x }\"}}\n```"
	c, ok := parseToolCall(text)
	require.True(t, ok)
	assert.Equal(t, "linear_graphql", c.Tool)
	assert.JSONEq(t, `{"query":"{ x }"}`, string(c.Arguments))
}

func TestParseToolCall_noFence(t *testing.T) {
	_, ok := parseToolCall("no fence here")
	assert.False(t, ok)
}

func TestParseToolCall_emptyTool(t *testing.T) {
	text := "```tool_call\n{\"tool\":\"\",\"arguments\":{}}\n```"
	_, ok := parseToolCall(text)
	assert.False(t, ok, "empty tool name must be rejected")
}

func TestParseToolCall_invalidJSON(t *testing.T) {
	text := "```tool_call\nnot json\n```"
	_, ok := parseToolCall(text)
	assert.False(t, ok)
}

// --- formatToolResult ---

// orchestratorReply mirrors the toolCallReply the orchestrator sends.
type orchestratorReply struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

func makeReply(t *testing.T, success bool, output string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(orchestratorReply{Success: success, Output: output})
	require.NoError(t, err)
	return b
}

func TestFormatToolResult_successUsesOutput(t *testing.T) {
	call := toolCall{Tool: "linear_graphql"}
	inner := `{"success":true,"data":{"viewer":{"id":"u1"}}}`
	result := makeReply(t, true, inner)

	text := formatToolResult(call, result)

	assert.Contains(t, text, "Result of external tool `linear_graphql`")
	assert.Contains(t, text, inner)
	assert.Contains(t, text, "Continue with the task.")
	assert.NotContains(t, text, "(the tool reported failure)")
}

func TestFormatToolResult_failureReportsStatus(t *testing.T) {
	call := toolCall{Tool: "linear_graphql"}
	inner := `{"success":false,"errors":[{"message":"not found"}]}`
	result := makeReply(t, false, inner)

	text := formatToolResult(call, result)

	assert.Contains(t, text, "(the tool reported failure)")
	assert.Contains(t, text, "not found")
}

func TestFormatToolResult_outputIsNotDoubleWrapped(t *testing.T) {
	call := toolCall{Tool: "linear_graphql"}
	inner := `{"success":true,"data":{"issue":{"id":"DEV-1"}}}`
	result := makeReply(t, true, inner)

	text := formatToolResult(call, result)

	// The raw output JSON must appear verbatim — not double-encoded.
	assert.Contains(t, text, inner)
}
