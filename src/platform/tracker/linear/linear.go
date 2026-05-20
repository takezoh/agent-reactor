package linear

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"context"

	"github.com/takezoh/agent-roost/platform/tracker"
)

var _ tracker.Adapter = (*Client)(nil)

// Client is a Linear GraphQL adapter (SPEC §11.2).
// active states are connection-level config per SPEC §11.1.
type Client struct {
	endpoint     string
	apiKey       string
	projectSlug  string
	activeStates []string
	http         *http.Client
}

// New creates a Linear client. activeStates is used by FetchCandidateIssues.
// terminal states are passed per-call to FetchIssuesByStates by the caller.
func New(endpoint, apiKey, projectSlug string, activeStates []string) *Client {
	return &Client{
		endpoint:     endpoint,
		apiKey:       apiKey,
		projectSlug:  projectSlug,
		activeStates: activeStates,
		http:         &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) FetchCandidateIssues(ctx context.Context) ([]tracker.Issue, error) {
	return c.fetchPages(ctx, projectStateQuery, func(after string) map[string]any {
		return projectStateVars(c.projectSlug, c.activeStates, after)
	})
}

func (c *Client) FetchIssuesByStates(ctx context.Context, stateNames []string) ([]tracker.Issue, error) {
	if len(stateNames) == 0 {
		return nil, nil
	}
	return c.fetchPages(ctx, projectStateQuery, func(after string) map[string]any {
		return projectStateVars(c.projectSlug, stateNames, after)
	})
}

func (c *Client) FetchIssueStatesByIDs(ctx context.Context, issueIDs []string) ([]tracker.Issue, error) {
	if len(issueIDs) == 0 {
		return nil, nil
	}
	return c.fetchPages(ctx, byIDsQuery, func(after string) map[string]any {
		return byIDsVars(issueIDs, after)
	})
}

func (c *Client) fetchPages(ctx context.Context, query string, buildVars func(after string) map[string]any) ([]tracker.Issue, error) {
	var issues []tracker.Issue
	after := ""
	for {
		data, err := c.post(ctx, query, buildVars(after))
		if err != nil {
			return nil, err
		}
		var raw struct {
			Issues rawIssuesConn `json:"issues"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrUnknownPayload, err)
		}
		for _, node := range raw.Issues.Nodes {
			issue, err := normalizeIssue(node)
			if err != nil {
				return nil, fmt.Errorf("%w: normalize %s: %s", ErrUnknownPayload, node.ID, err)
			}
			issues = append(issues, issue)
		}
		pi := raw.Issues.PageInfo
		if !pi.HasNextPage {
			break
		}
		if pi.EndCursor == "" {
			return nil, ErrMissingEndCursor
		}
		after = pi.EndCursor
	}
	return issues, nil
}

func (c *Client) post(ctx context.Context, query string, vars map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return nil, fmt.Errorf("%w: encode: %s", ErrUnknownPayload, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAPIRequest, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAPIRequest, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: %d", ErrAPIStatus, resp.StatusCode)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPayload, err)
	}
	if len(envelope.Errors) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrGraphQLErrors, envelope.Errors[0].Message)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, fmt.Errorf("%w: missing data field", ErrUnknownPayload)
	}
	return envelope.Data, nil
}
