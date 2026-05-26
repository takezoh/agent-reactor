package scheduler

import (
	"errors"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

func validCfg() wfconfig.Config {
	return wfconfig.Config{
		Tracker: wfconfig.TrackerConfig{
			Kind:         "linear",
			APIKey:       "lin_api_secret",
			ProjectSlugs: []string{"my-project"},
		},
		Codex: wfconfig.CodexConfig{
			Command: "codex app-server",
		},
	}
}

func TestPreflightValid(t *testing.T) {
	if err := Preflight(validCfg()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestPreflightMissingKind(t *testing.T) {
	cfg := validCfg()
	cfg.Tracker.Kind = ""
	err := Preflight(cfg)
	if !errors.Is(err, ErrPreflight) {
		t.Fatalf("expected ErrPreflight, got %v", err)
	}
}

func TestPreflightUnsupportedKind(t *testing.T) {
	cfg := validCfg()
	cfg.Tracker.Kind = "github"
	err := Preflight(cfg)
	if !errors.Is(err, ErrPreflight) {
		t.Fatalf("expected ErrPreflight, got %v", err)
	}
}

func TestPreflightMissingAPIKey(t *testing.T) {
	cfg := validCfg()
	cfg.Tracker.APIKey = ""
	err := Preflight(cfg)
	if !errors.Is(err, ErrPreflight) {
		t.Fatalf("expected ErrPreflight, got %v", err)
	}
}

func TestPreflightMissingProjectSlug(t *testing.T) {
	cfg := validCfg()
	cfg.Tracker.ProjectSlugs = nil
	err := Preflight(cfg)
	if !errors.Is(err, ErrPreflight) {
		t.Fatalf("expected ErrPreflight, got %v", err)
	}
}

func TestPreflightMissingCodexCommand(t *testing.T) {
	cfg := validCfg()
	cfg.Codex.Command = ""
	err := Preflight(cfg)
	if !errors.Is(err, ErrPreflight) {
		t.Fatalf("expected ErrPreflight, got %v", err)
	}
}
