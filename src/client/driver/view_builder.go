package driver

import (
	"path/filepath"
	"strings"

	"github.com/takezoh/agent-reactor/client/state"
)

// CommonTags returns the shared UI tags (e.g. Git branch) for the driver.
func CommonTags(c CommonState) []state.Tag {
	var tags []state.Tag
	if t := BranchTag(c.BranchTag, c.BranchBG, c.BranchFG, c.BranchParentBranch); t.Text != "" {
		tags = append(tags, t)
	}
	return tags
}

// EventLogTab returns the "EVENTS" log tab if the session and log directory are known.
func EventLogTab(c CommonState, eventLogDir string) *state.LogTab {
	if c.RoostSessionID != "" && eventLogDir != "" {
		return &state.LogTab{
			Label: "EVENTS",
			Path:  filepath.Join(eventLogDir, c.RoostSessionID+".log"),
			Kind:  state.TabKindText,
		}
	}
	return nil
}

// firstNonEmpty returns the first string in candidates that is not empty.
func firstNonEmpty(candidates ...string) string {
	for _, s := range candidates {
		if s != "" {
			return s
		}
	}
	return ""
}

// resolveCardTitleSubtitle picks Title from aiTitle→summary→"" and Subtitle
// from summary→lastPrompt→"". LastPrompt is never a Title candidate (only an
// AI title or a user-prompt summary qualifies). Subtitle is intentionally
// NOT deduped against Title here — non-rendering consumers may read
// Card.Subtitle as the human-context source and need it populated even when
// Summary was hoisted into Title. The web SessionList row and the TUI session
// card render the dedup so the same string never appears twice on screen.
func resolveCardTitleSubtitle(aiTitle, summary, lastPrompt string) (string, string) {
	// Multi-line summaries (legacy persisted, pre-single-line constraint)
	// are NOT Title candidates. The Title row has no per-line splitter, and
	// promoting only the first line would leave the same line in the
	// Subtitle row's multi-line splitter (tui/view.go sessionCardLines /
	// SessionList.subtitleText), defeating dedup. For those sessions Title
	// stays empty (placeholder "New Session" in the web client) and the
	// multi-line Subtitle row keeps the legacy rendering path.
	summaryAsTitle := summary
	if strings.ContainsRune(summaryAsTitle, '\n') {
		summaryAsTitle = ""
	}
	return firstNonEmpty(aiTitle, summaryAsTitle), firstNonEmpty(summary, lastPrompt)
}

// previewText truncates long text for display in info lines.
func previewText(text string) string {
	const max = 80
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
