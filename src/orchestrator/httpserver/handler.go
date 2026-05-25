package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/scheduler"
)

// snapshotTimeout bounds the per-request context for a snapshot read (SPEC §13.3).
// The scheduler snapshot itself is lock-free (atomic.Pointer) and cannot block, so this
// is a defensive ceiling on the HTTP request, not a wait on a state lock.
const snapshotTimeout = 500 * time.Millisecond

// SchedulerReader is the read-only interface the HTTP handlers use to access
// scheduler state. Satisfied by *scheduler.Scheduler; use a fake in tests.
type SchedulerReader interface {
	SnapshotCtx(ctx context.Context) (scheduler.StateSnapshot, error)
	Refresh() (coalesced bool)
}

// NewMux builds and returns the HTTP handler for the observability server.
// workspaceRoot is used to derive workspace paths for per-issue responses.
func NewMux(sched SchedulerReader, workspaceRoot string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			writeError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		// The dashboard is a static shell that consumes the JSON API client-side;
		// it does not read scheduler state here.
		renderDashboard(w)
	})

	mux.HandleFunc("GET /api/v1/state", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), snapshotTimeout)
		defer cancel()
		snap, err := sched.SnapshotCtx(ctx)
		if err != nil {
			writeSnapshotError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, projectState(snap, time.Now()))
	})

	// Explicit 405 for non-POST on /api/v1/refresh so GET /api/v1/{identifier}
	// wildcard does not consume wrong-method requests for this fixed path.
	mux.HandleFunc("GET /api/v1/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})

	mux.HandleFunc("POST /api/v1/refresh", func(w http.ResponseWriter, r *http.Request) {
		coalesced := sched.Refresh()
		writeJSON(w, http.StatusAccepted, refreshResponse{
			Queued:      true,
			Coalesced:   coalesced,
			RequestedAt: time.Now().UTC().Format(time.RFC3339),
			Operations:  []string{"poll", "reconcile"},
		})
	})

	// Per-issue endpoint. Must be registered after /api/v1/state and /api/v1/refresh
	// so those concrete paths win over the wildcard.
	mux.HandleFunc("GET /api/v1/{identifier}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("identifier")
		// Guard against empty and path separators (shouldn't occur with ServeMux, but defensive).
		if id == "" || strings.ContainsRune(id, '/') {
			writeError(w, http.StatusBadRequest, "invalid_identifier", "invalid identifier")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), snapshotTimeout)
		defer cancel()
		snap, err := sched.SnapshotCtx(ctx)
		if err != nil {
			writeSnapshotError(w, err)
			return
		}
		resp := projectIssue(snap, id, workspaceRoot)
		if resp == nil {
			writeError(w, http.StatusNotFound, "issue_not_found", "issue not found: "+id)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	return methodNotAllowedWrapper(mux)
}

// methodNotAllowedWrapper wraps a ServeMux so that 405 responses from the mux
// (auto-generated when path matches but method does not) are re-encoded as error envelopes.
func methodNotAllowedWrapper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusCapture{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.status == http.StatusMethodNotAllowed && !rw.written {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(errorEnvelope{
				Error: errorDetail{Code: "method_not_allowed", Message: "method not allowed"},
			})
		}
	})
}

// statusCapture records the status code set by WriteHeader without writing a body.
type statusCapture struct {
	http.ResponseWriter
	status  int
	written bool
}

func (s *statusCapture) WriteHeader(code int) {
	s.status = code
	if code != http.StatusMethodNotAllowed {
		s.ResponseWriter.WriteHeader(code)
		s.written = true
	}
}

func (s *statusCapture) Write(b []byte) (int, error) {
	if s.status == http.StatusMethodNotAllowed && !s.written {
		return len(b), nil // swallow mux-generated body; wrapper will write the envelope
	}
	s.written = true
	return s.ResponseWriter.Write(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: errorDetail{Code: code, Message: message}})
}

// writeSnapshotError maps ErrOrchestratorUnavailable → 503; other errors → 500 (SPEC §13.3).
// The lock-free snapshot cannot time out, so there is no snapshot_timeout path.
func writeSnapshotError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scheduler.ErrOrchestratorUnavailable):
		writeError(w, http.StatusServiceUnavailable, "orchestrator_unavailable", "orchestrator unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
