package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
	stateview "github.com/takezoh/agent-reactor/client/state/view"
)

// --- helpers ---

// makeTranscriptFile creates a temporary file with the given content and
// returns its path. The file is cleaned up when the test ends.
func makeTranscriptFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// sessionRespWithTab builds a proto.RespSessions with a single session (id="abc")
// carrying one LogTab of kind TabKindText at the given path.
func sessionRespWithTab(label, path string) proto.RespSessions {
	return proto.RespSessions{
		Sessions: []proto.SessionInfo{
			{
				ID:      "abc",
				Command: "test",
				View: stateview.View{
					LogTabs: []stateview.LogTab{
						{
							Label: label,
							Path:  path,
							Kind:  stateview.TabKindText,
						},
					},
				},
			},
		},
	}
}

// sendFakeResponse drives the fakeDaemon for a single round-trip in a goroutine.
func sendFakeResponse(t *testing.T, fd *fakeDaemon, resp proto.Response) {
	t.Helper()
	go func() {
		env := fd.recv()
		fd.sendResp(env.ReqID, resp)
	}()
}

// authedGet returns an authenticated GET request to the given path.
func authedGet(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Authorization", "Bearer tok")
	return r
}

// --- TestTranscript_AcceptsValidID ---

// TestTranscript_AcceptsValidID verifies the happy path: a session with a
// TRANSCRIPT LogTab returns 200 with the file body and ETag header set.
func TestTranscript_AcceptsValidID(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	content := "line1\nline2\n"
	path := makeTranscriptFile(t, "foo.transcript", content)

	sendFakeResponse(t, fd, sessionRespWithTab("TRANSCRIPT", path))

	r := authedGet("/api/sessions/abc/transcript?offset=0")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body %q)", w.Code, w.Body.String())
	}
	if w.Body.String() != content {
		t.Errorf("body = %q, want %q", w.Body.String(), content)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("ETag header missing")
	}
	wantETag := fmt.Sprintf(`"abc:%d"`, len(content))
	if etag != wantETag {
		t.Errorf("ETag = %q, want %q", etag, wantETag)
	}
}

// --- TestTranscript_RejectsTraversal ---

// TestTranscript_RejectsTraversal verifies that session IDs containing path
// traversal characters are rejected with 400 via the allowlist regex (ADR 0026).
// The handler is invoked directly with SetPathValue to bypass ServeMux routing,
// as ServeMux would not route such patterns here at all (defense-in-depth).
func TestTranscript_RejectsTraversal(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	handler := handleGetTranscript(d)

	for _, badID := range []string{"../evil", "bad/id", "foo bar", "id\x00null"} {
		t.Run(badID, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/sessions/x/transcript", nil)
			r.SetPathValue("id", badID)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("id=%q: want 400, got %d", badID, w.Code)
			}
		})
	}
}

// --- TestTranscript_ReturnsEmptyRangeAs204 ---

// TestTranscript_ReturnsEmptyRangeAs204 verifies that when offset equals the
// file size, the handler returns 204 No Content.
func TestTranscript_ReturnsEmptyRangeAs204(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	content := "hello"
	path := makeTranscriptFile(t, "foo.transcript", content)
	size := len(content) // 5

	sendFakeResponse(t, fd, sessionRespWithTab("TRANSCRIPT", path))

	r := authedGet(fmt.Sprintf("/api/sessions/abc/transcript?offset=%d", size))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204 for offset==size, got %d (body %q)", w.Code, w.Body.String())
	}
}

// --- TestTranscript_HonorsIfNoneMatch ---

// TestTranscript_HonorsIfNoneMatch verifies that a matching ETag in the
// If-None-Match request header elicits a 304 Not Modified response.
func TestTranscript_HonorsIfNoneMatch(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	content := "some content"
	path := makeTranscriptFile(t, "foo.transcript", content)

	sendFakeResponse(t, fd, sessionRespWithTab("TRANSCRIPT", path))

	expectedETag := fmt.Sprintf(`"abc:%d"`, len(content))
	r := authedGet("/api/sessions/abc/transcript?offset=0")
	r.Header.Set("If-None-Match", expectedETag)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotModified {
		t.Fatalf("want 304 for matching ETag, got %d (body %q)", w.Code, w.Body.String())
	}
}

// --- TestTranscript_404WhenSessionMissing ---

// TestTranscript_404WhenSessionMissing verifies that when the daemon returns
// an empty session list, the handler returns 404.
func TestTranscript_404WhenSessionMissing(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	sendFakeResponse(t, fd, proto.RespSessions{Sessions: nil})

	r := authedGet("/api/sessions/missing/transcript?offset=0")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for missing session, got %d", w.Code)
	}
}

// --- TestTranscript_404WhenLogTabMissing ---

// TestTranscript_404WhenLogTabMissing verifies that when a session exists but
// has no transcript LogTab, the handler returns 404.
func TestTranscript_404WhenLogTabMissing(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	sendFakeResponse(t, fd, proto.RespSessions{
		Sessions: []proto.SessionInfo{
			{ID: "abc", Command: "test", View: stateview.View{}},
		},
	})

	r := authedGet("/api/sessions/abc/transcript?offset=0")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for missing log tab, got %d", w.Code)
	}
}

// --- TestEventLog_Basic ---

// TestEventLog_Basic is a smoke test verifying that the event-log endpoint
// serves a .jsonl file correctly with application/x-ndjson Content-Type.
func TestEventLog_Basic(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	content := `{"event":"test"}` + "\n"
	path := makeTranscriptFile(t, "session.jsonl", content)

	sendFakeResponse(t, fd, sessionRespWithTab("EVENT-LOG", path))

	r := authedGet("/api/sessions/abc/event-log?offset=0")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body %q)", w.Code, w.Body.String())
	}
	if w.Body.String() != content {
		t.Errorf("body = %q, want %q", w.Body.String(), content)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want application/x-ndjson", ct)
	}
}

// --- TestTranscript_ContentTypePlain ---

// TestTranscript_ContentTypePlain verifies that the transcript endpoint uses
// text/plain; charset=utf-8 (not the JSONL type used for event-log).
func TestTranscript_ContentTypePlain(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	content := "line1\nline2\n"
	path := makeTranscriptFile(t, "foo.transcript", content)

	sendFakeResponse(t, fd, sessionRespWithTab("TRANSCRIPT", path))

	r := authedGet("/api/sessions/abc/transcript?offset=0")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", ct)
	}
}

// --- TestTranscript_503WhenDaemonDown ---

// TestTranscript_503WhenDaemonDown verifies that the transcript endpoint returns
// 503 Service Unavailable when the daemon is not reachable, matching the
// contract of the other REST handlers (list/create/delete).
func TestTranscript_503WhenDaemonDown(t *testing.T) {
	t.Parallel()
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, fmt.Errorf("no daemon") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()
	if waitHealth(d, true, 50*time.Millisecond) {
		t.Skip("daemon became healthy unexpectedly")
	}

	mux := NewMux(d, "tok")
	r := authedGet("/api/sessions/abc/transcript")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when daemon is down, got %d (body %q)", w.Code, w.Body.String())
	}
}

// --- TestTranscript_ProtoErrNotFoundMaps404 ---

// TestTranscript_ProtoErrNotFoundMaps404 verifies that a proto ErrNotFound
// from the daemon maps to 404 (not to a generic 500), consistent with other
// REST handlers that pipe errors through handleProtoError.
func TestTranscript_ProtoErrNotFoundMaps404(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	go func() {
		env := fd.recv()
		fd.sendErr(env.ReqID, proto.ErrNotFound, "session not found")
	}()

	r := authedGet("/api/sessions/abc/transcript")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for proto ErrNotFound, got %d (body %q)", w.Code, w.Body.String())
	}
}

// --- TestTranscript_ProtoErrInvalidArgumentMaps400 ---

// TestTranscript_ProtoErrInvalidArgumentMaps400 verifies that a proto
// ErrInvalidArgument from the daemon maps to 400, consistent with handleProtoError.
func TestTranscript_ProtoErrInvalidArgumentMaps400(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	go func() {
		env := fd.recv()
		fd.sendErr(env.ReqID, proto.ErrInvalidArgument, "bad argument")
	}()

	r := authedGet("/api/sessions/abc/transcript")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for proto ErrInvalidArgument, got %d (body %q)", w.Code, w.Body.String())
	}
}

// --- TestTranscript_InvalidOffset ---

// TestTranscript_InvalidOffset verifies that a non-numeric or negative offset
// returns 400 Bad Request.
func TestTranscript_InvalidOffset(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")

	for _, offset := range []string{"abc", "-1", "1.5"} {
		t.Run("offset="+offset, func(t *testing.T) {
			r := authedGet("/api/sessions/abc/transcript?offset=" + offset)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("want 400 for offset=%q, got %d", offset, w.Code)
			}
		})
	}
}

// --- matchLogTab unit tests ---

// TestMatchLogTab_TranscriptByLabel verifies label-based matching for transcripts.
func TestMatchLogTab_TranscriptByLabel(t *testing.T) {
	t.Parallel()
	tabs := []stateview.LogTab{
		{Label: "TRANSCRIPT", Path: "/a/b.transcript", Kind: stateview.TabKindText},
	}
	got := matchLogTab(tabs, "transcript")
	if got != "/a/b.transcript" {
		t.Errorf("got %q, want /a/b.transcript", got)
	}
}

// TestMatchLogTab_EventLogByLabel verifies label-based matching for event-log.
func TestMatchLogTab_EventLogByLabel(t *testing.T) {
	t.Parallel()
	tabs := []stateview.LogTab{
		{Label: "event-log", Path: "/a/session.jsonl", Kind: stateview.TabKindText},
	}
	got := matchLogTab(tabs, "event-log")
	if got != "/a/session.jsonl" {
		t.Errorf("got %q, want /a/session.jsonl", got)
	}
}

// TestMatchLogTab_EventLogByPathSuffix verifies path-suffix fallback matching.
func TestMatchLogTab_EventLogByPathSuffix(t *testing.T) {
	t.Parallel()
	tabs := []stateview.LogTab{
		{Label: "EVENTS", Path: "/a/session.jsonl", Kind: stateview.TabKindText},
	}
	got := matchLogTab(tabs, "event-log")
	if got != "/a/session.jsonl" {
		t.Errorf("got %q, want /a/session.jsonl", got)
	}
}

// TestMatchLogTab_TranscriptByPathSuffix verifies path-suffix fallback for .transcript.
func TestMatchLogTab_TranscriptByPathSuffix(t *testing.T) {
	t.Parallel()
	tabs := []stateview.LogTab{
		{Label: "LOG", Path: "/a/b.transcript", Kind: stateview.TabKindText},
	}
	got := matchLogTab(tabs, "transcript")
	if got != "/a/b.transcript" {
		t.Errorf("got %q, want /a/b.transcript", got)
	}
}

// TestMatchLogTab_NoMatch verifies that an empty string is returned when no
// tabs match.
func TestMatchLogTab_NoMatch(t *testing.T) {
	t.Parallel()
	tabs := []stateview.LogTab{
		{Label: "STDOUT", Path: "/a/stdout.log", Kind: stateview.TabKindText},
	}
	got := matchLogTab(tabs, "transcript")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// TestMatchLogTab_NonTextKindStillMatched verifies that LogTab.Kind is
// NOT used as a hard filter. Claude drivers stamp Kind="transcript" and
// Codex drivers stamp Kind="codex_transcript"; both are non-"text" Kinds.
// matchLogTab must locate the path by label/suffix regardless of Kind so
// the REST handler can serve those drivers (otherwise GET
// /api/sessions/{id}/transcript would 404 for every Claude/Codex session,
// the round-3 review found this as a blocker).
func TestMatchLogTab_NonTextKindStillMatched(t *testing.T) {
	t.Parallel()
	tabs := []stateview.LogTab{
		{Label: "TRANSCRIPT", Path: "/a/b.transcript", Kind: "transcript"},
	}
	got := matchLogTab(tabs, "transcript")
	if got != "/a/b.transcript" {
		t.Errorf("got %q, want %q (Kind must not exclude TRANSCRIPT tabs)", got, "/a/b.transcript")
	}
}

// TestMuxRoutesTranscriptEndpoint smoke-tests that the transcript/event-log
// routes are registered and do not conflict with DELETE /api/sessions/{id}.
func TestMuxRoutesTranscriptEndpoint(t *testing.T) {
	t.Parallel()
	d, fd := newDaemonPair(t)
	mux := NewMux(d, "tok")

	// Session with no transcript tab → 404, not 405 (MethodNotAllowed would
	// indicate the route is not registered at all).
	sendFakeResponse(t, fd, proto.RespSessions{
		Sessions: []proto.SessionInfo{{ID: "x", Command: "sh"}},
	})

	r := authedGet("/api/sessions/x/transcript")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code == http.StatusMethodNotAllowed {
		t.Fatalf("transcript route not registered: got 405 Method Not Allowed")
	}
}
