// Package streamjson parses the newline-delimited JSON stream emitted by
// `claude -p --output-format stream-json` into typed events.
// It is a pure leaf package with no dependencies beyond stdlib.
package streamjson

import "encoding/json"

// Event is the sealed interface implemented by all stream-json event types.
type Event interface{ isStreamEvent() }

// SystemInit is emitted by a type:"system", subtype:"init" line.
type SystemInit struct{ SessionID string }

// AssistantMessage is emitted by a type:"assistant" line.
type AssistantMessage struct {
	Text     string
	ToolUses []ToolUse
}

// ToolResult is emitted by a type:"user" line whose content contains a single
// tool_result block.
type ToolResult struct {
	ToolUseID string
	IsError   bool
	Content   string
}

// ToolResults is emitted by a type:"user" line whose content contains two or
// more tool_result blocks (parallel tool-use response). Callers that already
// handle ToolResult must also handle ToolResults.
type ToolResults struct {
	Results []ToolResult
}

// Result is emitted by a type:"result" line (session end).
type Result struct {
	Subtype    string // "success" | "error_*"
	ResultText string
	IsError    bool
	Usage      Usage
}

// Unknown is returned for any type not listed above. Callers may safely ignore
// it; the Scanner will never stop on an unknown type.
type Unknown struct{ Type string }

// ToolUse is a single tool-use block inside an AssistantMessage.
type ToolUse struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Usage holds token counts from a Result event.
// TotalTokens is zero when the upstream line omits it; use Total() for the
// effective total.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// Total returns TotalTokens when set, otherwise InputTokens + OutputTokens.
func (u Usage) Total() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens
}

func (SystemInit) isStreamEvent()       {}
func (AssistantMessage) isStreamEvent() {}
func (ToolResult) isStreamEvent()       {}
func (ToolResults) isStreamEvent()      {}
func (Result) isStreamEvent()           {}
func (Unknown) isStreamEvent()          {}
