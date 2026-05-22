package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// maxToolCalls caps the number of dynamic-tool round-trips simulated within a
// single codex turn, preventing a runaway resume loop.
const maxToolCalls = 25

// toolCallFence is the fenced-block language tag claude must use to invoke a
// dynamic tool (see buildToolSystemPrompt).
const toolCallFence = "tool_call"

// dynToolSpec mirrors the DynamicToolSpec entries advertised by the orchestrator
// on thread/start.
type dynToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// toolCall is the parsed payload claude emits to invoke a dynamic tool.
type toolCall struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

// buildToolSystemPrompt renders the --append-system-prompt text that teaches
// claude how to call orchestrator-provided tools. Returns "" when there are no
// dynamic tools (claude then runs unchanged).
//
// claude has no native client-tool mechanism without MCP, so the shim simulates
// codex's behaviour: claude emits a sentinel-fenced JSON block and stops, the
// shim forwards it to the orchestrator via item/tool/call, and resumes claude
// with the result.
func buildToolSystemPrompt(tools []dynToolSpec) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# External tools\n\n")
	b.WriteString("You can call external tools provided by the orchestrator (not part of your normal toolset). ")
	b.WriteString("They run outside your sandbox; you never see their credentials.\n\n")
	b.WriteString("Available external tools:\n\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "## %s\n%s\n", t.Name, t.Description)
		if len(t.InputSchema) > 0 {
			fmt.Fprintf(&b, "Input schema (JSON Schema):\n```json\n%s\n```\n", string(t.InputSchema))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## How to call an external tool\n")
	fmt.Fprintf(&b, "Output a single fenced code block tagged `%s` containing a JSON object "+
		"`{\"tool\": <name>, \"arguments\": <object matching the input schema>}`, then STOP and end your "+
		"response immediately. Write nothing after the block. Example:\n\n", toolCallFence)
	fmt.Fprintf(&b, "```%s\n{\"tool\": %q, \"arguments\": { ... }}\n```\n\n", toolCallFence, tools[0].Name)
	b.WriteString("You will then receive the tool result as the next message and can continue or call another " +
		"external tool. Use this mechanism only for the external tool names listed above; for everything else " +
		"use your normal tools (Bash, Edit, Read, etc.).")
	return b.String()
}

// parseToolCall extracts a dynamic-tool invocation from claude's final message,
// if present. It looks for a fenced block tagged `tool_call` and decodes the
// JSON {tool, arguments} inside. Returns ok=false when no valid call is found.
func parseToolCall(text string) (toolCall, bool) {
	_, after, found := strings.Cut(text, "```"+toolCallFence)
	if !found {
		return toolCall{}, false
	}
	body, _, found := strings.Cut(strings.TrimPrefix(after, "\n"), "```")
	if !found {
		return toolCall{}, false
	}
	var c toolCall
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &c); err != nil || c.Tool == "" {
		return toolCall{}, false
	}
	return c, true
}

// formatToolResult renders the orchestrator's tool reply as the prompt for the
// next (resumed) claude invocation. The orchestrator returns {success, output}
// where output is the JSON-encoded tool result string.
func formatToolResult(call toolCall, result json.RawMessage) string {
	var r struct {
		Success *bool  `json:"success"`
		Output  string `json:"output"`
	}
	_ = json.Unmarshal(result, &r)
	status := ""
	if r.Success != nil && !*r.Success {
		status = " (the tool reported failure)"
	}
	return fmt.Sprintf("Result of external tool `%s`%s:\n\n%s\n\nContinue with the task.", call.Tool, status, r.Output)
}
