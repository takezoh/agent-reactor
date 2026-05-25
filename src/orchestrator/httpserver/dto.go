// Package httpserver implements the SPEC §13.7 observability HTTP server.
package httpserver

import "encoding/json"

// stateResponse is the body of GET /api/v1/state (§13.7.2).
type stateResponse struct {
	GeneratedAt string          `json:"generated_at"`
	Counts      stateCounts     `json:"counts"`
	Running     []runningEntry  `json:"running"`
	Retrying    []retryingEntry `json:"retrying"`
	CodexTotals *codexTotals    `json:"codex_totals"`
	RateLimits  *rateLimits     `json:"rate_limits"`
}

type stateCounts struct {
	Running  int `json:"running"`
	Retrying int `json:"retrying"`
}

// runningEntry represents one running issue in the state response.
type runningEntry struct {
	IssueID         string      `json:"issue_id"`
	IssueIdentifier string      `json:"issue_identifier"`
	State           string      `json:"state"`
	SessionID       string      `json:"session_id"`
	TurnCount       int         `json:"turn_count"`
	LastEvent       string      `json:"last_event"`
	LastMessage     string      `json:"last_message"`
	StartedAt       string      `json:"started_at"`
	LastEventAt     string      `json:"last_event_at"`
	Tokens          tokenCounts `json:"tokens"`
}

// retryingEntry represents one retrying issue in the state response.
type retryingEntry struct {
	IssueID         string `json:"issue_id"`
	IssueIdentifier string `json:"issue_identifier"`
	Attempt         int    `json:"attempt"`
	DueAt           string `json:"due_at"`
	Error           string `json:"error,omitempty"`
}

// codexTotals is the lifetime aggregate token/runtime section of the state response.
type codexTotals struct {
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	TotalTokens    int64   `json:"total_tokens"`
	SecondsRunning float64 `json:"seconds_running"`
}

// rateLimits is the latest rate-limit snapshot across all running issues.
type rateLimits struct {
	PrimaryUsedPercent   int64 `json:"primary_used_percent"`
	PrimaryResetsAt      int64 `json:"primary_resets_at"`
	SecondaryUsedPercent int64 `json:"secondary_used_percent"`
	SecondaryResetsAt    int64 `json:"secondary_resets_at"`
}

// tokenCounts is a per-issue token summary.
type tokenCounts struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// issueResponse is the body of GET /api/v1/{issue_identifier} (§13.7.2).
type issueResponse struct {
	IssueIdentifier string          `json:"issue_identifier"`
	IssueID         string          `json:"issue_id"`
	Status          string          `json:"status"`
	Workspace       issueWorkspace  `json:"workspace"`
	Attempts        issueAttempts   `json:"attempts"`
	Running         *runningDetail  `json:"running"`
	Retry           *retryDetail    `json:"retry"`
	Logs            issueLogs       `json:"logs"`
	RecentEvents    []recentEvent   `json:"recent_events"`
	LastError       *string         `json:"last_error"`
	Tracked         json.RawMessage `json:"tracked"`
}

type issueWorkspace struct {
	Path string `json:"path"`
}

type issueAttempts struct {
	RestartCount        int `json:"restart_count"`
	CurrentRetryAttempt int `json:"current_retry_attempt"`
}

// runningDetail is the running sub-object in issueResponse.
type runningDetail struct {
	SessionID   string      `json:"session_id"`
	TurnCount   int         `json:"turn_count"`
	State       string      `json:"state"`
	StartedAt   string      `json:"started_at"`
	LastEvent   string      `json:"last_event"`
	LastMessage string      `json:"last_message"`
	LastEventAt string      `json:"last_event_at"`
	Tokens      tokenCounts `json:"tokens"`
}

// retryDetail is the retry sub-object in issueResponse (non-null when status == "retrying").
type retryDetail struct {
	Attempt int    `json:"attempt"`
	DueAt   string `json:"due_at"`
	Error   string `json:"error,omitempty"`
}

type issueLogs struct {
	CodexSessionLogs []codexLogEntry `json:"codex_session_logs"`
}

type codexLogEntry struct {
	Label string  `json:"label"`
	Path  string  `json:"path"`
	URL   *string `json:"url"`
}

type recentEvent struct {
	At      string `json:"at"`
	Event   string `json:"event"`
	Message string `json:"message"`
}

// refreshResponse is the body of POST /api/v1/refresh (§13.7.2).
type refreshResponse struct {
	Queued      bool     `json:"queued"`
	Coalesced   bool     `json:"coalesced"`
	RequestedAt string   `json:"requested_at"`
	Operations  []string `json:"operations"`
}

// errorEnvelope is the standard error body for all error responses (§13.7.2).
type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
