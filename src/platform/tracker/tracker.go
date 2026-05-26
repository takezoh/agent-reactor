package tracker

import (
	"context"
	"time"
)

// Issue is the normalized issue domain model (SPEC §4.1.1).
type Issue struct {
	ID          string
	Identifier  string
	Title       string
	Description string
	Priority    *int
	State       string
	BranchName  string
	URL         string
	Labels      []string
	BlockedBy   []Blocker
	Project     Project
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Project is the raw project metadata carried by an Issue. Content is the
// project overview markdown; the orchestrator interprets its front matter.
type Project struct {
	Name    string
	Content string
}

// Blocker represents an issue that blocks another issue (SPEC §4.1.1).
// Fields are empty string when the relation target is unavailable.
type Blocker struct {
	ID         string
	Identifier string
	State      string
}

// Adapter is the tracker adapter interface (SPEC §11.1).
type Adapter interface {
	FetchCandidateIssues(ctx context.Context) ([]Issue, error)
	FetchIssuesByStates(ctx context.Context, stateNames []string) ([]Issue, error)
	FetchIssueStatesByIDs(ctx context.Context, issueIDs []string) ([]Issue, error)
}
