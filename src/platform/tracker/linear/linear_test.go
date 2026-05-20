package linear_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/platform/tracker"
	"github.com/takezoh/agent-roost/platform/tracker/linear"
)

// issuesResp builds a minimal valid "issues" GraphQL response body.
func issuesResp(nodes []map[string]any, hasNextPage bool, endCursor string) string {
	data := map[string]any{
		"issues": map[string]any{
			"pageInfo": map[string]any{
				"hasNextPage": hasNextPage,
				"endCursor":   endCursor,
			},
			"nodes": nodes,
		},
	}
	b, _ := json.Marshal(map[string]any{"data": data})
	return string(b)
}

func fakeNode(id, identifier string) map[string]any {
	return map[string]any{
		"id": id, "identifier": identifier, "title": "T", "description": "",
		"priority": nil, "url": "", "branchName": "",
		"state":            map[string]any{"name": "Todo"},
		"labels":           map[string]any{"nodes": []any{}},
		"inverseRelations": map[string]any{"nodes": []any{}},
		"createdAt":        "2024-01-01T00:00:00Z",
		"updatedAt":        "2024-01-01T00:00:00Z",
	}
}

// captureServer starts a test server that captures the raw request body and replies with resp.
func captureServer(t *testing.T, resp string) (*httptest.Server, *string) {
	t.Helper()
	captured := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// sequenceServer returns different responses on successive calls.
func sequenceServer(t *testing.T, responses []string) (*httptest.Server, *int) {
	t.Helper()
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if count < len(responses) {
			io.WriteString(w, responses[count])
		}
		count++
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

// --- §17.3 conformance tests ---

func TestSPEC_17_3_LinearProjectFilterUsesSlugId(t *testing.T) {
	srv, body := captureServer(t, issuesResp(nil, false, ""))
	c := linear.New(srv.URL, "key", "myproject", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(*body, "slugId") {
		t.Errorf("query missing slugId filter; body: %s", *body)
	}
	if !strings.Contains(*body, "myproject") {
		t.Errorf("project slug 'myproject' missing from request body; body: %s", *body)
	}
}

func TestSPEC_17_3_CandidateFetchUsesActiveStates(t *testing.T) {
	srv, body := captureServer(t, issuesResp(nil, false, ""))
	c := linear.New(srv.URL, "key", "slug", []string{"Todo", "In Progress"})
	_, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(*body, "Todo") || !strings.Contains(*body, "In Progress") {
		t.Errorf("active states missing from request; body: %s", *body)
	}
}

func TestSPEC_17_3_FetchIssuesByStates_EmptyStates_NoAPICall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	issues, err := c.FetchIssuesByStates(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("API should not be called when stateNames is empty")
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestSPEC_17_3_PaginationPreservesOrder(t *testing.T) {
	page1 := issuesResp(
		[]map[string]any{fakeNode("id1", "PROJ-1"), fakeNode("id2", "PROJ-2")},
		true, "cursor1",
	)
	page2 := issuesResp(
		[]map[string]any{fakeNode("id3", "PROJ-3")},
		false, "",
	)
	srv, _ := sequenceServer(t, []string{page1, page2})

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	issues, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues across 2 pages, got %d", len(issues))
	}
	want := []string{"PROJ-1", "PROJ-2", "PROJ-3"}
	for i, w := range want {
		if issues[i].Identifier != w {
			t.Errorf("position %d: want %s, got %s", i, w, issues[i].Identifier)
		}
	}
}

func TestSPEC_17_3_MissingEndCursorError(t *testing.T) {
	resp := issuesResp([]map[string]any{fakeNode("id1", "PROJ-1")}, true, "")
	srv, _ := captureServer(t, resp)

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if !errors.Is(err, linear.ErrMissingEndCursor) {
		t.Errorf("want ErrMissingEndCursor, got %v", err)
	}
}

func TestSPEC_17_3_BlockedByFromBlocksInverseRelation(t *testing.T) {
	node := fakeNode("id1", "PROJ-1")
	node["inverseRelations"] = map[string]any{
		"nodes": []any{
			map[string]any{
				"type": "blocks",
				"issue": map[string]any{
					"id": "blocker-id", "identifier": "PROJ-0",
					"state": map[string]any{"name": "In Progress"},
				},
			},
			map[string]any{
				"type": "relates to",
				"issue": map[string]any{
					"id": "other-id", "identifier": "PROJ-9",
					"state": map[string]any{"name": "Todo"},
				},
			},
		},
	}
	srv, _ := captureServer(t, issuesResp([]map[string]any{node}, false, ""))

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	issues, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	blockers := issues[0].BlockedBy
	if len(blockers) != 1 {
		t.Fatalf("expected 1 blocker (only 'blocks' type), got %d", len(blockers))
	}
	b := blockers[0]
	if b.ID != "blocker-id" || b.Identifier != "PROJ-0" || b.State != "In Progress" {
		t.Errorf("unexpected blocker: %+v", b)
	}
}

func TestSPEC_17_3_LabelsLowercase(t *testing.T) {
	node := fakeNode("id1", "PROJ-1")
	node["labels"] = map[string]any{
		"nodes": []any{
			map[string]any{"name": "Backend"},
			map[string]any{"name": "HIGH-PRIORITY"},
		},
	}
	srv, _ := captureServer(t, issuesResp([]map[string]any{node}, false, ""))

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	issues, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	labels := issues[0].Labels
	for _, l := range labels {
		if l != strings.ToLower(l) {
			t.Errorf("label not lowercased: %q", l)
		}
	}
}

func TestSPEC_17_3_PriorityIntegerOnly(t *testing.T) {
	makeNode := func(id string, priority any) map[string]any {
		n := fakeNode(id, id)
		n["priority"] = priority
		return n
	}
	nodes := []map[string]any{
		makeNode("a", 2),
		makeNode("b", 1.5),
		makeNode("c", nil),
	}
	srv, _ := captureServer(t, issuesResp(nodes, false, ""))

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	issues, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issues[0].Priority == nil || *issues[0].Priority != 2 {
		t.Errorf("integer priority: want 2, got %v", issues[0].Priority)
	}
	if issues[1].Priority != nil {
		t.Errorf("non-integer priority: want nil, got %v", issues[1].Priority)
	}
	if issues[2].Priority != nil {
		t.Errorf("null priority: want nil, got %v", issues[2].Priority)
	}
}

func TestSPEC_17_3_TimesParsed(t *testing.T) {
	node := fakeNode("id1", "PROJ-1")
	node["createdAt"] = "2024-03-15T08:30:00.123Z"
	node["updatedAt"] = "2024-03-15T09:00:00Z"
	srv, _ := captureServer(t, issuesResp([]map[string]any{node}, false, ""))

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	issues, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantCreated := time.Date(2024, 3, 15, 8, 30, 0, 123_000_000, time.UTC)
	if !issues[0].CreatedAt.Equal(wantCreated) {
		t.Errorf("createdAt: want %v, got %v", wantCreated, issues[0].CreatedAt)
	}
	if issues[0].UpdatedAt.IsZero() {
		t.Error("updatedAt should not be zero")
	}
}

func TestSPEC_17_3_FetchIssueStatesByIDsUsesIDType(t *testing.T) {
	srv, body := captureServer(t, issuesResp(nil, false, ""))
	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	_, err := c.FetchIssueStatesByIDs(context.Background(), []string{"id1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(*body, "[ID!]") {
		t.Errorf("query must declare ids as [ID!]; body: %s", *body)
	}
}

func TestSPEC_17_3_FetchIssueStatesByIDs_EmptyIDs_NoAPICall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	issues, err := c.FetchIssueStatesByIDs(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("API should not be called when issueIDs is empty")
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

// --- Error mapping tests ---

func TestErrorMapping_RequestError(t *testing.T) {
	c := linear.New("http://127.0.0.1:1", "key", "slug", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if !errors.Is(err, linear.ErrAPIRequest) {
		t.Errorf("want ErrAPIRequest, got %v", err)
	}
}

func TestErrorMapping_NonHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":"internal"}`)
	}))
	t.Cleanup(srv.Close)

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if !errors.Is(err, linear.ErrAPIStatus) {
		t.Errorf("want ErrAPIStatus, got %v", err)
	}
}

func TestErrorMapping_GraphQLErrors(t *testing.T) {
	resp := `{"errors":[{"message":"Unauthorized"}]}`
	srv, _ := captureServer(t, resp)

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if !errors.Is(err, linear.ErrGraphQLErrors) {
		t.Errorf("want ErrGraphQLErrors, got %v", err)
	}
}

func TestErrorMapping_MalformedPayload(t *testing.T) {
	srv, _ := captureServer(t, `not-json`)

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if !errors.Is(err, linear.ErrUnknownPayload) {
		t.Errorf("want ErrUnknownPayload, got %v", err)
	}
}

func TestErrorMapping_MissingDataField(t *testing.T) {
	srv, _ := captureServer(t, `{"data":null}`)

	c := linear.New(srv.URL, "key", "slug", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if !errors.Is(err, linear.ErrUnknownPayload) {
		t.Errorf("want ErrUnknownPayload, got %v", err)
	}
}

// --- Adapter interface conformance ---

func TestAdapterInterface(t *testing.T) {
	// Verifies *Client satisfies tracker.Adapter at compile time.
	// The var _ check in linear.go already ensures this;
	// this test documents the behaviour via a runtime assertion.
	var _ tracker.Adapter = linear.New("", "", "", nil)
}

// --- Authorization header ---

func TestAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, issuesResp(nil, false, ""))
	}))
	t.Cleanup(srv.Close)

	c := linear.New(srv.URL, "my-api-key", "slug", []string{"Todo"})
	_, err := c.FetchCandidateIssues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "my-api-key" {
		t.Errorf("Authorization header: want %q, got %q", "my-api-key", gotAuth)
	}
}
