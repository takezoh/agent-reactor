package driver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiTranscriptParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := `{"sessionId":"sess-1","projectHash":"hash-1","startTime":"2026-04-12T00:00:00Z","lastUpdated":"2026-04-12T00:00:00Z"}
{"id":"u1","timestamp":"2026-04-12T00:00:01Z","type":"user","content":[{"text":"inspect repo"}]}
{"id":"g1","timestamp":"2026-04-12T00:00:02Z","type":"gemini","content":[{"text":"checking files"}],"toolCalls":[{"id":"tool-1","name":"read_file","displayName":"ReadFile","status":"running"}]}
{"$set":{"summary":"Inspecting repository structure"}}
{"id":"g1","timestamp":"2026-04-12T00:00:03Z","type":"gemini","content":[{"text":"checked files"}],"toolCalls":[{"id":"tool-1","name":"read_file","displayName":"ReadFile","status":"success"}]}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	run := newGeminiTranscriptParse()
	got, err := run(context.Background(), GeminiTranscriptParseInput{Path: path})
	if err != nil {
		t.Fatalf("parse transcript: %v", err)
	}
	if got.Title != "Inspecting repository structure" {
		t.Fatalf("Title = %q", got.Title)
	}
	if got.LastPrompt != "inspect repo" {
		t.Fatalf("LastPrompt = %q", got.LastPrompt)
	}
	if got.LastAssistantMessage != "checked files" {
		t.Fatalf("LastAssistantMessage = %q", got.LastAssistantMessage)
	}
	if got.CurrentTool != "ReadFile" {
		t.Fatalf("CurrentTool = %q", got.CurrentTool)
	}
	if len(got.RecentTurns) != 2 {
		t.Fatalf("RecentTurns = %d, want 2", len(got.RecentTurns))
	}
}
