package agent

import (
	"testing"

	"github.com/takezoh/agent-roost/platform/tracker"
)

func TestResolveProjectMeta_FrontMatterAndBody(t *testing.T) {
	meta := resolveProjectMeta(tracker.Project{
		Name:    "Roost",
		Content: "---\nbranch: develop\n---\nFollow the project guidelines.",
	})
	if meta.Name != "Roost" {
		t.Errorf("Name = %q, want Roost", meta.Name)
	}
	if meta.Branch != "develop" {
		t.Errorf("Branch = %q, want develop", meta.Branch)
	}
	if meta.Prompt != "Follow the project guidelines." {
		t.Errorf("Prompt = %q", meta.Prompt)
	}
}

func TestResolveProjectMeta_NoFrontMatter(t *testing.T) {
	meta := resolveProjectMeta(tracker.Project{
		Name:    "Roost",
		Content: "Just an additional prompt.",
	})
	if meta.Branch != "" {
		t.Errorf("Branch = %q, want empty", meta.Branch)
	}
	if meta.Prompt != "Just an additional prompt." {
		t.Errorf("Prompt = %q", meta.Prompt)
	}
}

func TestResolveProjectMeta_LeadingRuleNotFrontMatter(t *testing.T) {
	// Content that opens with a markdown horizontal rule is not valid front
	// matter; the whole content must survive as the prompt, not be dropped.
	content := "---\nProject overview\n---\nDo the work."
	meta := resolveProjectMeta(tracker.Project{Name: "Roost", Content: content})
	if meta.Branch != "" {
		t.Errorf("Branch = %q, want empty", meta.Branch)
	}
	if meta.Prompt != content {
		t.Errorf("Prompt = %q, want whole content %q", meta.Prompt, content)
	}
}

func TestResolveProjectMeta_NonStringBranchIgnored(t *testing.T) {
	meta := resolveProjectMeta(tracker.Project{
		Name:    "Roost",
		Content: "---\nbranch: 2024\n---\nbody",
	})
	if meta.Branch != "" {
		t.Errorf("Branch = %q, want empty for non-string branch", meta.Branch)
	}
	if meta.Prompt != "body" {
		t.Errorf("Prompt = %q, want body", meta.Prompt)
	}
}

func TestResolveProjectMeta_EmptyContent(t *testing.T) {
	meta := resolveProjectMeta(tracker.Project{Name: "Roost"})
	if meta.Branch != "" || meta.Prompt != "" {
		t.Errorf("want empty branch/prompt, got branch=%q prompt=%q", meta.Branch, meta.Prompt)
	}
	if meta.Name != "Roost" {
		t.Errorf("Name = %q, want Roost", meta.Name)
	}
}
