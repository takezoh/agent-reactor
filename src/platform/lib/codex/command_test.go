package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunHelp(t *testing.T) {
	for _, a := range []string{"help", "-h", "--help"} {
		if err := Run([]string{a}); err != nil {
			t.Errorf("Run(%q) err: %v", a, err)
		}
	}
}

func TestRunMissingSubcommand(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Error("expected error")
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	if err := Run([]string{"nope"}); err == nil {
		t.Error("expected error")
	}
}

func TestGetConfigDirFromEnv(t *testing.T) {
	t.Setenv("CODEX_CONFIG_DIR", "/custom/codex")
	got, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/codex" {
		t.Errorf("got %q", got)
	}
}

func TestGetConfigDirDefault(t *testing.T) {
	t.Setenv("CODEX_CONFIG_DIR", "")
	got, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestRunSetupCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_CONFIG_DIR", dir)
	if err := RunSetup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mcp.json")); err != nil {
		t.Errorf("mcp.json not created: %v", err)
	}
	// Second call: idempotent
	if err := RunSetup(); err != nil {
		t.Fatal(err)
	}
}
