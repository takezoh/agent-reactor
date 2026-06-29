// End-to-end tests against a real server-daemon process. The unit tests under
// mux_daemon_test.go drive the gateway against a net.Pipe fake daemon — that
// covers the wire-level behaviour but cannot catch:
//
//   - regressions in the cmd/server bootstrap (e.g. a startup panic that masks
//     itself as a 500 from /api/sessions),
//   - wedge regressions in the daemon's runtime event loop (the original
//     "internal channel full" feedback loop that wedged the production
//     daemon hosting the user's agent session),
//   - cross-package wiring drift (proto codec changes that break the
//     gateway↔daemon contract but pass each side's unit tests in isolation).
//
// This file spins the actual server binary (built on demand by `go build`)
// under a scratch data dir, then drives /api/sessions through an in-test
// gateway handler against the daemon's socket. The spawned server also
// brings up its co-resident gateway listener; we pin it to an ephemeral
// loopback port so it never conflicts with any other test or the user's
// daemon. Test data dir cleanup happens via t.TempDir(); the daemon is
// killed in t.Cleanup so a panicking test still reaps its child process.
//
// These tests are skipped under `go test -short` because the build step is
// ~5s; the unit-level coverage is unaffected.
package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// serverBinaryCache memoises the on-demand-built server binary path so
// multiple e2e tests within the same `go test` invocation share one build.
var (
	serverBinaryMu   sync.Mutex
	serverBinaryPath string
	serverBinaryErr  error
)

// buildServerOnce builds the server binary under t.TempDir() (shared across
// tests in the same package run) and returns its absolute path. Subsequent
// calls reuse the cached path. Skips the test on platforms where the build
// cannot succeed for environmental reasons (no go toolchain on PATH, etc.).
func buildServerOnce(t *testing.T) string {
	t.Helper()
	serverBinaryMu.Lock()
	defer serverBinaryMu.Unlock()
	if serverBinaryPath != "" || serverBinaryErr != nil {
		if serverBinaryErr != nil {
			t.Skipf("server binary unavailable: %v", serverBinaryErr)
		}
		return serverBinaryPath
	}

	dir, err := os.MkdirTemp("", "server-e2e-bin-")
	if err != nil {
		serverBinaryErr = err
		t.Skipf("mkdir tempdir: %v", err)
	}
	bin := filepath.Join(dir, "server")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/server")
	// Run from the module root (this test file lives at src/server/web/);
	// `go build ./cmd/server` resolves under it.
	cmd.Dir = moduleRoot(t)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		serverBinaryErr = err
		_ = os.RemoveAll(dir)
		t.Skipf("go build cmd/server failed: %v\nstderr:\n%s", err, stderr.String())
	}
	serverBinaryPath = bin
	return serverBinaryPath
}

// moduleRoot returns the directory containing go.mod. The tests live three
// levels deep (src/server/web/), so we walk up until go.mod is found rather
// than hard-coding "../../".
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod walking up from %s", dir)
		}
		dir = parent
	}
}

// startServerDaemon launches the server binary against a scratch data dir,
// waits for the IPC socket to appear, and returns its absolute path. The
// server's co-resident HTTP/WS gateway is pinned to an ephemeral loopback
// port (-no-auth -insecure -addr 127.0.0.1:0) so it never conflicts with
// another listener; the test does not exercise it directly — instead it
// drives its own in-test mux against the daemon socket. Killed at the end
// of the test via t.Cleanup so a panicking test still reaps the child.
func startServerDaemon(t *testing.T) string {
	t.Helper()
	bin := buildServerOnce(t)
	dataDir := t.TempDir()

	cmd := exec.Command(bin,
		"-data-dir", dataDir,
		"-insecure",
		"-no-auth",
		"-addr", "127.0.0.1:0",
	)
	cmd.Env = os.Environ()
	logFile, err := os.Create(filepath.Join(dataDir, "server.log"))
	if err != nil {
		t.Fatalf("create log: %v", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		_ = logFile.Close()
	})

	sockPath := filepath.Join(dataDir, "server.sock")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fi, err := os.Stat(sockPath); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return sockPath
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Daemon never bound the socket — surface the log so a reader can debug.
	tail, _ := os.ReadFile(filepath.Join(dataDir, "server.log"))
	t.Fatalf("daemon did not bind %s within 5s. log tail:\n%s", sockPath, string(tail))
	return ""
}

// withRealDaemonMux is the entry point for tests that need a fully-wired
// (gateway → real daemon) stack. Returns the *http.ServeMux configured
// against a DaemonClient connected to a freshly-spawned server daemon under
// a scratch data dir. All teardown (daemon process kill, socket cleanup) is
// registered on t.Cleanup so a panicking test still releases resources.
func withRealDaemonMux(t *testing.T) http.Handler {
	t.Helper()
	sock := startServerDaemon(t)
	d := NewDaemonClient(sock)
	t.Cleanup(d.Close)
	if !waitHealth(d, true, 3*time.Second) {
		t.Fatalf("DaemonClient never became healthy against real daemon at %s", sock)
	}
	return NewMux(d, "tok")
}

// TestE2E_ListEmptyAtStartup exercises the path the user's complaint pinned
// (GET /api/sessions returned 500). With an isolated, freshly-launched
// daemon, the response must be 200 + empty array — no fake daemons,
// no in-process plumbing shortcuts.
func TestE2E_ListEmptyAtStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon e2e in -short mode")
	}
	t.Parallel()

	mux := withRealDaemonMux(t)

	r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", w.Code, w.Body.String())
	}
	body, _ := io.ReadAll(w.Body)
	if string(bytes.TrimSpace(body)) != "[]" {
		t.Fatalf("body = %q, want \"[]\"", string(body))
	}
}

// TestE2E_CreateThenListRoundtrip verifies the full create→list flow against
// a real daemon. A successful POST must result in the new session appearing
// in the subsequent GET — proves the daemon is actually persisting state,
// not just acknowledging the command.
func TestE2E_CreateThenListRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon e2e in -short mode")
	}
	t.Parallel()

	mux := withRealDaemonMux(t)

	// Create.
	dir := t.TempDir()
	body := `{"project":"` + dir + `","command":"shell"}`
	rc := httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewBufferString(body))
	rc.Header.Set("Authorization", "Bearer tok")
	rc.Header.Set("Content-Type", "application/json")
	wc := httptest.NewRecorder()
	mux.ServeHTTP(wc, rc)
	if wc.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%q", wc.Code, wc.Body.String())
	}
	var created apiSessionInfo
	if err := json.Unmarshal(wc.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create resp: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create returned empty ID")
	}

	// List — give the daemon a brief moment to publish the session through
	// its event loop. The response is synchronous so usually one read suffices,
	// but pty spawn is async; poll for up to 2s before failing.
	deadline := time.Now().Add(2 * time.Second)
	for {
		rl := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
		rl.Header.Set("Authorization", "Bearer tok")
		wl := httptest.NewRecorder()
		mux.ServeHTTP(wl, rl)
		if wl.Code != http.StatusOK {
			t.Fatalf("list status = %d, want 200; body=%q", wl.Code, wl.Body.String())
		}
		var list []apiSessionInfo
		if err := json.Unmarshal(wl.Body.Bytes(), &list); err != nil {
			t.Fatalf("decode list resp: %v", err)
		}
		for _, s := range list {
			if s.ID == created.ID {
				return // pass
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("created session %q not in list after 2s; list=%s",
				created.ID, wl.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestE2E_DeleteUnknownSessionReturns404 verifies the error mapping against a
// real daemon: an unknown session ID must surface as 404 (not 500). Pre-fix,
// the gateway mapped some error paths to 500 unconditionally — this pins the
// proto.ErrNotFound → 404 wiring at the integration boundary.
func TestE2E_DeleteUnknownSessionReturns404(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon e2e in -short mode")
	}
	t.Parallel()

	mux := withRealDaemonMux(t)

	r := httptest.NewRequest(http.MethodDelete, "/api/sessions/no-such-session-id", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %q)", w.Code, w.Body.String())
	}
}

// TestE2E_CreateRejectsRelativeProject pins the gateway-side guard added
// after the user's "the working directory 'bash' is invalid" 502 incident.
// A non-absolute project path used to slip past the gateway, reach the
// devcontainer launcher, and surface as a 502 with a docker-internals
// message. The gateway now rejects it as 400 with the rule stated.
func TestE2E_CreateRejectsRelativeProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon e2e in -short mode")
	}
	t.Parallel()

	mux := withRealDaemonMux(t)

	// "bash" is exactly what the user typed when a session was killed by
	// the resulting devcontainer error — pin that specific input so a
	// regression that re-allows relative paths surfaces here first.
	for _, p := range []string{"bash", "claude", "myproject", "./relative", "foo/bar"} {
		t.Run(p, func(t *testing.T) {
			body := `{"project":"` + p + `","command":"shell"}`
			r := httptest.NewRequest(http.MethodPost, "/api/sessions",
				bytes.NewBufferString(body))
			r.Header.Set("Authorization", "Bearer tok")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("project=%q: status = %d, want 400 (body %q)", p, w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("absolute path")) {
				t.Errorf("project=%q: body should mention 'absolute path' rule; got %q", p, w.Body.String())
			}
		})
	}
}

// TestE2E_CreateRejectsMissingProject verifies that the daemon-level reducer
// validation (project arg required) maps cleanly to 400, not 500. The user's
// 500 complaint motivated tightening this path — the gateway must never
// surface an "internal error" code for a request that should be a 400.
func TestE2E_CreateRejectsMissingProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon e2e in -short mode")
	}
	t.Parallel()

	mux := withRealDaemonMux(t)

	r := httptest.NewRequest(http.MethodPost, "/api/sessions",
		bytes.NewBufferString(`{"command":"shell"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
}

// TestE2E_CreateRejectsMalformedJSON pins the http-level 400 for body parse
// failures. The handler must not even attempt a daemon RPC.
func TestE2E_CreateRejectsMalformedJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon e2e in -short mode")
	}
	t.Parallel()

	mux := withRealDaemonMux(t)

	r := httptest.NewRequest(http.MethodPost, "/api/sessions",
		bytes.NewBufferString(`{not json`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
}

// TestE2E_HostileSessionIDRejectedAt400 verifies the ADR 0026 allowlist on
// path params: anything outside [a-zA-Z0-9_-]+ must be rejected before the
// daemon RPC. We go through a real httptest.NewServer so URL escaping
// matches what a browser actually sends, and we cover characters that
// don't trip net/http's ServeMux cleanup of "../" (already a 301 redirect
// outside the allowlist's reach) or panic httptest.NewRequest on spaces.
func TestE2E_HostileSessionIDRejectedAt400(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-daemon e2e in -short mode")
	}
	t.Parallel()

	mux := withRealDaemonMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Stay within "characters that survive HTTP URL parsing but are blocked
	// by our allowlist" — leaving traversal sequences and shell injection to
	// the unit-level allowlist tests (transcript_test.go) where they can be
	// asserted without router quirks.
	cases := []string{
		"abc.def",   // dot — not in [a-zA-Z0-9_-]
		"abc%21def", // url-encoded '!' → '!' is not in allowlist
		"AbC$123",   // dollar sign
	}
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/"+id, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer tok")
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Allowlist failures map to 400. Anything else means the request
			// reached the daemon (which would 404 for missing sessions) —
			// also undesirable for hostile input but at least not a 5xx.
			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("hostile id %q: status = %d, want 400 (body %q)", id, resp.StatusCode, string(body))
			}
		})
	}
}
