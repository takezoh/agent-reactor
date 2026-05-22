// Package prompt renders WORKFLOW.md prompt templates per SPEC §5.4 / §12.
package prompt

import (
	"errors"
	"fmt"
	"sync"

	"github.com/osteele/liquid"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const defaultPrompt = "You are working on an issue from Linear."

// ErrTemplateRender is returned when a template contains unknown variables or filters.
var ErrTemplateRender = errors.New("prompt: template render error")

// Vars holds the variables available in a WORKFLOW.md prompt template.
type Vars struct {
	Issue   tracker.Issue
	Attempt int
}

var (
	engineOnce sync.Once
	eng        *liquid.Engine
)

func engine() *liquid.Engine {
	engineOnce.Do(func() {
		eng = liquid.NewEngine()
		eng.StrictVariables()
	})
	return eng
}

// Render executes tmpl with vars. An empty template returns the default prompt.
// Unknown variables or filters return ErrTemplateRender.
func Render(tmpl string, vars Vars) (string, error) {
	if tmpl == "" {
		return defaultPrompt, nil
	}
	// attempt=0 represents the first run (SPEC §4.1.5: attempt is null on first run).
	// Pass false so {% if attempt %} is falsy on the first run (SPEC §5.4).
	// Liquid: only nil and false are falsy; "" and 0 are truthy. StrictVariables
	// errors on nil (treated as undefined), so false is used as the null sentinel.
	// Templates must guard attempt-specific content with {% if attempt %}.
	var attemptVal any = false
	if vars.Attempt > 0 {
		attemptVal = vars.Attempt
	}
	bindings := liquid.Bindings{
		"issue":   toIssueMap(vars.Issue),
		"attempt": attemptVal,
	}
	out, err := engine().ParseAndRenderString(tmpl, bindings)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateRender, err)
	}
	return out, nil
}

func toIssueMap(iss tracker.Issue) map[string]any {
	// Every SPEC §4.1.1 field is always present as a key so that referencing a
	// defined-but-null field renders empty rather than tripping StrictVariables;
	// only genuinely unknown variables must fail (§5.4). liquid's StrictVariables
	// treats a nil value as undefined, so null priority is represented as "" —
	// matching how the string-or-null fields (description/url) render when empty.
	var priority any = ""
	if iss.Priority != nil {
		priority = *iss.Priority
	}
	return map[string]any{
		"id":          iss.ID,
		"identifier":  iss.Identifier,
		"title":       iss.Title,
		"description": iss.Description,
		"priority":    priority,
		"state":       iss.State,
		"branch_name": iss.BranchName,
		"url":         iss.URL,
		"labels":      iss.Labels,
		"blocked_by":  toBlockerList(iss.BlockedBy),
		"created_at":  iss.CreatedAt,
		"updated_at":  iss.UpdatedAt,
	}
}

func toBlockerList(blockers []tracker.Blocker) []map[string]any {
	out := make([]map[string]any, len(blockers))
	for i, b := range blockers {
		out[i] = map[string]any{
			"id":         b.ID,
			"identifier": b.Identifier,
			"state":      b.State,
		}
	}
	return out
}
