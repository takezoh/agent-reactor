package httpserver_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/scheduler"
)

// SPEC §17.6/§13.3 — orchestrator_unavailable returns 503 with the correct error envelope.
// (The snapshot_timeout path is gone: the scheduler now publishes immutable state read
// lock-free, so a snapshot read cannot block or time out — see scheduler/snapshot.go.)
func TestSPEC_17_6_OrchestratorUnavailable(t *testing.T) {
	sched := &fakeScheduler{snapshotErr: scheduler.ErrOrchestratorUnavailable}
	h := newMux(sched)

	// Test both snapshot-reading endpoints.
	endpoints := []string{"/api/v1/state", "/api/v1/ISSUE-1"}
	for _, ep := range endpoints {
		status, body := getBody(t, h, http.MethodGet, ep)
		if status != http.StatusServiceUnavailable {
			t.Errorf("%s: status %d, want 503; body: %s", ep, status, body)
			continue
		}

		var resp map[string]any
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Errorf("%s: invalid JSON: %v; body: %s", ep, err, body)
			continue
		}
		errField, ok := resp["error"].(map[string]any)
		if !ok {
			t.Errorf("%s: want {\"error\": {...}} envelope, got: %s", ep, body)
			continue
		}
		if errField["code"] != "orchestrator_unavailable" {
			t.Errorf("%s: error.code want \"orchestrator_unavailable\", got %v", ep, errField["code"])
		}
	}
}

// SPEC §17.6 — /api/v1/state response contains the required top-level fields
// (generated_at, counts, running, retrying, codex_totals) per §13.7.
func TestSPEC_17_6_StateShape(t *testing.T) {
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

	required := []string{"generated_at", "counts", "running", "retrying", "codex_totals"}
	for _, field := range required {
		if _, ok := resp[field]; !ok {
			t.Errorf("state shape: missing required field %q", field)
		}
	}
}

// SPEC §17.6 — a 405 Method Not Allowed response uses the standard error envelope
// {"error": {"code": "method_not_allowed", ...}} rather than a bare status line.
func TestSPEC_17_6_MethodNotAllowedEnvelope(t *testing.T) {
	sched := &fakeScheduler{snap: scheduler.StateSnapshot{
		Running:       map[string]scheduler.RunAttempt{},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]scheduler.RetryEntry{},
	}}
	h := newMux(sched)

	// DELETE on /api/v1/state is not a registered method → 405.
	status, body := getBody(t, h, http.MethodDelete, "/api/v1/state")
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, want 405; body: %s", status, body)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("405 body is not valid JSON: %v; body: %s", err, body)
	}
	errField, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("want {\"error\": {...}} envelope, got: %s", body)
	}
	if errField["code"] != "method_not_allowed" {
		t.Errorf("error.code want \"method_not_allowed\", got %v", errField["code"])
	}
}
