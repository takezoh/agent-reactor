package runtime_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoToolSpecificEnvLiterals guards against tool-specific environment variable
// names (AWS_*, ANTHROPIC_*, GOOGLE_*, OPENAI_*, etc.) appearing as string literals
// in generic layers. These names must live exclusively in the credproxy module's
// providers/<name>/ packages, the local hostexec/ package, or lib/<tool>/ —
// see ARCHITECTURE.md "Driver/Connector isolation".
//
// golangci-lint forbidigo cannot detect string literals (only call expressions),
// so this test acts as the static enforcement gate.
func TestNoToolSpecificEnvLiterals(t *testing.T) {
	forbidden := []string{
		"AWS_CONTAINER_",
		"AWS_ACCESS_KEY",
		"AWS_SECRET_ACCESS",
		"AWS_SESSION_TOKEN",
		"AWS_PROFILE",
		"AWS_REGION",
		"ANTHROPIC_API_KEY",
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_BASE_URL",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"OPENAI_API_KEY",
		"GITHUB_TOKEN",
		"GH_TOKEN",
		"CLAUDE_API_KEY",
		"GEMINI_API_KEY",
		"CLOUDSDK_ACTIVE_CONFIG_NAME",
	}

	// Packages whose non-test .go files must contain no tool-specific env literals.
	srcRoot := ".."
	checkedDirs := []string{
		filepath.Join(srcRoot, "runtime"),
		filepath.Join(srcRoot, "sandbox"),
		filepath.Join(srcRoot, "state"),
		filepath.Join(srcRoot, "tui"),
		filepath.Join(srcRoot, "proto"),
		filepath.Join(srcRoot, "connector"),
	}

	for _, dir := range checkedDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			// Directory may not exist for all builds; skip gracefully.
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("read %s: %v", path, err)
				continue
			}
			for _, kw := range forbidden {
				if bytes.Contains(data, []byte(`"`+kw)) {
					t.Errorf(
						"%s contains tool-specific env literal %q\n"+
							"  → move to credproxy providers/<name>/, hostexec/, or lib/<tool>/ (see ARCHITECTURE.md)",
						path, kw,
					)
				}
			}
		}
	}
}

// TestNoDriverNameLiterals guards against driver/tool names ("claude", "gemini",
// "codex") appearing as routing keys or string literals in generic layers.
// Driver names must stay within driver/ and lib/<tool>/ — see ARCHITECTURE.md.
//
// Both exact matches ("claude") and embedded occurrences (" claude setup",
// "run claude -") are detected via word-boundary regexp on string literals.
func TestNoDriverNameLiterals(t *testing.T) {
	// Word-boundary patterns that must not appear inside any double-quoted string
	// in generic layer source files.
	forbidden := []string{"claude", "gemini", "codex"}

	srcRoot := ".."
	checkedDirs := []string{
		filepath.Join(srcRoot, "runtime"),
		filepath.Join(srcRoot, "sandbox"),
		filepath.Join(srcRoot, "state"),
		filepath.Join(srcRoot, "tui"),
		filepath.Join(srcRoot, "proto"),
		filepath.Join(srcRoot, "connector"),
	}

	for _, dir := range checkedDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("read %s: %v", path, err)
				continue
			}
			for _, name := range forbidden {
				// Match driver name as a whole word inside any double-quoted string literal.
				re := regexp.MustCompile(`"[^"\n]*\b` + name + `\b[^"\n]*"`)
				if loc := re.Find(data); loc != nil {
					t.Errorf(
						"%s contains driver name %q inside a string literal: %s\n"+
							"  → driver names must stay within driver/ or lib/<tool>/ (see ARCHITECTURE.md)",
						path, name, bytes.TrimSpace(loc),
					)
				}
			}
		}
	}
}
