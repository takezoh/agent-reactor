package driver

import (
	"strings"
)

const (
	summaryEntryTextCap = 1500
	summaryTotalCap     = 12000
)

func appendHookPromptTurn(turns []SummaryTurn, hookPrompt string) []SummaryTurn {
	if hookPrompt == "" {
		return turns
	}
	if n := len(turns); n > 0 && turns[n-1].Role == "user" && turns[n-1].Text == hookPrompt {
		return turns
	}
	return append(turns, SummaryTurn{Role: "user", Text: hookPrompt})
}

// userOnlyTurns returns the most recent n turns whose Role == "user", in
// chronological order. Assistant, tool, and system turns are filtered out
// entirely — the summarizer must see ONLY user inputs (the session card's
// Subtitle is meant to reflect user intent, not model output).
func userOnlyTurns(turns []SummaryTurn, n int) []SummaryTurn {
	if n <= 0 || len(turns) == 0 {
		return nil
	}
	out := make([]SummaryTurn, 0, n)
	for i := len(turns) - 1; i >= 0 && len(out) < n; i-- {
		if turns[i].Role == "user" {
			out = append(out, turns[i])
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func formatSummaryPrompt(prev string, turns []SummaryTurn) string {
	var b strings.Builder
	b.WriteString("You are a session summarizer. From the user inputs and previous summary below, ")
	b.WriteString("summarize the work or goal the user is driving in this AI coding session ")
	b.WriteString("into a single concise line (about 25 characters). ")
	b.WriteString("Use ONLY the user inputs below; never summarize assistant outputs, tool results, ")
	b.WriteString("or any non-user content. ")
	b.WriteString("Return only the body text, no headings, decoration, preamble, or quotes.\n\n")
	if prev != "" {
		b.WriteString("<previous_summary>\n")
		b.WriteString(prev)
		b.WriteString("\n</previous_summary>\n\n")
	}
	b.WriteString("<user_inputs>\n")
	b.WriteString(renderRecentTurns(turns))
	b.WriteString("</user_inputs>\n")
	return b.String()
}

// clampGraphemes truncates s to at most n Unicode code points, appending "…"
// when truncation actually occurred. Used as a defense-in-depth backstop in
// case the LLM ignores the length instruction in formatSummaryPrompt.
func clampGraphemes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func renderRecentTurns(turns []SummaryTurn) string {
	clipped := make([]SummaryTurn, len(turns))
	for i, t := range turns {
		clipped[i] = SummaryTurn{Role: t.Role, Text: tailClip(t.Text, summaryEntryTextCap)}
	}
	var blocks []string
	prevRole := ""
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		blocks = append(blocks, cur.String())
		cur.Reset()
	}
	for _, t := range clipped {
		if t.Role != prevRole {
			flush()
			cur.WriteString("[")
			cur.WriteString(t.Role)
			cur.WriteString("]\n")
			prevRole = t.Role
		} else {
			cur.WriteString("\n")
		}
		cur.WriteString(t.Text)
		cur.WriteString("\n")
	}
	flush()
	body := strings.Join(blocks, "\n")
	for len(body) > summaryTotalCap && len(blocks) > 1 {
		blocks = blocks[1:]
		body = strings.Join(blocks, "\n")
	}
	return body
}

func tailClip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return "…" + string(r[len(r)-max:])
}

// formatGenericSummaryPrompt builds a summary prompt for non-agent terminal
// sessions (generic shells, git diff, tig, build logs, etc.). Unlike the
// agent-oriented formatSummaryPrompt which models the session as a
// user/assistant conversation, this prompt treats the capture as raw
// terminal screen contents, annotated with the session's command and
// working directory so the LLM can ground its interpretation.
func formatGenericSummaryPrompt(prev, command, workingDir, content string) string {
	var b strings.Builder
	b.WriteString("You are a terminal session summarizer. ")
	b.WriteString("Describe in a single concise line (roughly 30 characters) what is being worked on ")
	b.WriteString("in this terminal session. ")
	b.WriteString("Ground your description in all three signals: the <command> tag (what program is running), ")
	b.WriteString("the <working_directory> tag (where it is running), ")
	b.WriteString("and the <terminal_output> tag (what is currently visible on screen). ")
	b.WriteString("Combine them into a task-oriented summary of the activity in progress — ")
	b.WriteString("for example \"reviewing git log in the agent-reactor repo\" or \"running make build, tests in progress\" — ")
	b.WriteString("not a verbatim quote of the screen. ")
	b.WriteString("Return only the body text, no headings, decoration, preamble, or quotes.\n\n")
	if prev != "" {
		b.WriteString("<previous_summary>\n")
		b.WriteString(prev)
		b.WriteString("\n</previous_summary>\n\n")
	}
	if command != "" {
		b.WriteString("<command>\n")
		b.WriteString(command)
		b.WriteString("\n</command>\n\n")
	}
	if workingDir != "" {
		b.WriteString("<working_directory>\n")
		b.WriteString(workingDir)
		b.WriteString("\n</working_directory>\n\n")
	}
	clipped := tailClip(strings.TrimSpace(content), summaryTotalCap)
	b.WriteString("<terminal_output>\n")
	b.WriteString(clipped)
	b.WriteString("\n</terminal_output>\n")
	return b.String()
}
