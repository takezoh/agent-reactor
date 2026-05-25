package driver

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
)

type geminiTranscriptMessage struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Content      any                    `json:"content"`
	Display      any                    `json:"displayContent"`
	ToolCalls    []geminiTranscriptTool `json:"toolCalls"`
	Thoughts     []map[string]any       `json:"thoughts"`
	Model        string                 `json:"model"`
	Timestamp    string                 `json:"timestamp"`
	Description  string                 `json:"description"`
	Result       string                 `json:"resultDisplay"`
	MetadataOnly bool                   `json:"-"`
}

type geminiTranscriptTool struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName"`
	Args        map[string]any `json:"args"`
	Status      string         `json:"status"`
}

type geminiTranscriptMetadata struct {
	SessionID   string `json:"sessionId"`
	ProjectHash string `json:"projectHash"`
	Summary     string `json:"summary"`
}

func newGeminiTranscriptParse() func(context.Context, GeminiTranscriptParseInput) (GeminiTranscriptParseResult, error) {
	return geminiTranscriptParse
}

func geminiTranscriptParse(ctx context.Context, in GeminiTranscriptParseInput) (GeminiTranscriptParseResult, error) {
	if err := ctx.Err(); err != nil {
		return GeminiTranscriptParseResult{}, err
	}
	f, err := os.Open(in.Path)
	if err != nil {
		return GeminiTranscriptParseResult{}, err
	}
	defer f.Close()

	meta, messages, err := scanGeminiTranscript(ctx, f)
	if err != nil {
		return GeminiTranscriptParseResult{}, err
	}

	lastPrompt, lastAssistant, currentTool, recentTurns := processGeminiMessages(messages)
	return GeminiTranscriptParseResult{
		Title:                strings.TrimSpace(meta.Summary),
		LastPrompt:           lastPrompt,
		LastAssistantMessage: lastAssistant,
		CurrentTool:          currentTool,
		RecentTurns:          recentTurns,
	}, nil
}

func scanGeminiTranscript(ctx context.Context, f *os.File) (geminiTranscriptMetadata, []geminiTranscriptMessage, error) {
	var (
		meta     geminiTranscriptMetadata
		messages []geminiTranscriptMessage
	)
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return meta, nil, err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if set, ok := raw["$set"].(map[string]any); ok {
			applyGeminiMetadataUpdate(&meta, set)
			continue
		}
		if isGeminiMetadata(raw) {
			applyGeminiMetadataUpdate(&meta, raw)
			continue
		}
		msg, ok := parseGeminiTranscriptMessage(raw)
		if !ok {
			continue
		}
		messages = upsertGeminiMessage(messages, msg)
	}
	if err := scanner.Err(); err != nil {
		return meta, nil, err
	}
	return meta, messages, nil
}

func processGeminiMessages(messages []geminiTranscriptMessage) (string, string, string, []SummaryTurn) {
	var (
		lastPrompt    string
		lastAssistant string
		currentTool   string
		recentTurns   []SummaryTurn
	)
	for _, msg := range messages {
		text := strings.TrimSpace(geminiPartListToString(msg.Display))
		if text == "" {
			text = strings.TrimSpace(geminiPartListToString(msg.Content))
		}
		switch msg.Type {
		case "user":
			if text != "" {
				lastPrompt = text
				recentTurns = appendGeminiRecentTurn(recentTurns, "user", text)
			}
		case "gemini":
			if text != "" {
				lastAssistant = text
				recentTurns = appendGeminiRecentTurn(recentTurns, "assistant", text)
			}
			if tool := lastGeminiCurrentTool(msg.ToolCalls); tool != "" {
				currentTool = tool
			}
		}
	}
	return lastPrompt, lastAssistant, currentTool, recentTurns
}

func isGeminiMetadata(raw map[string]any) bool {
	_, hasSessionID := raw["sessionId"]
	_, hasProjectHash := raw["projectHash"]
	return hasSessionID && hasProjectHash
}

func applyGeminiMetadataUpdate(meta *geminiTranscriptMetadata, raw map[string]any) {
	if v, _ := raw["sessionId"].(string); v != "" {
		meta.SessionID = v
	}
	if v, _ := raw["projectHash"].(string); v != "" {
		meta.ProjectHash = v
	}
	if v, _ := raw["summary"].(string); v != "" {
		meta.Summary = v
	}
}

func parseGeminiTranscriptMessage(raw map[string]any) (geminiTranscriptMessage, bool) {
	id, _ := raw["id"].(string)
	if id == "" {
		return geminiTranscriptMessage{}, false
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return geminiTranscriptMessage{}, false
	}
	var msg geminiTranscriptMessage
	if err := json.Unmarshal(b, &msg); err != nil {
		return geminiTranscriptMessage{}, false
	}
	return msg, true
}

func upsertGeminiMessage(messages []geminiTranscriptMessage, msg geminiTranscriptMessage) []geminiTranscriptMessage {
	for i := range messages {
		if messages[i].ID == msg.ID {
			messages[i] = msg
			return messages
		}
	}
	return append(messages, msg)
}

func geminiPartListToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := m["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func lastGeminiCurrentTool(calls []geminiTranscriptTool) string {
	for i := len(calls) - 1; i >= 0; i-- {
		name := strings.TrimSpace(firstNonEmpty(calls[i].DisplayName, calls[i].Name))
		if name == "" {
			continue
		}
		if calls[i].Status == "" || calls[i].Status == "pending" || calls[i].Status == "running" {
			return name
		}
		if i == len(calls)-1 {
			return name
		}
	}
	return ""
}

func appendGeminiRecentTurn(turns []SummaryTurn, role, text string) []SummaryTurn {
	if text == "" {
		return turns
	}
	turns = append(turns, SummaryTurn{Role: role, Text: text})
	if len(turns) <= 64 {
		return turns
	}
	out := make([]SummaryTurn, 64)
	copy(out, turns[len(turns)-64:])
	return out
}
