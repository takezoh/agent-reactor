// Package scheduler implements dispatch preflight and the scheduler loop per SPEC §6.3/§16.2.
package scheduler

import (
	"errors"
	"fmt"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

var ErrPreflight = errors.New("scheduler: preflight")

// supportedTrackerKinds is the set of tracker kinds the orchestrator can operate with.
var supportedTrackerKinds = map[string]bool{
	"linear": true,
}

// Preflight validates operational readiness from a resolved Config (SPEC §6.3).
// This is distinct from wfconfig.validate which enforces type/range invariants;
// Preflight checks runtime-observable fields (env-expanded api_key, supported kind, etc.).
func Preflight(cfg wfconfig.Config) error {
	if cfg.Tracker.Kind == "" {
		return fmt.Errorf("%w: tracker.kind is required", ErrPreflight)
	}
	if !supportedTrackerKinds[cfg.Tracker.Kind] {
		return fmt.Errorf("%w: tracker.kind %q is not supported (supported: linear)", ErrPreflight, cfg.Tracker.Kind)
	}
	if cfg.Tracker.APIKey == "" {
		return fmt.Errorf("%w: tracker.api_key is required", ErrPreflight)
	}
	if cfg.Tracker.Kind == "linear" && len(cfg.Tracker.ProjectSlugs) == 0 {
		return fmt.Errorf("%w: tracker.project_slugs is required for kind=linear", ErrPreflight)
	}
	if cfg.Codex.Command == "" {
		return fmt.Errorf("%w: codex.command is required", ErrPreflight)
	}
	return nil
}
