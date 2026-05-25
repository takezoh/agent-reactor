package transcript

import (
	"encoding/json"
	"strconv"
	"strings"
)

// transcript_render.go holds rendering helpers extracted from transcript.go
// to keep file sizes within the 500-line limit.

// RenderEntries joins the text of all entries with newlines, skipping blanks.
func RenderEntries(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		text := strings.TrimSpace(entry.Text)
		if text != "" {
			lines = append(lines, text)
		}
	}
	return strings.Join(lines, "\n")
}

type rolloutLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

func (p *Parser) renderToolCall(payload json.RawMessage, fallback string) (Entry, bool) {
	var item struct {
		Type      string `json:"type"`
		Name      string `json:"name"`
		CallID    string `json:"call_id"`
		Arguments string `json:"arguments"`
		Action    struct {
			Type    string `json:"type"`
			Query   string `json:"query"`
			Command string `json:"command"`
		} `json:"action"`
		Command string `json:"command"`
		Query   string `json:"query"`
	}
	_ = json.Unmarshal(payload, &item)
	name := firstNonEmpty(item.Name, item.Type, fallback)
	detail := firstNonEmpty(
		previewText(item.Command),
		previewText(item.Query),
		previewText(item.Action.Command),
		previewText(item.Action.Query),
		previewText(item.Arguments),
	)
	if detail == "" {
		return Entry{Text: appendPromptContext("▸ "+name, p.lastPrompt)}, true
	}
	return Entry{Text: appendPromptContext("▸ "+name+" "+detail, p.lastPrompt)}, true
}

func (p *Parser) renderToolResult(payload json.RawMessage, fallback string) (Entry, bool) {
	var item struct {
		Type   string `json:"type"`
		CallID string `json:"call_id"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(payload, &item)
	name := firstNonEmpty(item.Type, fallback)
	detail := previewText(item.Output)
	if detail == "" {
		return Entry{Text: appendPromptContext("← "+name, p.lastPrompt)}, true
	}
	return Entry{Text: appendPromptContext("← "+name+" "+detail, p.lastPrompt)}, true
}

func appendPromptContext(label, prompt string) string {
	prompt = previewText(prompt)
	if prompt == "" {
		return label
	}
	return label + ` <- "` + prompt + `"`
}

func stripUserMessagePrefix(text string) string {
	const prefix = "## My request for Codex:"
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, prefix); idx >= 0 {
		return strings.TrimSpace(text[idx+len(prefix):])
	}
	return text
}

func comma(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	sign := ""
	if s[0] == '-' {
		sign, s = "-", s[1:]
	}
	var out []byte
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	out = append(out, s[:pre]...)
	for i := pre; i < len(s); i += 3 {
		out = append(out, ',')
		out = append(out, s[i:i+3]...)
	}
	return sign + string(out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func previewText(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	const max = 80
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max-3]) + "..."
}
