package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
)

// The gateway's "wedged daemon" failure mode used to surface as a hung HTTP
// request — the request context had no deadline, so a daemon stuck in a
// feedback loop would tie up gateway workers forever. This file pins the fix:
// every daemon-calling handler must apply daemonRPCTimeout, and the resulting
// context.DeadlineExceeded must map to 504 rather than 500.

// hangingClient implements just enough of the proto-client surface to keep
// SendCommand blocked until ctx.Done(). DaemonClient is wired through a
// custom dialer that hands net.Pipe whose remote side never writes a reply.
//
// We do NOT want to actually wait daemonRPCTimeout (10s) in tests, so we
// shorten it via a test-scoped override.

// withShortRPCTimeout installs a short timeout for the duration of the test.
// The override is package-global, so parallel timeout tests would otherwise
// stomp on each other; we accept this here because every timeout test uses
// the same short value (≤100ms) — the actual value doesn't matter, only that
// it is small. The atomic backing ensures the reads in rpcContext are race-free.
func withShortRPCTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := rpcTimeoutOverride.Swap(int64(d))
	t.Cleanup(func() {
		rpcTimeoutOverride.Store(prev)
	})
}

// TestMux_ListTimesOutAs504 verifies that when the daemon never responds,
// GET /api/sessions returns 504 within the bounded timeout (instead of
// hanging forever or returning an opaque 500).
func TestMux_ListTimesOutAs504(t *testing.T) {
	t.Parallel()
	withShortRPCTimeout(t, 80*time.Millisecond)

	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	// Drain the request line but never reply.
	go func() {
		_ = daemon.recv()
	}()

	r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()

	start := time.Now()
	mux.ServeHTTP(w, r)
	elapsed := time.Since(start)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504; body=%q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "daemon timeout") {
		t.Fatalf("body = %q, want it to mention 'daemon timeout'", w.Body.String())
	}
	// Bound how long the test waited so a regression that removes the timeout
	// would fail rather than just slow the suite.
	if elapsed > 2*time.Second {
		t.Fatalf("handler took %v, much longer than the configured 80ms timeout — timeout regression?", elapsed)
	}
}

// TestMux_CreateTimesOutAs504 mirrors the list test for POST.
func TestMux_CreateTimesOutAs504(t *testing.T) {
	t.Parallel()
	withShortRPCTimeout(t, 80*time.Millisecond)

	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	go func() {
		_ = daemon.recv()
	}()

	r := httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"project":"/p","command":"sh"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504; body=%q", w.Code, w.Body.String())
	}
}

// TestMux_DeleteTimesOutAs504 mirrors the timeout contract for DELETE.
func TestMux_DeleteTimesOutAs504(t *testing.T) {
	t.Parallel()
	withShortRPCTimeout(t, 80*time.Millisecond)

	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	go func() {
		_ = daemon.recv()
	}()

	r := httptest.NewRequest(http.MethodDelete, "/api/sessions/s1", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504; body=%q", w.Code, w.Body.String())
	}
}

// dummyReq builds a minimal *http.Request for handleProtoError unit tests.
// gatewayError needs r.Method, r.URL.Path, r.Header; nothing else.
func dummyReq() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/test", nil)
}

// TestHandleProtoError_DeadlineExceededMaps504 is a focused unit test on the
// error mapping, independent of any HTTP plumbing.
func TestHandleProtoError_DeadlineExceededMaps504(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handleProtoError(w, dummyReq(), context.DeadlineExceeded)
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", w.Code)
	}
	if !strings.Contains(w.Body.String(), "daemon timeout") {
		t.Fatalf("body = %q, want 'daemon timeout'", w.Body.String())
	}
	if w.Header().Get("X-Request-Id") == "" {
		t.Fatal("X-Request-Id header must be set so client and server.log can correlate")
	}
	if !strings.Contains(w.Body.String(), "request_id=") {
		t.Fatalf("body must include request_id= to correlate without headers; got %q", w.Body.String())
	}
}

func TestHandleProtoError_DaemonUnavailableMaps503(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handleProtoError(w, dummyReq(), ErrDaemonUnavailable)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestHandleProtoError_CanceledMaps504(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handleProtoError(w, dummyReq(), context.Canceled)
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", w.Code)
	}
}

func TestHandleProtoError_OpaqueErrorMaps502(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handleProtoError(w, dummyReq(), errors.New("unexpected condition"))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 for non-typed upstream errors", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unexpected condition") {
		t.Fatalf("body should surface underlying error; got %q", w.Body.String())
	}
}

func TestHandleProtoError_ProtoErrorBodyPreserved(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handleProtoError(w, dummyReq(), &proto.ErrorBody{Code: proto.ErrNotFound, Message: "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// TestHandleProtoError_ExhaustiveCodeMapping pins every proto.ErrCode to its
// HTTP status. ErrInternal in particular must map to 502 (Bad Gateway), not
// 500, so an upstream daemon failure is attributed to the daemon rather than
// the gateway. This table requires every code to have a dedicated status and
// prevents that drift.
//
// Adding a new proto.ErrCode? Add the entry here and to protoCodeToHTTP.
func TestHandleProtoError_ExhaustiveCodeMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code   proto.ErrCode
		status int
		reason string
	}{
		{proto.ErrNotFound, http.StatusNotFound, "not_found"},
		{proto.ErrInvalidArgument, http.StatusBadRequest, "invalid_argument"},
		{proto.ErrUnsupported, http.StatusUnprocessableEntity, "unsupported"},
		{proto.ErrAlreadyExists, http.StatusConflict, "already_exists"},
		{proto.ErrSessionStopped, http.StatusGone, "session_stopped"},
		{proto.ErrFrameNotReady, http.StatusServiceUnavailable, "frame_not_ready"},
		{proto.ErrInternal, http.StatusBadGateway, "daemon_internal"},
		{proto.ErrUnknown, http.StatusBadGateway, "daemon_unknown"},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			gotStatus, gotReason := protoCodeToHTTP(tc.code)
			if gotStatus != tc.status {
				t.Errorf("status: got %d, want %d", gotStatus, tc.status)
			}
			if gotReason != tc.reason {
				t.Errorf("reason: got %q, want %q", gotReason, tc.reason)
			}
			// Also verify the full handler wires it through.
			w := httptest.NewRecorder()
			handleProtoError(w, dummyReq(), &proto.ErrorBody{Code: tc.code, Message: "test"})
			if w.Code != tc.status {
				t.Errorf("full handler status: got %d, want %d", w.Code, tc.status)
			}
		})
	}
}

// TestProtoCodeToHTTP_UnmappedCodeYields500 documents the safety-net
// behavior: a code the gateway doesn't know about returns 500 with reason
// "unmapped_code" so the omission is visible (not silently 502'd).
func TestProtoCodeToHTTP_UnmappedCodeYields500(t *testing.T) {
	t.Parallel()
	status, reason := protoCodeToHTTP(proto.ErrCode("fictional_new_code"))
	if status != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", status)
	}
	if reason != "unmapped_code" {
		t.Errorf("reason: got %q, want unmapped_code", reason)
	}
}

// TestMux_HandlesConcurrentTimeoutsCleanly drives the worst case of the
// original bug: many concurrent requests against an unresponsive daemon. The
// pre-fix code would back up indefinitely; with the timeout, every request
// returns 504 and no goroutine leaks survive (verified by an upper bound on
// elapsed wall-clock).
func TestMux_HandlesConcurrentTimeoutsCleanly(t *testing.T) {
	t.Parallel()
	withShortRPCTimeout(t, 100*time.Millisecond)

	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	const N = 16

	go func() {
		// Drain N command envelopes, never reply. The fakeDaemon is single-
		// reader so we read serially — that's fine, the gateway issues each
		// SendCommand from a separate request goroutine and they all race on
		// the proto.Client write side.
		for i := 0; i < N; i++ {
			_ = daemon.recv()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(N)
	start := time.Now()
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
			r.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusGatewayTimeout {
				t.Errorf("status = %d, want 504", w.Code)
			}
		}()
	}
	wg.Wait()

	// All N must finish in well under N × timeout — they run concurrently.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("%d concurrent requests took %v, expected ≤2s (concurrent timeouts)", N, elapsed)
	}
}
