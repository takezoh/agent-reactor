package workflowfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_FrontMatterAndBody(t *testing.T) {
	path := writeFile(t, "---\nfoo: bar\n---\n# title\n\nbody")
	wf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Config["foo"] != "bar" {
		t.Errorf("Config[foo] = %v, want bar", wf.Config["foo"])
	}
	if wf.PromptTemplate != "# title\n\nbody" {
		t.Errorf("PromptTemplate = %q, want %q", wf.PromptTemplate, "# title\n\nbody")
	}
}

func TestLoad_NoFrontMatter(t *testing.T) {
	path := writeFile(t, "# title\nbody")
	wf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Config) != 0 {
		t.Errorf("Config = %v, want empty map", wf.Config)
	}
	if wf.Config == nil {
		t.Error("Config must not be nil")
	}
	if wf.PromptTemplate != "# title\nbody" {
		t.Errorf("PromptTemplate = %q, want %q", wf.PromptTemplate, "# title\nbody")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "WORKFLOW.md"))
	if !errors.Is(err, ErrMissingWorkflowFile) {
		t.Errorf("err = %v, want ErrMissingWorkflowFile", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeFile(t, "---\nkey: [unclosed\n---\n")
	_, err := Load(path)
	if !errors.Is(err, ErrWorkflowParse) {
		t.Errorf("err = %v, want ErrWorkflowParse", err)
	}
}

func TestLoad_FrontMatterNotMap(t *testing.T) {
	path := writeFile(t, "---\n- a\n- b\n---\n")
	_, err := Load(path)
	if !errors.Is(err, ErrFrontMatterNotMap) {
		t.Errorf("err = %v, want ErrFrontMatterNotMap", err)
	}
}

func TestLoad_EmptyFrontMatter(t *testing.T) {
	path := writeFile(t, "---\n---\nbody")
	wf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wf.Config) != 0 {
		t.Errorf("Config = %v, want empty map", wf.Config)
	}
	if wf.PromptTemplate != "body" {
		t.Errorf("PromptTemplate = %q, want %q", wf.PromptTemplate, "body")
	}
}

func TestLoad_BodyTrimmed(t *testing.T) {
	path := writeFile(t, "---\nk: v\n---\n\n\n  body  \n\n")
	wf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.PromptTemplate != "body" {
		t.Errorf("PromptTemplate = %q, want %q", wf.PromptTemplate, "body")
	}
}

func TestLoad_UnclosedFrontMatter(t *testing.T) {
	path := writeFile(t, "---\nfoo: bar\n")
	_, err := Load(path)
	if !errors.Is(err, ErrWorkflowParse) {
		t.Errorf("err = %v, want ErrWorkflowParse", err)
	}
}
