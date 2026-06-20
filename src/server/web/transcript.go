package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
	stateview "github.com/takezoh/agent-reactor/client/state/view"
)

// sessionIDPattern is the allowlist regex for session IDs (ADR 0026).
// Only alphanumeric characters, underscores, and hyphens are permitted.
var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Typed sentinel errors returned by resolveSessionFilePath.
// Callers map these to HTTP status codes; internal detail stays server-side.
var (
	errSessionNotFound = errors.New("session not found")
	errNoTab           = errors.New("no log tab for session")
)

// handleGetTranscript returns an http.HandlerFunc that serves the transcript
// file for the given session.
func handleGetTranscript(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveSessionFile(d, w, r, "transcript", "text/plain; charset=utf-8")
	}
}

// handleGetEventLog returns an http.HandlerFunc that serves the event-log
// file for the given session.
func handleGetEventLog(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveSessionFile(d, w, r, "event-log", "application/x-ndjson")
	}
}

// serveSessionFile resolves the file path for the given session and kind,
// then streams from offset to EOF with ETag support.
func serveSessionFile(d *DaemonClient, w http.ResponseWriter, r *http.Request, kindMatch, contentType string) {
	// a. Validate session ID via allowlist (ADR 0026).
	id := r.PathValue("id")
	if !sessionIDPattern.MatchString(id) {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	// b. Parse offset query param.
	var offset int64
	if s := r.URL.Query().Get("offset"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 0 {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
		offset = n
	}

	// c. Check daemon availability before querying.
	if !d.Health() {
		http.Error(w, "daemon unavailable", http.StatusServiceUnavailable)
		return
	}

	// d. Resolve the file path via daemon session info.
	// resolveSessionFilePath returns typed sentinels (errSessionNotFound,
	// errNoTab) for 404 conditions, a *proto.ErrorBody for daemon-reported
	// errors, or a plain error for unexpected internal failures.
	path, err := resolveSessionFilePath(d, r, id, kindMatch)
	if err != nil {
		switch {
		case errors.Is(err, errSessionNotFound), errors.Is(err, errNoTab):
			http.Error(w, "transcript not found", http.StatusNotFound)
		default:
			// Proto errors (ErrNotFound → 404, ErrInvalidArgument → 400,
			// others → 500) and unexpected internal errors.
			handleProtoError(w, err)
		}
		return
	}

	// e. Stat the file to get size.
	fi, err := os.Stat(path)
	if err != nil {
		http.Error(w, "transcript not found", http.StatusNotFound)
		return
	}

	// f. Build ETag (ADR 0027 simple form: <sessionID>:<file-size>).
	etag := fmt.Sprintf(`"%s:%d"`, id, fi.Size())
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// g. Empty range: offset at or past EOF → 204 No Content.
	if offset >= fi.Size() {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// h. Open, seek, and stream.
	f, err := os.Open(path) //nolint:gosec // path is resolved from daemon-controlled LogTabs, not user input
	if err != nil {
		http.Error(w, "transcript not found", http.StatusNotFound)
		return
	}
	// i. Close file on exit.
	defer func() { _ = f.Close() }()

	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		http.Error(w, "seek error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// resolveSessionFilePath queries the daemon for session list info and returns
// the file path for the matching LogTab.
//
// Errors returned:
//   - errSessionNotFound: the session ID was not in the daemon response
//   - errNoTab: the session exists but has no matching LogTab
//   - *proto.ErrorBody: the daemon reported a protocol-level error
//   - plain error: unexpected internal failure (bad response type, etc.)
//
// The caller is responsible for mapping these to HTTP status codes; this
// function never writes to the ResponseWriter.
//
// TODO(future-pr): improve tab matching beyond label/path heuristics once the
// proto layer exposes a dedicated tab-kind field.
func resolveSessionFilePath(d *DaemonClient, r *http.Request, id, kindMatch string) (string, error) {
	resp, err := d.SendCommand(r.Context(), proto.CmdEvent{
		Event:   state.EventListSessions,
		Payload: json.RawMessage("{}"),
	})
	if err != nil {
		return "", err
	}
	rs, ok := resp.(proto.RespSessions)
	if !ok {
		return "", fmt.Errorf("unexpected response type")
	}

	// Find the session with the matching ID.
	var found *proto.SessionInfo
	for i := range rs.Sessions {
		if rs.Sessions[i].ID == id {
			found = &rs.Sessions[i]
			break
		}
	}
	if found == nil {
		return "", errSessionNotFound
	}

	// Match the LogTab by kind and label/path heuristic.
	path := matchLogTab(found.View.LogTabs, kindMatch)
	if path == "" {
		return "", errNoTab
	}
	return path, nil
}

// matchLogTab searches LogTabs for the best path matching the given kindMatch.
// kindMatch is "transcript" or "event-log".
//
// The driver-side LogTab labels are short uppercase strings ("TRANSCRIPT",
// "EVENTS"), and the file paths use driver-specific extensions
// (".transcript" for transcripts, ".log" for event logs — see
// client/driver/view_builder.go). The match table below enumerates the
// concrete tokens each kind may carry so that grep-style label matching
// stays correct as drivers are added.
//
// Matching priority:
//  1. TabKindText tab whose label (lower-cased) contains any of the kind's
//     label tokens.
//  2. TabKindText tab whose path suffix is one of the kind's path suffixes.
func matchLogTab(tabs []stateview.LogTab, kindMatch string) string {
	lowerKind := strings.ToLower(kindMatch)

	labelTokens, pathSuffixes := logTabMatchers(lowerKind)

	// The Kind filter is intentionally permissive: Claude drivers stamp
	// LogTab.Kind = "transcript", Codex drivers stamp "codex_transcript",
	// and EventLogTab stamps TabKindText for the .log path. Filtering on
	// TabKindText alone would silently exclude every actual transcript tab.
	// Match against label tokens first, then path suffix — the same
	// table-driven logic works across all kinds.
	for _, tab := range tabs {
		label := strings.ToLower(tab.Label)
		for _, tok := range labelTokens {
			if strings.Contains(label, tok) {
				return tab.Path
			}
		}
	}
	for _, tab := range tabs {
		for _, suf := range pathSuffixes {
			if strings.HasSuffix(tab.Path, suf) {
				return tab.Path
			}
		}
	}
	return ""
}

// logTabMatchers returns the (labelTokens, pathSuffixes) match table for one
// REST kind. The label tokens cover the upper-case label literals each driver
// sets in view_builder.go; the path suffixes mirror the file extensions
// driver code chooses (EventLogTab → ".log", driver transcript paths →
// ".transcript"). Add new entries when drivers ship new tab kinds.
func logTabMatchers(lowerKind string) (labels []string, pathSuffixes []string) {
	switch lowerKind {
	case "transcript":
		return []string{"transcript"}, []string{".transcript"}
	case "event-log":
		// "events" matches the EVENTS label that EventLogTab sets; "event-log"
		// is kept as a defensive synonym so future drivers can label their
		// JSON-line tabs explicitly.
		return []string{"events", "event-log"}, []string{".log", ".jsonl"}
	}
	return nil, nil
}
