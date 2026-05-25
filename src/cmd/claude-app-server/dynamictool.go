package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// dynToolSpec mirrors the DynamicToolSpec entries advertised by the orchestrator
// on thread/start.
type dynToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// buildCLISystemPrompt renders the --append-system-prompt text that tells
// claude about the CLI tools available in its PATH.  Returns "" when there
// are no dynamic tools (claude then runs unchanged).
//
// Each dynamic tool is exposed as a CLI binary in the PATH; claude invokes
// it via its native Bash tool.  The tool bridge running inside the shim
// intercepts the CLI call and forwards it to the orchestrator as
// item/tool/call, keeping credentials outside the container.
func buildCLISystemPrompt(tools []dynToolSpec) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# External tools\n\n")
	b.WriteString("You have access to CLI tools in your PATH that let you interact with " +
		"external services. They run outside your sandbox; you never see their credentials.\n\n")
	b.WriteString("Available external tools:\n\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "## %s\n%s\n", t.Name, t.Description)
		if len(t.InputSchema) > 0 {
			fmt.Fprintf(&b, "Input schema (JSON Schema):\n```json\n%s\n```\n", string(t.InputSchema))
		}
		b.WriteString("\n## How to call\n")
		fmt.Fprintf(&b, "Use your Bash tool to run: `%s '<json-arguments>'`\n\n", t.Name)
		b.WriteString("Pass the JSON arguments matching the input schema as a single quoted string. " +
			"The tool writes the result to stdout.\n\n")
	}
	return b.String()
}
