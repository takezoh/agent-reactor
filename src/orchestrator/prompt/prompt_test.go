package prompt_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/takezoh/agent-roost/orchestrator/prompt"
	"github.com/takezoh/agent-roost/platform/tracker"
)

func TestRender_interpolatesIssue(t *testing.T) {
	iss := tracker.Issue{Identifier: "PROJ-1", Title: "Fix login"}
	out, err := prompt.Render(
		"Issue: {{ issue.identifier }} — {{ issue.title }}",
		prompt.Vars{Issue: iss, Attempt: 1},
	)
	assert.NoError(t, err)
	assert.Equal(t, "Issue: PROJ-1 — Fix login", out)
}

func TestRender_interpolatesAttempt(t *testing.T) {
	out, err := prompt.Render("Attempt {{ attempt }}", prompt.Vars{Attempt: 3})
	assert.NoError(t, err)
	assert.Equal(t, "Attempt 3", out)
}

func TestRender_interpolatesProject(t *testing.T) {
	out, err := prompt.Render(
		"{{ project.name }}@{{ project.branch }}: {{ project.prompt }}",
		prompt.Vars{Project: prompt.ProjectVars{Name: "Roost", Branch: "develop", Prompt: "extra"}},
	)
	assert.NoError(t, err)
	assert.Equal(t, "Roost@develop: extra", out)
}

func TestRender_emptyProjectRendersEmpty(t *testing.T) {
	out, err := prompt.Render(
		"[{{ project.name }}|{{ project.branch }}|{{ project.prompt }}]",
		prompt.Vars{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "[||]", out)
}

func TestRender_unknownVariableErrors(t *testing.T) {
	_, err := prompt.Render("{{ unknown_var }}", prompt.Vars{})
	assert.True(t, errors.Is(err, prompt.ErrTemplateRender), "want ErrTemplateRender, got %v", err)
}

func TestRender_unknownFilterErrors(t *testing.T) {
	iss := tracker.Issue{Title: "hello"}
	_, err := prompt.Render("{{ issue.title | nonexistent_filter }}", prompt.Vars{Issue: iss})
	assert.True(t, errors.Is(err, prompt.ErrTemplateRender), "want ErrTemplateRender, got %v", err)
}

func TestRender_emptyTemplateReturnsDefault(t *testing.T) {
	out, err := prompt.Render("", prompt.Vars{})
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestRender_allIssueFields(t *testing.T) {
	prio := 1
	iss := tracker.Issue{
		ID:          "id-1",
		Identifier:  "PROJ-2",
		Title:       "T",
		Description: "D",
		State:       "active",
		BranchName:  "feature/proj-2",
		URL:         "https://example.com",
		Labels:      []string{"bug"},
		Priority:    &prio,
	}
	tmpl := "{{ issue.id }}|{{ issue.identifier }}|{{ issue.title }}|{{ issue.state }}|{{ issue.branch_name }}|{{ issue.url }}|{{ issue.priority }}"
	out, err := prompt.Render(tmpl, prompt.Vars{Issue: iss})
	assert.NoError(t, err)
	assert.Equal(t, "id-1|PROJ-2|T|active|feature/proj-2|https://example.com|1", out)
}

func TestRender_labelsAndBlockers(t *testing.T) {
	iss := tracker.Issue{
		Labels:    []string{"bug", "urgent"},
		BlockedBy: []tracker.Blocker{{Identifier: "PROJ-9", State: "Done"}},
	}
	tmpl := `labels={{ issue.labels | join: "," }} blockers={% for b in issue.blocked_by %}{{ b.identifier }}:{{ b.state }}{% endfor %}`
	out, err := prompt.Render(tmpl, prompt.Vars{Issue: iss})
	assert.NoError(t, err)
	assert.Equal(t, "labels=bug,urgent blockers=PROJ-9:Done", out)
}

func TestRender_nullPriorityRendersEmpty(t *testing.T) {
	// priority is "integer or null" (§4.1.1); a null value must render empty, not error.
	out, err := prompt.Render("p={{ issue.priority }}", prompt.Vars{Issue: tracker.Issue{}})
	assert.NoError(t, err)
	assert.Equal(t, "p=", out)
}

func TestRender_attempt0FalsyOnFirstRun(t *testing.T) {
	out, err := prompt.Render("{% if attempt %}visible{% endif %}", prompt.Vars{Attempt: 0})
	assert.NoError(t, err)
	assert.Equal(t, "", out, "{% if attempt %} must be falsy on first run")

	out, err = prompt.Render("{% if attempt %}retry {{ attempt }}{% endif %}", prompt.Vars{Attempt: 0})
	assert.NoError(t, err)
	assert.Equal(t, "", out, "retry block must not render on first run")
}

func TestRender_attempt1TruthyOnRetry(t *testing.T) {
	out, err := prompt.Render("a={{ attempt }}", prompt.Vars{Attempt: 1})
	assert.NoError(t, err)
	assert.Equal(t, "a=1", out, "attempt=1 must render its value on first retry")

	out, err = prompt.Render("{% if attempt %}retry {{ attempt }}{% endif %}", prompt.Vars{Attempt: 1})
	assert.NoError(t, err)
	assert.Equal(t, "retry 1", out, "{% if attempt %} must be truthy on retry")
}
