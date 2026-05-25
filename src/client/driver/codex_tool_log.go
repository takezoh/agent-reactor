package driver

import (
	"time"

	"github.com/takezoh/agent-roost/client/state"
)

func (d CodexDriver) emitToolLog(cs CodexState, ev codexToolEvent, now time.Time, kind string, effs []state.Effect) (CodexState, []state.Effect) {
	var (
		durationMs int64
		toolInput  map[string]any
	)

	if ev.ToolUseID == "" {
		toolInput = ev.ToolInput
	} else if entry, ok := cs.PendingTools[ev.ToolUseID]; ok {
		delete(cs.PendingTools, ev.ToolUseID)
		if !entry.StartedAt.IsZero() && !now.IsZero() {
			durationMs = now.Sub(entry.StartedAt).Milliseconds()
		}
		toolInput = entry.Input
	} else {
		if kind == "auto" {
			kind = "orphan"
		}
		toolInput = ev.ToolInput
	}

	project := cs.Project
	if project == "" {
		project = cs.StartDir
	}
	slug := resolveProjectSlug(project)
	if slug == "" || ev.ToolName == "" {
		return cs, effs
	}

	line := buildToolLogLine(toolLogEntry{
		TS:             now,
		RoostSessionID: cs.RoostSessionID,
		ToolUseID:      ev.ToolUseID,
		ToolName:       ev.ToolName,
		Kind:           kind,
		DurationMs:     durationMs,
		ToolInput:      summariseToolInput(ev.ToolName, toolInput),
		Error:          ev.Error,
	})
	effs = append(effs, state.EffToolLogAppend{
		Namespace: CodexDriverName,
		Project:   slug,
		Line:      line,
	})
	return cs, effs
}
