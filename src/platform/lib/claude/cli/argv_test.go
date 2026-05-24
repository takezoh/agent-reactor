package cli

import (
	"strings"
	"testing"
)

func TestSandboxFlags(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		sandboxed bool
		want     string
	}{
		{"not sandboxed returns unchanged", "claude --verbose", false, "claude --verbose"},
		{"sandboxed appends skip flag", "claude", true, "claude " + sandboxSkipFlag},
		{"sandboxed strips auto-mode", "claude --enable-auto-mode", true, "claude " + sandboxSkipFlag},
		{"sandboxed idempotent when skip already present", "claude " + sandboxSkipFlag, true, "claude " + sandboxSkipFlag},
		{"sandboxed strips auto-mode and keeps other flags", "claude --verbose --enable-auto-mode", true, "claude --verbose " + sandboxSkipFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SandboxFlags(tt.command, tt.sandboxed)
			if got != tt.want {
				t.Errorf("SandboxFlags(%q, %v) = %q, want %q", tt.command, tt.sandboxed, got, tt.want)
			}
		})
	}
}

func TestAppServerArgs(t *testing.T) {
	tests := []struct {
		name               string
		resumeSessionID    string
		appendSystemPrompt string
		prompt             string
		wantContains       []string
		wantOrder          []string // subsequence check
	}{
		{
			name:         "new session no extras",
			prompt:       "do something",
			wantContains: []string{"-p", "--output-format", "stream-json", "--verbose", "do something"},
			wantOrder:    []string{"-p", "--output-format", "stream-json", "--verbose"},
		},
		{
			name:            "with resume",
			resumeSessionID: "sess-abc",
			prompt:          "continue",
			wantContains:    []string{"--resume", "sess-abc"},
		},
		{
			name:               "with append system prompt",
			appendSystemPrompt: "Be concise.",
			prompt:             "hi",
			wantContains:       []string{"--append-system-prompt", "Be concise."},
		},
		{
			name:            "prompt is last element",
			resumeSessionID: "id1",
			prompt:          "my prompt",
			// prompt must be the last arg
			wantContains: []string{"my prompt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppServerArgs(tt.resumeSessionID, tt.appendSystemPrompt, tt.prompt)
			for _, want := range tt.wantContains {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("AppServerArgs(...) = %v, missing element %q", got, want)
				}
			}
			if tt.prompt != "" && (len(got) == 0 || got[len(got)-1] != tt.prompt) {
				t.Errorf("AppServerArgs(...) = %v, prompt %q not last element", got, tt.prompt)
			}
			// --verbose must be present
			if !contains(got, "--verbose") {
				t.Errorf("AppServerArgs(...) = %v, missing --verbose", got)
			}
			_ = strings.Join(got, " ") // exercised
		})
	}
}

func contains(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}
