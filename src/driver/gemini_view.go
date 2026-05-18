package driver

import (
	"github.com/takezoh/agent-roost/state"
	"github.com/takezoh/fishpath-go"
)

func (d GeminiDriver) view(gs GeminiState) state.View {
	tags := CommonTags(gs.CommonState)

	var tabs []state.LogTab
	if gs.TranscriptPath != "" {
		tabs = append(tabs, state.LogTab{
			Label: "TRANSCRIPT",
			Path:  gs.TranscriptPath,
			Kind:  state.TabKindText,
		})
	}
	if tab := EventLogTab(gs.CommonState, d.eventLogDir); tab != nil {
		tabs = append(tabs, *tab)
	}

	return state.View{
		Card: state.Card{
			Subtitle:    firstNonEmpty(gs.Summary, gs.LastPrompt, gs.LastAssistantMessage),
			Tags:        tags,
			Indicators:  geminiIndicators(gs),
			BorderTitle: GeminiCommandTag(),
			BorderBadge: fishpath.Shorten(gs.StartDir, ""),
		},
		DisplayName:     GeminiDriverName,
		LogTabs:         tabs,
		InfoExtras:      geminiInfoExtras(gs),
		StatusLine:      gs.StatusLine,
		Status:          gs.Status,
		StatusChangedAt: gs.StatusChangedAt,
	}
}

func geminiIndicators(gs GeminiState) []string {
	if gs.CurrentTool == "" {
		return nil
	}
	return []string{"▸ " + gs.CurrentTool}
}

func geminiInfoExtras(gs GeminiState) []state.InfoLine {
	var lines []state.InfoLine
	add := func(label, value string) {
		if value != "" {
			lines = append(lines, state.InfoLine{Label: label, Value: value})
		}
	}
	add("Gemini Session", gs.GeminiSessionID)
	add("Working Dir", gs.StartDir)
	add("Worktree Name", gs.WorktreeName)
	add("Current Tool", gs.CurrentTool)
	if gs.BranchIsWorktree {
		add("Parent Branch", gs.BranchParentBranch)
	}
	add("Transcript", gs.TranscriptPath)
	add("Summary", gs.Summary)
	add("Status Line", gs.StatusLine)
	add("Last Prompt", previewText(gs.LastPrompt))
	add("Last Assistant", previewText(gs.LastAssistantMessage))
	add("Last Hook", gs.LastHookEvent)
	return lines
}
