package linear_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/platform/tracker/linear"
)

// TestSPEC_17_8_RealLinearFetchCandidates is the §17.8 Real Integration Profile.
// It runs only when LINEAR_API_KEY and LINEAR_PROJECT_SLUG are set; otherwise it skips.
//
// Required env vars:
//
//	LINEAR_API_KEY          — Linear personal API key
//	LINEAR_PROJECT_SLUG     — slugId of the target project (e.g. "my-project")
//
// Optional env vars:
//
//	LINEAR_TRACKER_ENDPOINT — override API endpoint (default: https://api.linear.app/graphql)
//	LINEAR_ACTIVE_STATES    — comma-separated active state names (default: "Todo,In Progress")
//
// All three Adapter operations (FetchCandidateIssues / FetchIssuesByStates /
// FetchIssueStatesByIDs) are exercised read-only in a single test to minimise API calls.
func TestSPEC_17_8_RealLinearFetchCandidates(t *testing.T) {
	apiKey := os.Getenv("LINEAR_API_KEY")
	if apiKey == "" {
		t.Skip("LINEAR_API_KEY not set; skipping §17.8 real integration profile")
	}
	projectSlug := os.Getenv("LINEAR_PROJECT_SLUG")
	if projectSlug == "" {
		t.Skip("LINEAR_PROJECT_SLUG not set; skipping §17.8 real integration profile")
	}

	endpoint := os.Getenv("LINEAR_TRACKER_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.linear.app/graphql"
	}

	activeStatesEnv := os.Getenv("LINEAR_ACTIVE_STATES")
	var activeStates []string
	if activeStatesEnv != "" {
		for _, s := range strings.Split(activeStatesEnv, ",") {
			if s := strings.TrimSpace(s); s != "" {
				activeStates = append(activeStates, s)
			}
		}
	}
	if len(activeStates) == 0 {
		activeStates = []string{"Todo", "In Progress"}
	}

	c := linear.New(endpoint, apiKey, projectSlug, activeStates)
	ctx := context.Background()

	// Operation 1: fetch_candidate_issues — reads issues in active states.
	candidates, err := c.FetchCandidateIssues(ctx)
	if err != nil {
		t.Fatalf("FetchCandidateIssues: %v", err)
	}
	t.Logf("FetchCandidateIssues: %d issues returned", len(candidates))

	// Operation 2: fetch_issues_by_states — reads issues in a terminal state.
	terminated, err := c.FetchIssuesByStates(ctx, []string{"Done"})
	if err != nil {
		t.Fatalf("FetchIssuesByStates([Done]): %v", err)
	}
	t.Logf("FetchIssuesByStates([Done]): %d issues returned", len(terminated))

	// Operation 3: fetch_issue_states_by_ids — refreshes the first candidate by ID.
	// If there are no candidates, skip rather than fail (empty projects are valid).
	if len(candidates) == 0 {
		t.Skip("no candidate issues in the project; skipping FetchIssueStatesByIDs sub-check")
	}
	firstID := candidates[0].ID
	refreshed, err := c.FetchIssueStatesByIDs(ctx, []string{firstID})
	if err != nil {
		t.Fatalf("FetchIssueStatesByIDs([%s]): %v", firstID, err)
	}
	if len(refreshed) == 0 {
		t.Errorf("FetchIssueStatesByIDs([%s]): want at least 1 result, got 0", firstID)
	}
	t.Logf("FetchIssueStatesByIDs: %d issues returned", len(refreshed))
}
