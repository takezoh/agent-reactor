package agent

import (
	"log/slog"
	"strings"

	"github.com/takezoh/agent-roost/orchestrator/workflowfile"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// projectMeta is the per-project configuration resolved from a Linear project's
// content. The content reuses the WORKFLOW.md grammar: an optional YAML front
// matter (currently only "branch") followed by an additional prompt body.
type projectMeta struct {
	Name   string
	Branch string
	Prompt string
}

// resolveProjectMeta parses p.Content as front matter + body. Front matter and
// body are both optional: empty content yields a zero meta; content without
// front matter is treated entirely as the prompt body with an empty branch.
//
// Project content is author-written markdown where a leading "---" is often a
// thematic break rather than a front-matter fence. When parsing fails, the
// whole content is kept as the prompt body rather than discarded, so the
// agent never silently loses its project instructions.
func resolveProjectMeta(p tracker.Project) projectMeta {
	meta := projectMeta{Name: p.Name}
	if p.Content == "" {
		return meta
	}
	wf, err := workflowfile.Parse([]byte(p.Content))
	if err != nil {
		slog.Warn("agent: project content not valid front matter; using whole content as prompt",
			"project", p.Name, "err", err)
		meta.Prompt = strings.TrimSpace(p.Content)
		return meta
	}
	if b, ok := wf.Config["branch"]; ok {
		if s, isStr := b.(string); isStr {
			meta.Branch = s
		} else {
			slog.Warn("agent: project branch is not a string; ignoring", "project", p.Name, "branch", b)
		}
	}
	meta.Prompt = wf.PromptTemplate
	return meta
}
