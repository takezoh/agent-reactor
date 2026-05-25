package streamjson

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Parse parses a single NDJSON line into an Event.
//
// Empty or whitespace-only lines return (nil, nil) — callers should skip them.
// Unknown event types return (Unknown{Type}, nil) so the stream continues.
// Malformed JSON returns (nil, error).
func Parse(line []byte) (Event, error) {
	if len(bytes.TrimSpace(line)) == 0 {
		return nil, nil
	}

	var head struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return nil, fmt.Errorf("streamjson: %w", err)
	}

	switch head.Type {
	case "system":
		return parseSystem(line, head.Subtype)
	case "assistant":
		return parseAssistant(line)
	case "user":
		return parseUser(line)
	case "result":
		return parseResult(line)
	default:
		return Unknown{Type: head.Type}, nil
	}
}

func parseSystem(line []byte, subtype string) (Event, error) {
	if subtype != "init" {
		return Unknown{Type: "system"}, nil
	}
	var v struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(line, &v); err != nil {
		return nil, fmt.Errorf("streamjson: %w", err)
	}
	return SystemInit{SessionID: v.SessionID}, nil
}

func parseAssistant(line []byte) (Event, error) {
	var v struct {
		Message *struct {
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &v); err != nil {
		return nil, fmt.Errorf("streamjson: %w", err)
	}
	if v.Message == nil {
		return AssistantMessage{}, nil
	}

	var texts []string
	var tools []ToolUse
	for _, b := range v.Message.Content {
		switch b.Type {
		case "text":
			if b.Text != "" {
				texts = append(texts, b.Text)
			}
		case "tool_use":
			tools = append(tools, ToolUse{ID: b.ID, Name: b.Name, Input: b.Input})
		}
	}

	text := ""
	if len(texts) > 0 {
		text = texts[0]
		for _, t := range texts[1:] {
			text += t
		}
	}
	return AssistantMessage{Text: text, ToolUses: tools}, nil
}

func parseUser(line []byte) (Event, error) {
	var v struct {
		Message *struct {
			Content []struct {
				Type      string `json:"type"`
				ToolUseID string `json:"tool_use_id"`
				IsError   bool   `json:"is_error"`
				Content   string `json:"content"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &v); err != nil {
		return nil, fmt.Errorf("streamjson: %w", err)
	}
	if v.Message == nil {
		return Unknown{Type: "user"}, nil
	}
	var results []ToolResult
	for _, b := range v.Message.Content {
		if b.Type == "tool_result" {
			results = append(results, ToolResult{
				ToolUseID: b.ToolUseID,
				IsError:   b.IsError,
				Content:   b.Content,
			})
		}
	}
	switch len(results) {
	case 0:
		return Unknown{Type: "user"}, nil
	case 1:
		return results[0], nil
	default:
		// Parallel tool-use: Claude emits multiple tool_result blocks in a single
		// user message. Return ToolResults so callers can handle all of them.
		return ToolResults{Results: results}, nil
	}
}

func parseResult(line []byte) (Event, error) {
	var v struct {
		Subtype string `json:"subtype"`
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
		Usage   *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(line, &v); err != nil {
		return nil, fmt.Errorf("streamjson: %w", err)
	}
	r := Result{
		Subtype:    v.Subtype,
		ResultText: v.Result,
		IsError:    v.IsError,
	}
	if v.Usage != nil {
		r.Usage = Usage{
			InputTokens:  v.Usage.InputTokens,
			OutputTokens: v.Usage.OutputTokens,
			TotalTokens:  v.Usage.TotalTokens,
		}
	}
	return r, nil
}
