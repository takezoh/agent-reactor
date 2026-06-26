package driver

import (
	"strings"

	"github.com/takezoh/agent-reactor/client/state"
)

func enqueueSummaryJob(
	effs []state.Effect,
	inFlight bool,
	prompt string,
) ([]state.Effect, bool) {
	if inFlight || strings.TrimSpace(prompt) == "" {
		return effs, inFlight
	}
	effs = append(effs, state.EffStartJob{
		Input: SummaryCommandInput{
			Prompt: prompt,
		},
	})
	return effs, true
}

func applySummaryJobResult(summary string, inFlight bool, e state.DEvJobResult) (string, bool, bool) {
	r, ok := e.Result.(SummaryCommandResult)
	if !ok {
		return summary, inFlight, false
	}
	if e.Err != nil {
		return summary, false, true
	}
	if r.Summary != "" {
		summary = clampGraphemes(r.Summary, summaryDisplayCap)
	}
	return summary, false, true
}

// summaryDisplayCap caps the final summary length in code-points. Defense-in-
// depth: the LLM is asked for ~25 chars in formatSummaryPrompt, but models
// frequently overshoot; capping at 30 keeps the value usable in 25-grapheme
// SessionList cards (the web layer further clamps for display).
const summaryDisplayCap = 30
