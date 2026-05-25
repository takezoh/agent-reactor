// Package tracker wraps the platform adapter with config-driven construction (SPEC §11.1/§11.4).
package tracker

import (
	"context"
	"fmt"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	ptracker "github.com/takezoh/agent-roost/platform/tracker"
	"github.com/takezoh/agent-roost/platform/tracker/linear"
)

// Tracker is the orchestrator-side config-driven tracker, exposing the three business operations.
type Tracker struct {
	adapter        ptracker.Adapter
	terminalStates []string
}

type adapterFactory func(endpoint, apiKey, projectSlug string, activeStates []string) ptracker.Adapter

func defaultFactory(ep, key, slug string, active []string) ptracker.Adapter {
	return linear.New(ep, key, slug, active)
}

// New constructs a Tracker from cfg. Returns typed sentinel errors for config failures (§11.4).
func New(cfg wfconfig.Config) (*Tracker, error) {
	return newWithFactory(cfg, defaultFactory)
}

func newWithFactory(cfg wfconfig.Config, factory adapterFactory) (*Tracker, error) {
	if cfg.Tracker.Kind != "linear" {
		return nil, fmt.Errorf("%w: %s", ptracker.ErrUnsupportedTrackerKind, cfg.Tracker.Kind)
	}
	if cfg.Tracker.APIKey == "" {
		return nil, ptracker.ErrMissingTrackerAPIKey
	}
	if cfg.Tracker.ProjectSlug == "" {
		return nil, ptracker.ErrMissingTrackerProjectSlug
	}
	return &Tracker{
		adapter:        factory(cfg.Tracker.Endpoint, cfg.Tracker.APIKey, cfg.Tracker.ProjectSlug, cfg.Tracker.ActiveStates),
		terminalStates: cfg.Tracker.TerminalStates,
	}, nil
}

// Candidates fetches issues in active states (§11.1).
func (t *Tracker) Candidates(ctx context.Context) ([]ptracker.Issue, error) {
	return t.adapter.FetchCandidateIssues(ctx)
}

// RefreshStates fetches current state for the given issue IDs (§11.1). Empty ids returns immediately.
func (t *Tracker) RefreshStates(ctx context.Context, ids []string) ([]ptracker.Issue, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return t.adapter.FetchIssueStatesByIDs(ctx, ids)
}

// TerminalIssues fetches issues in terminal states for startup cleanup (§11.1).
func (t *Tracker) TerminalIssues(ctx context.Context) ([]ptracker.Issue, error) {
	return t.adapter.FetchIssuesByStates(ctx, t.terminalStates)
}
