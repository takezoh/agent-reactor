package scheduler

import (
	"context"

	"github.com/takezoh/agent-roost/platform/tracker"
)

// CandidateSource abstracts the tracker poll for testability.
type CandidateSource interface {
	Candidates(ctx context.Context) ([]tracker.Issue, error)
}

// IssueRevalidator re-fetches current issue state before spawn (SPEC §16.4).
// Satisfied by *orchestrator/tracker.Tracker via its RefreshStates method.
type IssueRevalidator interface {
	RefreshStates(ctx context.Context, ids []string) ([]tracker.Issue, error)
}

// SpawnResult is what a successful spawn yields: the pure session identity (stored in
// State) and the live Worker handle (held by the shell's id→handle map, never in State).
type SpawnResult struct {
	Session LiveSession
	Worker  Worker
}

// SpawnFunc launches an agent worker for the given issue (injected by the agent runner).
type SpawnFunc func(ctx context.Context, issue tracker.Issue, attempt int) (SpawnResult, error)

// revalidateIssue re-fetches a single issue's current state from the tracker (SPEC §16.4).
// Returns (nil, nil) when the issue is not found; (nil, err) on fetch failure. Used by the
// shell when interpreting EffRevalidate.
func revalidateIssue(ctx context.Context, rv IssueRevalidator, issueID string) (*tracker.Issue, error) {
	issues, err := rv.RefreshStates(ctx, []string{issueID})
	if err != nil {
		return nil, err
	}
	for i := range issues {
		if issues[i].ID == issueID {
			return &issues[i], nil
		}
	}
	return nil, nil
}
