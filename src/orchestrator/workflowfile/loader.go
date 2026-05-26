// Package workflowfile loads WORKFLOW.md files per SPEC §5.1–§5.2.
package workflowfile

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Workflow holds the parsed WORKFLOW.md.
type Workflow struct {
	Config         map[string]any // front matter root object; never nil
	PromptTemplate string         // trimmed Markdown body
}

var (
	ErrMissingWorkflowFile = errors.New("workflowfile: missing WORKFLOW.md")
	ErrWorkflowParse       = errors.New("workflowfile: workflow parse error")
	ErrFrontMatterNotMap   = errors.New("workflowfile: front matter is not a map")
)

// Load reads path and parses it into a Workflow.
func Load(path string) (Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, fmt.Errorf("%w: %s: %v", ErrMissingWorkflowFile, path, err)
	}
	return Parse(data)
}

// Parse splits data into front matter and Markdown body using the WORKFLOW.md
// grammar (§5.1–§5.2). A leading "---" opens the front matter; without it the
// whole input is the prompt template. Reused to parse Linear project content.
func Parse(data []byte) (Workflow, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return Workflow{Config: map[string]any{}, PromptTemplate: strings.TrimSpace(text)}, nil
	}
	fm, body, err := splitFrontMatter(lines)
	if err != nil {
		return Workflow{}, err
	}
	return buildWorkflow(fm, body)
}

func splitFrontMatter(lines []string) (fm, body string, err error) {
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[1:i], "\n"),
				strings.Join(lines[i+1:], "\n"),
				nil
		}
	}
	return "", "", fmt.Errorf("%w: missing closing '---'", ErrWorkflowParse)
}

func buildWorkflow(fm, body string) (Workflow, error) {
	var raw any
	if err := yaml.Unmarshal([]byte(fm), &raw); err != nil {
		return Workflow{}, fmt.Errorf("%w: %v", ErrWorkflowParse, err)
	}
	if raw == nil {
		return Workflow{Config: map[string]any{}, PromptTemplate: strings.TrimSpace(body)}, nil
	}
	cfg, ok := raw.(map[string]any)
	if !ok {
		return Workflow{}, fmt.Errorf("%w", ErrFrontMatterNotMap)
	}
	return Workflow{Config: cfg, PromptTemplate: strings.TrimSpace(body)}, nil
}
