package driver

import (
	"strings"
	"testing"
)

func TestFormatGenericSummaryPromptIncludesContent(t *testing.T) {
	prompt := formatGenericSummaryPrompt("", "", "", "diff --git a/foo.go b/foo.go")
	if !strings.Contains(prompt, "diff --git a/foo.go b/foo.go") {
		t.Errorf("prompt does not contain the content: %q", prompt)
	}
	if !strings.Contains(prompt, "<terminal_output>") {
		t.Errorf("prompt missing <terminal_output> tag")
	}
}

func TestFormatGenericSummaryPromptIncludesPreviousSummary(t *testing.T) {
	withPrev := formatGenericSummaryPrompt("prev summary text", "", "", "some output")
	if !strings.Contains(withPrev, "<previous_summary>") {
		t.Error("expected <previous_summary> block when prev is non-empty")
	}
	if !strings.Contains(withPrev, "prev summary text") {
		t.Error("expected previous summary text in prompt")
	}

	withoutPrev := formatGenericSummaryPrompt("", "", "", "some output")
	if strings.Contains(withoutPrev, "<previous_summary>") {
		t.Error("unexpected <previous_summary> block when prev is empty")
	}
}

func TestFormatGenericSummaryPromptClipsLargeContent(t *testing.T) {
	// Build content larger than summaryTotalCap runes
	large := strings.Repeat("x", summaryTotalCap+100)
	prompt := formatGenericSummaryPrompt("", "", "", large)
	// The clipped marker "…" must appear
	if !strings.Contains(prompt, "…") {
		t.Error("expected tailClip ellipsis for large content")
	}
}

func TestFormatGenericSummaryPromptDoesNotMentionAgent(t *testing.T) {
	prompt := formatGenericSummaryPrompt("", "", "", "htop output")
	for _, bad := range []string{"AI coding session", "user turn", "recent_turns", "coding session"} {
		if strings.Contains(prompt, bad) {
			t.Errorf("prompt contains agent-specific wording %q", bad)
		}
	}
	if !strings.Contains(prompt, "terminal session summarizer") {
		t.Error("prompt missing expected 'terminal session summarizer' wording")
	}
}

func TestFormatGenericSummaryPromptIncludesCommandAndWorkingDir(t *testing.T) {
	prompt := formatGenericSummaryPrompt("", "tig", "/workspace/agent-roost", "diff content")
	if !strings.Contains(prompt, "<command>\ntig\n</command>") {
		t.Errorf("prompt missing <command> block: %q", prompt)
	}
	if !strings.Contains(prompt, "<working_directory>\n/workspace/agent-roost\n</working_directory>") {
		t.Errorf("prompt missing <working_directory> block: %q", prompt)
	}
}

func TestFormatGenericSummaryPromptOmitsEmptyMetadata(t *testing.T) {
	prompt := formatGenericSummaryPrompt("", "", "", "some output")
	// Opening-tag-with-newline signals an actual block (the instruction
	// text may mention the tag names inline, so a bare "<command>" match
	// is not sufficient).
	if strings.Contains(prompt, "<command>\n") {
		t.Error("expected no <command> block when command is empty")
	}
	if strings.Contains(prompt, "<working_directory>\n") {
		t.Error("expected no <working_directory> block when workingDir is empty")
	}
}

func TestUserOnlyTurnsFiltersNonUserRoles(t *testing.T) {
	turns := []SummaryTurn{
		{Role: "user", Text: "u1"},
		{Role: "assistant", Text: "a1"},
		{Role: "tool", Text: "t1"},
		{Role: "user", Text: "u2"},
		{Role: "system", Text: "s1"},
		{Role: "assistant", Text: "a2"},
	}
	out := userOnlyTurns(turns, 5)
	if len(out) != 2 {
		t.Fatalf("expected 2 user turns, got %d: %+v", len(out), out)
	}
	if out[0].Text != "u1" || out[1].Text != "u2" {
		t.Errorf("expected [u1,u2] in chronological order, got %+v", out)
	}
	for _, ot := range out {
		if ot.Role != "user" {
			t.Errorf("non-user role leaked through: %q", ot.Role)
		}
	}
}

func TestUserOnlyTurnsTakesMostRecentN(t *testing.T) {
	turns := []SummaryTurn{
		{Role: "user", Text: "u1"},
		{Role: "assistant", Text: "a1"},
		{Role: "user", Text: "u2"},
		{Role: "user", Text: "u3"},
		{Role: "assistant", Text: "a2"},
		{Role: "user", Text: "u4"},
	}
	out := userOnlyTurns(turns, 2)
	if len(out) != 2 {
		t.Fatalf("expected 2 user turns, got %d: %+v", len(out), out)
	}
	if out[0].Text != "u3" || out[1].Text != "u4" {
		t.Errorf("expected most-recent [u3,u4], got %+v", out)
	}
}

func TestUserOnlyTurnsZeroOrEmpty(t *testing.T) {
	if out := userOnlyTurns(nil, 5); out != nil {
		t.Errorf("nil input should return nil, got %+v", out)
	}
	if out := userOnlyTurns([]SummaryTurn{{Role: "user", Text: "x"}}, 0); out != nil {
		t.Errorf("n=0 should return nil, got %+v", out)
	}
	if out := userOnlyTurns([]SummaryTurn{{Role: "assistant", Text: "x"}}, 3); len(out) != 0 {
		t.Errorf("no user turns should return empty, got %+v", out)
	}
}

func TestFormatSummaryPromptUsesUserOnlyDirective(t *testing.T) {
	prompt := formatSummaryPrompt("", []SummaryTurn{{Role: "user", Text: "rebuild auth flow"}})
	if !strings.Contains(prompt, "ONLY the user inputs") {
		t.Error("prompt missing the user-only constraint directive")
	}
	if !strings.Contains(prompt, "about 25 characters") {
		t.Error("prompt missing the 25-character length directive")
	}
	if !strings.Contains(prompt, "<user_inputs>") {
		t.Error("prompt missing <user_inputs> block tag")
	}
	if strings.Contains(prompt, "<recent_turns>") {
		t.Error("legacy <recent_turns> tag should be replaced")
	}
}

func TestClampGraphemes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"helloworld", 5, "hello…"},
		{"あいうえおかきくけこ", 5, "あいうえお…"},
		{"🐶🐱🐭🐹🐰🦊", 3, "🐶🐱🐭…"},
		{"x", 0, ""},
	}
	for _, c := range cases {
		got := clampGraphemes(c.in, c.n)
		if got != c.want {
			t.Errorf("clampGraphemes(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
