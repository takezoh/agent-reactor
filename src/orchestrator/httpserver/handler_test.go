package httpserver_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/httpserver"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/platform/metrics"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"
)

// fakeScheduler implements SchedulerReader for tests.
type fakeScheduler struct {
	snap          scheduler.StateSnapshot
	refreshed     bool
	coalesce      bool
	snapshotCalls int
}

func (f *fakeScheduler) Snapshot() scheduler.StateSnapshot {
	f.snapshotCalls++
	return f.snap
}
func (f *fakeScheduler) Refresh() (coalesced bool) {
	f.refreshed = true
	return f.coalesce
}

func newMux(sched httpserver.SchedulerReader) http.Handler {
	return httpserver.NewMux(sched, "/tmp/workspaces")
}

func getBody(t *testing.T, h http.Handler, method, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	body, _ := io.ReadAll(rr.Body)
	return rr.Code, body
}

// TestStateEndpoint_EmptySnapshot ensures the /api/v1/state shape is correct when no issues run.
func TestStateEndpoint_EmptySnapshot(t *testing.T) {
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running:       map[string]scheduler.RunAttempt{},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/api/v1/state")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200; body: %s", status, body)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := resp["generated_at"]; !ok {
		t.Error("missing field generated_at")
	}
	counts, _ := resp["counts"].(map[string]any)
	if counts["running"].(float64) != 0 {
		t.Errorf("counts.running want 0, got %v", counts["running"])
	}
	if _, ok := resp["running"]; !ok {
		t.Error("missing field running")
	}
	if _, ok := resp["retrying"]; !ok {
		t.Error("missing field retrying")
	}
	if _, ok := resp["codex_totals"]; !ok {
		t.Error("missing field codex_totals")
	}
}

// TestStateEndpoint_WithRunningIssue validates the running entry shape matches §13.7.2.
func TestStateEndpoint_WithRunningIssue(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running: map[string]scheduler.RunAttempt{
			"id1": {
				Issue:              ptrackerv.Issue{ID: "id1", Identifier: "MT-649", State: "In Progress"},
				Session:            scheduler.LiveSession{SessionID: "thread-1-turn-1"},
				Attempt:            1,
				StartedAt:          now,
				LastCodexEvent:     "turn_completed",
				LastCodexTimestamp: now,
				TotalInputTokens:   1200,
				TotalOutputTokens:  800,
				TotalTokens:        2000,
			},
		},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
		CodexTotals:   metrics.Totals{Input: 1200, Output: 800, Total: 2000},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/api/v1/state")
	if status != http.StatusOK {
		t.Fatalf("status %d, body: %s", status, body)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	running := resp["running"].([]any)
	if len(running) != 1 {
		t.Fatalf("running len want 1, got %d", len(running))
	}
	entry := running[0].(map[string]any)
	if entry["issue_id"] != "id1" {
		t.Errorf("issue_id want id1, got %v", entry["issue_id"])
	}
	if entry["issue_identifier"] != "MT-649" {
		t.Errorf("issue_identifier want MT-649, got %v", entry["issue_identifier"])
	}
	if entry["session_id"] != "thread-1-turn-1" {
		t.Errorf("session_id want thread-1-turn-1, got %v", entry["session_id"])
	}

	tokens := entry["tokens"].(map[string]any)
	if tokens["input_tokens"].(float64) != 1200 {
		t.Errorf("input_tokens want 1200, got %v", tokens["input_tokens"])
	}

	totals := resp["codex_totals"].(map[string]any)
	if totals["input_tokens"].(float64) != 1200 {
		t.Errorf("codex_totals.input_tokens want 1200, got %v", totals["input_tokens"])
	}

	counts := resp["counts"].(map[string]any)
	if counts["running"].(float64) != 1 {
		t.Errorf("counts.running want 1, got %v", counts["running"])
	}
}

// TestStateEndpoint_RateLimitMostRecent verifies that with multiple running
// issues carrying rate limits, the state response reports the snapshot from the
// most-recently-active issue (deterministic, not map-iteration-order dependent).
func TestStateEndpoint_RateLimitMostRecent(t *testing.T) {
	older := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	newer := time.Now().UTC().Truncate(time.Second)
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running: map[string]scheduler.RunAttempt{
			"id-old": {
				Issue:              ptrackerv.Issue{ID: "id-old", Identifier: "MT-1", State: "In Progress"},
				LastCodexTimestamp: older,
				RateLimit:          &metrics.RateLimitSnapshot{PrimaryUsedPercent: 20},
			},
			"id-new": {
				Issue:              ptrackerv.Issue{ID: "id-new", Identifier: "MT-2", State: "In Progress"},
				LastCodexTimestamp: newer,
				RateLimit:          &metrics.RateLimitSnapshot{PrimaryUsedPercent: 90},
			},
		},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)
	// Run repeatedly: map order is randomized, so a non-deterministic selection
	// would eventually pick the older issue and fail.
	for i := 0; i < 20; i++ {
		_, body := getBody(t, h, http.MethodGet, "/api/v1/state")
		var resp map[string]any
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		rl, ok := resp["rate_limits"].(map[string]any)
		if !ok {
			t.Fatalf("rate_limits missing or null: %s", body)
		}
		if rl["primary_used_percent"].(float64) != 90 {
			t.Fatalf("want most-recent rate limit (90), got %v", rl["primary_used_percent"])
		}
	}
}

// TestStateEndpoint_WithRetrying validates the retrying entry shape.
func TestStateEndpoint_WithRetrying(t *testing.T) {
	dueMS := time.Now().Add(30 * time.Second).UnixMilli()
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running: map[string]scheduler.RunAttempt{},
		Claimed: map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{
			"id2": {
				IssueID:    "id2",
				Identifier: "MT-650",
				Attempt:    3,
				DueAtMS:    dueMS,
			},
		},
	}}
	h := newMux(sched)
	_, body := getBody(t, h, http.MethodGet, "/api/v1/state")

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	retrying := resp["retrying"].([]any)
	if len(retrying) != 1 {
		t.Fatalf("retrying len want 1, got %d", len(retrying))
	}
	entry := retrying[0].(map[string]any)
	if entry["issue_identifier"] != "MT-650" {
		t.Errorf("issue_identifier want MT-650, got %v", entry["issue_identifier"])
	}
	if entry["attempt"].(float64) != 3 {
		t.Errorf("attempt want 3, got %v", entry["attempt"])
	}
	if entry["due_at"] == "" {
		t.Error("due_at should be non-empty")
	}
}

// TestRefreshEndpoint_202 validates POST /api/v1/refresh returns 202 with the correct shape.
func TestRefreshEndpoint_202(t *testing.T) {
	sched := &fakeScheduler{}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodPost, "/api/v1/refresh")
	if status != http.StatusAccepted {
		t.Fatalf("status want 202, got %d; body: %s", status, body)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["queued"] != true {
		t.Errorf("queued want true, got %v", resp["queued"])
	}
	if resp["coalesced"] != false {
		t.Errorf("coalesced want false, got %v", resp["coalesced"])
	}
	if _, ok := resp["requested_at"]; !ok {
		t.Error("missing requested_at")
	}
	ops := resp["operations"].([]any)
	if len(ops) != 2 {
		t.Errorf("operations len want 2, got %d", len(ops))
	}
	if !sched.refreshed {
		t.Error("Refresh() was not called")
	}
}

// TestRefreshEndpoint_Coalesced checks the coalesced=true path.
func TestRefreshEndpoint_Coalesced(t *testing.T) {
	sched := &fakeScheduler{coalesce: true}
	h := newMux(sched)
	_, body := getBody(t, h, http.MethodPost, "/api/v1/refresh")

	var resp map[string]any
	_ = json.Unmarshal(body, &resp)
	if resp["coalesced"] != true {
		t.Errorf("coalesced want true, got %v", resp["coalesced"])
	}
}

// TestIssueEndpoint_Running validates per-issue GET when issue is running.
func TestIssueEndpoint_Running(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running: map[string]scheduler.RunAttempt{
			"id1": {
				Issue:             ptrackerv.Issue{ID: "id1", Identifier: "MT-649", State: "In Progress"},
				Session:           scheduler.LiveSession{SessionID: "sess-1"},
				Attempt:           2,
				StartedAt:         now,
				TotalInputTokens:  100,
				TotalOutputTokens: 50,
				TotalTokens:       150,
			},
		},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/api/v1/MT-649")
	if status != http.StatusOK {
		t.Fatalf("status want 200, got %d; body: %s", status, body)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["issue_identifier"] != "MT-649" {
		t.Errorf("issue_identifier want MT-649, got %v", resp["issue_identifier"])
	}
	if resp["status"] != "running" {
		t.Errorf("status want running, got %v", resp["status"])
	}
	if resp["running"] == nil {
		t.Error("running should be non-null")
	}
	if resp["retry"] != nil {
		t.Errorf("retry should be null, got %v", resp["retry"])
	}
	ws := resp["workspace"].(map[string]any)
	if ws["path"] == "" {
		t.Error("workspace.path should be non-empty")
	}
	attempts := resp["attempts"].(map[string]any)
	if attempts["current_retry_attempt"].(float64) != 2 {
		t.Errorf("current_retry_attempt want 2, got %v", attempts["current_retry_attempt"])
	}
}

// TestIssueEndpoint_Retrying validates per-issue GET when issue is retrying.
func TestIssueEndpoint_Retrying(t *testing.T) {
	dueMS := time.Now().Add(10 * time.Second).UnixMilli()
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running: map[string]scheduler.RunAttempt{},
		Claimed: map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{
			"id2": {
				IssueID:    "id2",
				Identifier: "MT-650",
				Attempt:    3,
				DueAtMS:    dueMS,
			},
		},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/api/v1/MT-650")
	if status != http.StatusOK {
		t.Fatalf("status want 200, got %d; body: %s", status, body)
	}

	var resp map[string]any
	_ = json.Unmarshal(body, &resp)
	if resp["status"] != "retrying" {
		t.Errorf("status want retrying, got %v", resp["status"])
	}
	if resp["retry"] == nil {
		t.Error("retry should be non-null")
	}
	if resp["running"] != nil {
		t.Errorf("running should be null, got %v", resp["running"])
	}
}

// TestIssueEndpoint_NotFound validates 404 + error envelope for unknown identifiers.
func TestIssueEndpoint_NotFound(t *testing.T) {
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running:       map[string]scheduler.RunAttempt{},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/api/v1/UNKNOWN-999")
	if status != http.StatusNotFound {
		t.Fatalf("status want 404, got %d; body: %s", status, body)
	}

	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	errObj := envelope["error"].(map[string]any)
	if errObj["code"] != "issue_not_found" {
		t.Errorf("error.code want issue_not_found, got %v", errObj["code"])
	}
	if _, ok := errObj["message"]; !ok {
		t.Error("error.message missing")
	}
}

// TestMethodNotAllowed_405 validates 405 + error envelope for unsupported methods.
func TestMethodNotAllowed_405(t *testing.T) {
	sched := &fakeScheduler{}
	h := newMux(sched)

	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/state"},
		{http.MethodGet, "/api/v1/refresh"},
		{http.MethodDelete, "/api/v1/state"},
	}
	for _, tc := range cases {
		status, body := getBody(t, h, tc.method, tc.path)
		if status != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status want 405, got %d; body: %s", tc.method, tc.path, status, body)
			continue
		}
		var envelope map[string]any
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Errorf("%s %s: invalid JSON: %v", tc.method, tc.path, err)
			continue
		}
		errObj, _ := envelope["error"].(map[string]any)
		if errObj == nil {
			t.Errorf("%s %s: missing error field", tc.method, tc.path)
		}
	}
}

// TestDashboard_200 validates GET / returns a 200 static HTML shell that
// consumes the JSON REST API client-side (it must not read scheduler state).
func TestDashboard_200(t *testing.T) {
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running:       map[string]scheduler.RunAttempt{},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/")
	if status != http.StatusOK {
		t.Fatalf("status want 200, got %d", status)
	}
	s := string(body)
	if !strings.Contains(s, "Orchestrator Dashboard") {
		t.Error("dashboard body should contain 'Orchestrator Dashboard'")
	}
	// The shell is an API client: it fetches state and posts refresh.
	if !strings.Contains(s, "/api/v1/state") {
		t.Error("dashboard should fetch /api/v1/state client-side")
	}
	if !strings.Contains(s, "/api/v1/refresh") {
		t.Error("dashboard should POST /api/v1/refresh for manual refresh")
	}
	// Decoupling: rendering the dashboard must not touch scheduler state.
	if sched.snapshotCalls != 0 {
		t.Errorf("GET / must not call Snapshot(); got %d calls", sched.snapshotCalls)
	}
}

// TestProjectState_CodexTotals verifies codex_totals is projected from snapshot.
func TestProjectState_CodexTotals(t *testing.T) {
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running:             map[string]scheduler.RunAttempt{},
		Claimed:             map[string]struct{}{},
		RetryAttempts:       map[string]scheduler.RetryEntry{},
		CodexTotals:         metrics.Totals{Input: 5000, Output: 2400, Total: 7400},
		CodexSecondsRunning: 1834.2,
	}}
	h := newMux(sched)
	_, body := getBody(t, h, http.MethodGet, "/api/v1/state")

	var resp map[string]any
	_ = json.Unmarshal(body, &resp)
	totals := resp["codex_totals"].(map[string]any)
	if totals["input_tokens"].(float64) != 5000 {
		t.Errorf("input_tokens want 5000, got %v", totals["input_tokens"])
	}
	if totals["seconds_running"].(float64) != 1834.2 {
		t.Errorf("seconds_running want 1834.2, got %v", totals["seconds_running"])
	}
}

// TestStateEndpoint_TurnCount verifies that the running entry reflects the
// TurnCount from RunAttempt (SPEC §4.1.6 / DEV-179).
func TestStateEndpoint_TurnCount(t *testing.T) {
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running: map[string]scheduler.RunAttempt{
			"id-tc": {
				Issue:     ptrackerv.Issue{ID: "id-tc", Identifier: "TC-1", State: "In Progress"},
				Attempt:   1,
				TurnCount: 3,
			},
		},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/api/v1/state")
	if status != http.StatusOK {
		t.Fatalf("status %d, body: %s", status, body)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	running := resp["running"].([]any)
	if len(running) != 1 {
		t.Fatalf("running len want 1, got %d", len(running))
	}
	entry := running[0].(map[string]any)
	if entry["turn_count"].(float64) != 3 {
		t.Errorf("turn_count want 3, got %v", entry["turn_count"])
	}
}

// TestIssueEndpoint_TurnCount verifies that the /api/v1/{identifier} running
// detail also reflects the correct TurnCount (SPEC §4.1.6 / DEV-179).
func TestIssueEndpoint_TurnCount(t *testing.T) {
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running: map[string]scheduler.RunAttempt{
			"id-tc2": {
				Issue:     ptrackerv.Issue{ID: "id-tc2", Identifier: "TC-2", State: "In Progress"},
				Attempt:   1,
				TurnCount: 5,
			},
		},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)
	status, body := getBody(t, h, http.MethodGet, "/api/v1/TC-2")
	if status != http.StatusOK {
		t.Fatalf("status %d, body: %s", status, body)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	running := resp["running"].(map[string]any)
	if running["turn_count"].(float64) != 5 {
		t.Errorf("running.turn_count want 5, got %v", running["turn_count"])
	}
}
