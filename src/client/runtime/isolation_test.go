// Package runtime_test contains isolation enforcement tests that complement depguard.
//
// Enforcement split:
//   - depguard (.golangci.yml): prevents driver/, connector/, and lib/<tool>/ packages
//     from being imported by generic layers (state/, runtime/, tui/, proto/, sandbox/).
//     This is the primary, zero-false-positive gate.
//   - This file: catches cases that import-boundary checks cannot see — env variable
//     name literals (e.g. "ANTHROPIC_API_KEY"), driver command strings (e.g. "claude"),
//     and standalone connector name literals (e.g. "github") in generic layers.
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

	walkChecked(t, checkedDirs, func(t *testing.T, path string, data []byte) {
		for _, kw := range forbidden {
			if bytes.Contains(data, []byte(`"`+kw)) {
				t.Errorf(
					"%s contains tool-specific env literal %q\n"+
						"  → move to credproxy providers/<name>/, hostexec/, or lib/<tool>/ (see ARCHITECTURE.md)",
					path, kw,
				)
			}
		}
	})
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

	walkChecked(t, checkedDirs, func(t *testing.T, path string, data []byte) {
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
	})
}

// TestNoConnectorNameLiteralsInTUI guards against connector names ("github")
// appearing in tui/ routing logic. TUI must not branch on connector identity
// — see ARCHITECTURE.md "Driver/Connector isolation".
//
// connector/ itself is exempt (it is the implementation site).
// Import paths containing the connector name are also exempt.
func TestNoConnectorNameLiteralsInTUI(t *testing.T) {
	connectorNames := []string{"github"}

	srcRoot := ".."
	// Only check presentation and protocol layers; connector/ is the
	// canonical owner of connector names.
	checkedDirs := []string{
		filepath.Join(srcRoot, "tui"),
		filepath.Join(srcRoot, "proto"),
		filepath.Join(srcRoot, "state"),
		filepath.Join(srcRoot, "runtime"),
	}

	walkChecked(t, checkedDirs, func(t *testing.T, path string, data []byte) {
		for _, name := range connectorNames {
			// Match the connector name as a standalone value (not part of a URL path).
			// Skip occurrences that are clearly import paths (contain a dot after the name).
			re := regexp.MustCompile(`"` + name + `"`)
			if loc := re.Find(data); loc != nil {
				t.Errorf(
					"%s contains connector name %q as a standalone string literal: %s\n"+
						"  → connector names must stay within connector/ (see ARCHITECTURE.md)",
					path, name, bytes.TrimSpace(loc),
				)
			}
		}
	})
}

// TestNoDriverNameLiteralsMain guards the main package (coordinator.go,
// subcommand.go, main.go) against tool-specific string literals. The main
// package wires generic config values into runtime; it must not embed
// driver/connector names as string literals — use constants from driver/.
func TestNoDriverNameLiteralsMain(t *testing.T) {
	forbidden := []string{"claude", "gemini", "codex"}

	srcRoot := ".."
	// The main package lives at the src/ root — walk only *.go files there.
	walkChecked(t, []string{srcRoot}, func(t *testing.T, path string, data []byte) {
		// Only check files directly in srcRoot (not sub-packages).
		if filepath.Dir(path) != srcRoot {
			return
		}
		for _, name := range forbidden {
			re := regexp.MustCompile(`"[^"\n]*\b` + name + `\b[^"\n]*"`)
			if loc := re.Find(data); loc != nil {
				t.Errorf(
					"%s (main package) contains driver name %q as a string literal: %s\n"+
						"  → use constants from driver/ instead (see ARCHITECTURE.md)",
					path, name, bytes.TrimSpace(loc),
				)
			}
		}
	})
}

// walkChecked walks each dir recursively, calling fn for every non-test .go file.
func walkChecked(t *testing.T, dirs []string, fn func(*testing.T, string, []byte)) {
	t.Helper()
	for _, dir := range dirs {
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries silently
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("read %s: %v", path, err)
				return nil
			}
			fn(t, path, data)
			return nil
		}); err != nil {
			t.Errorf("walk %s: %v", dir, err)
		}
	}
}
