package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/takezoh/agent-reactor/platform/lib/tlsdev"
	serverweb "github.com/takezoh/agent-reactor/server/web"
)

// shutdownGracePeriod bounds the time gateway.Shutdown waits for in-flight
// requests to drain before forcing the listener closed. Used by both the
// belt-and-braces Close() path on the daemon's exit and by the ctx-cancel
// goroutine that watches the shared daemon context.
const shutdownGracePeriod = 5 * time.Second

// gatewayHandle keeps the resources the gateway goroutine owns so the daemon
// can reap them on shutdown.
type gatewayHandle struct {
	daemon *serverweb.DaemonClient
	srv    *http.Server
	done   <-chan struct{}
}

// Close shuts the gateway HTTP server down (best-effort shutdownGracePeriod
// grace) and closes the in-process DaemonClient. Safe to call once; the
// goroutine that bound the listener watches ctx and tears down on cancel as
// well, so Close is a belt-and-braces hook for the deferred path on the
// daemon's exit.
func (g *gatewayHandle) Close() {
	if g == nil {
		return
	}
	if g.srv != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		_ = g.srv.Shutdown(shutCtx)
		cancel()
	}
	if g.done != nil {
		<-g.done
	}
	if g.daemon != nil {
		g.daemon.Close()
	}
}

// startGateway boots the co-resident HTTP/WS gateway as a goroutine that dials
// the daemon's IPC socket. The gateway shares the daemon context, so signal-
// driven shutdown cascades into it. A panic inside the gateway goroutine is
// contained by recover() — it cancels the daemon ctx so the binary exits, but
// the recovered stack is logged rather than dropped into the void.
func startGateway(ctx context.Context, cancel context.CancelFunc, sockPath string, df *daemonFlagSet) (*gatewayHandle, error) {
	token, err := resolveAuth(df.token, df.tokenFile, df.noAuth, df.addr)
	if err != nil {
		return nil, err
	}
	daemon := serverweb.NewDaemonClient(sockPath)
	srv := &http.Server{
		Addr:              df.addr,
		Handler:           buildHTTPHandler(daemon, token, df.noAuth),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Listener has to bind synchronously so a port conflict (or TLS cert error)
	// is reported BEFORE startGateway returns nil. tlsdev.Serve calls
	// ListenAndServe internally, which blocks; so we open the listener here
	// and hand it to a custom Serve loop on a goroutine.
	ln, err := net.Listen("tcp", df.addr)
	if err != nil {
		daemon.Close()
		return nil, fmt.Errorf("listen %s: %w", df.addr, err)
	}

	doneCh := make(chan struct{})

	go func() {
		<-ctx.Done()
		grace, gcancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer gcancel()
		_ = srv.Shutdown(grace)
	}()

	go func() {
		defer close(doneCh)
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("gateway: goroutine panicked",
					"err", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()))
				cancel()
			}
		}()
		logStartup(df.addr, df.insecure, df.noAuth, sockPath, token)
		serveErr := tlsdev.ServeListener(srv, ln, df.insecure, df.certFile, df.keyFile)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("gateway: serve failed", "err", serveErr)
			cancel()
		}
	}()

	return &gatewayHandle{daemon: daemon, srv: srv, done: doneCh}, nil
}

// resolveAuth materializes the effective bearer token. -no-auth wins and
// forces the token to "" (the gateway-side TokenAuth contract treats "" as
// "reject everything", which is why no-auth mounts the API handler directly
// instead of routing it through TokenAuth). Without -no-auth, an empty user
// token is replaced with a freshly minted random one. -no-auth additionally
// refuses non-loopback binds to keep the unauthenticated REST surface
// off-network.
//
// Precedence (highest first): -no-auth > -token > -token-file > random.
// -token and -token-file are mutually exclusive: silent precedence between
// "value flag" and "file flag" hides operator intent (was the literal stale,
// or did the file path typo?). Surface that ambiguity BEFORE the -no-auth
// short-circuit so a misconfig is reported now, not the moment an operator
// later removes -no-auth and the hidden conflict suddenly becomes fatal.
func resolveAuth(tokenFlag, tokenFile string, noAuth bool, addr string) (string, error) {
	if tokenFlag != "" && tokenFile != "" {
		return "", errors.New("-token and -token-file are mutually exclusive; pick one")
	}
	if noAuth {
		if !isLoopbackAddr(addr) {
			return "", fmt.Errorf("-no-auth refuses non-loopback bind %q (use 127.0.0.1:<port> or localhost:<port>)", addr)
		}
		return "", nil
	}
	if tokenFlag != "" {
		return tokenFlag, nil
	}
	if tokenFile != "" {
		return tokenFromFile(tokenFile)
	}
	return randToken(), nil
}

// tokenFromFile reads the bearer token from path, or generates and persists a
// fresh one if the file is absent or empty. Persisted form is the raw hex
// token followed by a newline so cat / sed-friendly tools see a sensible value.
//
// Persistence uses write-tmp-then-rename to guarantee atomicity: a crash
// between truncate and write would otherwise leave a zero-byte file, which
// this same function treats as "regenerate" on next boot — silently rotating
// the token and invalidating every bookmarked browser URL the systemd unit
// guide promises will survive restarts. The tmp file is chmod'd 0600 before
// rename, which also forces tight permissions on a pre-existing target path
// that an operator might have created with looser permissions (os.WriteFile
// inherits the existing file's mode and would otherwise leak the secret).
func tokenFromFile(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		if tok := strings.TrimSpace(string(data)); tok != "" {
			// Best-effort tighten in case the file pre-existed at a looser
			// mode (e.g. 0644 from a manual edit). Failure here is not
			// fatal — the token is still functional; leaving the warning
			// to journald would just add noise on every boot.
			_ = os.Chmod(path, 0o600)
			return tok, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read token file %q: %w", path, err)
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("mkdir parent of token file %q: %w", path, err)
		}
	}
	tok := randToken()
	return tok, writeTokenAtomic(path, tok)
}

// writeTokenAtomic writes "<tok>\n" to path via a temp file in the same
// directory followed by an atomic rename. The temp file is chmod'd 0600
// before rename so even a brief intermediate exposure window is at the
// final mode. On any error mid-flight the temp file is removed so a
// half-written secret does not linger.
func writeTokenAtomic(path, tok string) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("create temp for token file %q: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.WriteString(tok + "\n"); err != nil {
		cleanup()
		return fmt.Errorf("write temp token file %q: %w", tmpName, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp token file %q: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp token file %q: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp token file %q to %q: %w", tmpName, path, err)
	}
	return nil
}

// buildHTTPHandler picks the appropriate mux variant and bolts on /healthz.
// no-auth mode goes through NewMuxNoAuth, which mounts apiHandler directly
// (no TokenAuth wrap) and skips the WS-ticket consume check.
func buildHTTPHandler(daemon *serverweb.DaemonClient, token string, noAuth bool) http.Handler {
	mux := http.NewServeMux()
	if noAuth {
		mux.Handle("/", serverweb.NewMuxNoAuth(daemon))
	} else {
		mux.Handle("/", serverweb.NewMux(daemon, token))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeHealth(w, daemon)
	})
	return mux
}

func logStartup(addr string, insecure, noAuth bool, sockPath, token string) {
	scheme := "https"
	if insecure {
		scheme = "http"
	}
	authDesc := "token=" + token
	if noAuth {
		authDesc = "auth=disabled"
	}
	slog.Info("gateway listening",
		"url", fmt.Sprintf("%s://%s", scheme, addr),
		"sock", sockPath,
		"auth", authDesc)
	if noAuth {
		slog.Warn("gateway: -no-auth — bearer-token and WS-ticket checks are disabled. " +
			"Anyone reaching this loopback port can drive every session.")
	}
}

func writeHealth(w http.ResponseWriter, d *serverweb.DaemonClient) {
	healthy := d.Health()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	status := "ok"
	if !healthy {
		status = "daemon-unavailable"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          status,
		"last_attempt_at": d.LastAttemptAt(),
	})
}

func randToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// isLoopbackAddr reports whether listenAddr binds to a loopback interface.
// Used as a guardrail for -no-auth: a non-loopback bind would expose the
// authenticated REST surface to anyone on the network.
// Accepts:  "127.0.0.1:8443", "[::1]:8443", "localhost:8443", "127.0.0.1"
// Rejects:  ":8443" (wildcard), "0.0.0.0:8443", "192.168.1.5:8443"
func isLoopbackAddr(listenAddr string) bool {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		// SplitHostPort fails on a bare host with no port. Treat the input as
		// a host literal in that case.
		host = listenAddr
	}
	if host == "" {
		return false // ":8443" form binds the wildcard — explicitly unsafe.
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
