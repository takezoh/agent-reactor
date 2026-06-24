// Command server is the agent-reactor backend HTTP/WS gateway. It is a
// browser-facing HTTPS endpoint that fronts a long-lived arc daemon over its
// Unix socket. Sessions live in the daemon; this binary only proxies.
//
// # Process isolation contract
//
// A gateway-induced bug (a wedged daemon, a runaway feedback loop, a panic)
// can cascade to every session inside the daemon it dials — including the
// user's TUI session if those two share state. To make that cross-talk
// physically impossible, the gateway runs in one of two modes:
//
//  1. Spawn mode (-data-dir <path>, RECOMMENDED): the gateway forks its own
//     arc daemon under <path>, owns its lifecycle (SIGTERM/SIGKILL on
//     gateway shutdown), and dials the auto-generated socket. The TUI's
//     daemon (under $HOME/.agent-reactor) is untouched; even if the
//     gateway's daemon wedges or panics, nothing the user owns is affected.
//
//  2. Attach mode (-arc-sock <path>): the gateway dials an existing daemon
//     the operator chose. This is the "I know what I'm doing" knob — useful
//     for orchestration tools that run their own daemon supervisor. The
//     gateway refuses to attach to $HOME/.agent-reactor/arc.sock (the TUI
//     default) unless ARC_ALLOW_SHARED_DAEMON=1 is explicitly set.
//
// The two modes are mutually exclusive. Neither set is an error — there is
// no implicit default; previous versions silently fell back to
// $HOME/.agent-reactor/arc.sock and that fallback was the direct cause of
// the "POST /api/sessions killed my TUI session" incident.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/takezoh/agent-reactor/platform/lib/tlsdev"
	serverweb "github.com/takezoh/agent-reactor/server/web"
)

// daemonReadyTimeoutOverride is a test hook (nanoseconds) for the spawn
// socket-wait. Zero means "use daemonReadyTimeout". Production never writes
// it — it has no flag wiring. atomic.Int64 because spawn-mode tests run in
// parallel and the production path reads concurrently.
var daemonReadyTimeoutOverride atomic.Int64

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// daemonReadyTimeout caps how long we wait for a freshly-spawned arc daemon
// to bind its Unix socket. 10s is generous on slow disks; on a developer
// machine the socket appears in <500ms.
const daemonReadyTimeout = 10 * time.Second

// daemonShutdownTimeout caps how long we wait for the spawned arc daemon to
// exit cleanly after SIGTERM before escalating to SIGKILL. The daemon's own
// signal handler should reach Stop() in well under a second; 5s leaves
// generous headroom for sessions-snapshot flush.
const daemonShutdownTimeout = 5 * time.Second

func run() error {
	addr := flag.String("addr", ":8443", "listen address")
	tokenFlag := flag.String("token", "", "bearer token (generated and printed if empty); ignored with -no-auth")
	certFile := flag.String("tls-cert", "", "TLS certificate file (self-signed if empty)")
	keyFile := flag.String("tls-key", "", "TLS key file")
	insecure := flag.Bool("insecure", false, "serve plain HTTP (no TLS) — local dev only")
	noAuth := flag.Bool("no-auth", false,
		"disable bearer-token AND WS-ticket auth — local dev only (loopback only). "+
			"Bind MUST be 127.0.0.1/localhost; refuses non-loopback addrs.")
	arcSock := flag.String("arc-sock", "",
		"ATTACH MODE: path to an existing arc daemon Unix socket. "+
			"Mutually exclusive with -data-dir. Refuses $HOME/.agent-reactor/arc.sock "+
			"unless ARC_ALLOW_SHARED_DAEMON=1 (the TUI default path; sharing it would "+
			"propagate gateway wedges to every TUI session).")
	dataDir := flag.String("data-dir", "",
		"SPAWN MODE: gateway forks its own arc daemon under this directory and "+
			"owns its lifecycle. Mutually exclusive with -arc-sock. Refuses "+
			"$HOME/.agent-reactor (would collide with the TUI daemon). RECOMMENDED.")
	arcBin := flag.String("arc-bin", "",
		"SPAWN MODE: path to the arc binary. Defaults to ./arc next to this "+
			"binary, falling back to PATH lookup. Only consulted with -data-dir.")
	flag.Parse()

	token, err := resolveAuth(*tokenFlag, *noAuth, *addr)
	if err != nil {
		return err
	}

	sockPath, child, err := resolveDaemon(*arcSock, *dataDir, *arcBin, os.Getenv("ARC_SOCKET"))
	if err != nil {
		return err
	}
	// child may be nil in attach mode; the cleanup func is always non-nil.
	defer child.shutdown()

	daemon := serverweb.NewDaemonClient(sockPath)
	defer daemon.Close()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           buildHTTPHandler(daemon, token, *noAuth),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	logStartup(*addr, *insecure, *noAuth, sockPath, token, child.mode())
	if err := tlsdev.Serve(srv, *insecure, *certFile, *keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// resolveAuth materializes the effective bearer token. -no-auth wins and
// forces the token to "" (the gateway-side TokenAuth contract treats "" as
// "reject everything", which is why no-auth mounts the API handler directly
// instead of routing it through TokenAuth). Without -no-auth, an empty user
// token is replaced with a freshly minted random one. -no-auth additionally
// refuses non-loopback binds to keep the unauthenticated REST surface
// off-network.
func resolveAuth(tokenFlag string, noAuth bool, addr string) (string, error) {
	if noAuth {
		if !isLoopbackAddr(addr) {
			return "", fmt.Errorf("-no-auth refuses non-loopback bind %q (use 127.0.0.1:<port> or localhost:<port>)", addr)
		}
		return "", nil
	}
	if tokenFlag == "" {
		return randToken(), nil
	}
	return tokenFlag, nil
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

func logStartup(addr string, insecure, noAuth bool, sockPath, token, mode string) {
	scheme := "https"
	if insecure {
		scheme = "http"
	}
	authDesc := "token=" + token
	if noAuth {
		authDesc = "auth=disabled"
	}
	log.Printf("agent-reactor backend on %s://%s  arc-sock=%s  %s  mode=%s",
		scheme, addr, sockPath, authDesc, mode)
	if noAuth {
		log.Printf("WARNING: -no-auth — bearer-token and WS-ticket checks are disabled. " +
			"Anyone reaching this loopback port can drive every arc session. " +
			"Local dev only; never expose this listener off-host.")
	}
	if os.Getenv("ARC_ALLOW_SHARED_DAEMON") == "1" {
		log.Printf("WARNING: ARC_ALLOW_SHARED_DAEMON=1 — gateway is attached to a shared daemon; " +
			"a gateway-induced wedge will affect every session inside it.")
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

// daemonHandle owns the spawned arc child (if any) so the gateway can reap
// it on shutdown. In attach mode (no spawn), all methods are no-ops — a
// nil-safe wrapper avoids littering the main flow with conditionals.
type daemonHandle struct {
	cmd     *exec.Cmd // nil in attach mode
	logFile *os.File  // nil in attach mode
	dataDir string    // empty in attach mode
	once    sync.Once
}

func (h *daemonHandle) mode() string {
	if h.cmd == nil {
		return "attach"
	}
	return "spawn"
}

// shutdown terminates the spawned arc daemon. Idempotent. SIGTERM first;
// SIGKILL after daemonShutdownTimeout. Always closes the log file. Logs an
// error if the daemon exited non-zero so a developer sees it on the gateway's
// stderr without having to tail the daemon log separately.
func (h *daemonHandle) shutdown() {
	h.once.Do(func() {
		if h.cmd == nil || h.cmd.Process == nil {
			if h.logFile != nil {
				_ = h.logFile.Close()
			}
			return
		}
		// SIGTERM gives the daemon a chance to flush sessions.json.
		_ = h.cmd.Process.Signal(syscall.SIGTERM)
		exit := make(chan error, 1)
		go func() { exit <- h.cmd.Wait() }()
		select {
		case err := <-exit:
			if err != nil {
				log.Printf("agent-reactor: spawned arc exited: %v (log: %s)",
					err, filepath.Join(h.dataDir, "arc.log"))
			}
		case <-time.After(daemonShutdownTimeout):
			log.Printf("agent-reactor: spawned arc did not exit within %s, sending SIGKILL",
				daemonShutdownTimeout)
			_ = h.cmd.Process.Kill()
			<-exit // reap
		}
		if h.logFile != nil {
			_ = h.logFile.Close()
		}
	})
}

// resolveDaemon picks the gateway's daemon-source mode and returns the
// socket path + a handle whose shutdown() must be deferred by the caller.
//
// Mode selection:
//   - dataDir non-empty → spawn mode. arcSock and envSock must be empty
//     (mixing modes is an explicit error: ambiguity here was the root of
//     the original "shared daemon" incident).
//   - arcSock non-empty OR envSock non-empty → attach mode.
//   - all empty → error (the gateway refuses to start with no daemon
//     reference; no implicit default).
func resolveDaemon(arcSock, dataDir, arcBin, envSock string) (string, *daemonHandle, error) {
	switch {
	case dataDir != "" && (arcSock != "" || envSock != ""):
		return "", &daemonHandle{}, errors.New(
			"cmd/server: -data-dir and -arc-sock (or $ARC_SOCKET) are mutually exclusive; " +
				"-data-dir spawns its own daemon under the given dir, -arc-sock attaches to an existing one")
	case dataDir != "":
		return spawnMode(dataDir, arcBin)
	default:
		sock, err := resolveSocket(arcSock, envSock)
		return sock, &daemonHandle{}, err
	}
}

// spawnMode launches a dedicated arc daemon under dataDir and returns its
// socket path plus a handle that will reap the daemon on gateway shutdown.
//
// Refuses to use $HOME/.agent-reactor as dataDir — that is the TUI's data
// dir and spawning a second daemon there would race the TUI's pidfile lock
// (and corrupt its sessions snapshot if the lock somehow allowed it). Any
// other path, including /tmp and explicit user-chosen dirs, is fine.
func spawnMode(dataDir, arcBin string) (string, *daemonHandle, error) {
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", &daemonHandle{}, fmt.Errorf("cmd/server: -data-dir %q: %w", dataDir, err)
	}
	if isSharedDataDir(absDataDir) {
		return "", &daemonHandle{}, fmt.Errorf(
			"cmd/server: refusing to spawn arc in %q because that is the TUI's data "+
				"dir; pick a different -data-dir (e.g. /tmp/agent-reactor-web) so the "+
				"two daemons do not race the same sessions.json / pidfile lock", absDataDir)
	}
	if err := os.MkdirAll(absDataDir, 0o700); err != nil {
		return "", &daemonHandle{}, fmt.Errorf("cmd/server: mkdir %q: %w", absDataDir, err)
	}

	bin, err := resolveArcBinary(arcBin)
	if err != nil {
		return "", &daemonHandle{}, err
	}

	logPath := filepath.Join(absDataDir, "arc.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", &daemonHandle{}, fmt.Errorf("cmd/server: open %s: %w", logPath, err)
	}

	// We deliberately do NOT call Setpgid: keeping the spawned arc in the
	// same process group means Ctrl-C in an interactive shell signals both
	// at once, AND the gateway's signal.Notify will run shutdown() to reap
	// it cleanly. Either path leads to a clean exit; no double-handling.
	cmd := exec.Command(bin) //nolint:gosec // bin is operator-supplied flag, not user input
	cmd.Env = append(os.Environ(), "ROOST_DATA_DIR="+absDataDir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return "", &daemonHandle{}, fmt.Errorf("cmd/server: start %s: %w", bin, err)
	}

	handle := &daemonHandle{cmd: cmd, logFile: logFile, dataDir: absDataDir}
	sockPath := filepath.Join(absDataDir, "arc.sock")
	timeout := daemonReadyTimeout
	if override := time.Duration(daemonReadyTimeoutOverride.Load()); override > 0 {
		timeout = override
	}
	if err := waitForSocket(sockPath, timeout); err != nil {
		// Reap the partially-started child before bubbling up.
		handle.shutdown()
		return "", &daemonHandle{}, fmt.Errorf(
			"cmd/server: spawned arc did not bind %s within %s (log: %s): %w",
			sockPath, timeout, logPath, err)
	}
	log.Printf("agent-reactor: spawned arc pid=%d data-dir=%s sock=%s",
		cmd.Process.Pid, absDataDir, sockPath)
	return sockPath, handle, nil
}

// resolveArcBinary returns an absolute path to the arc binary the gateway
// should fork. Precedence: explicit flag → same dir as this server binary
// → $PATH. If nothing is found, returns an actionable error.
func resolveArcBinary(flagVal string) (string, error) {
	if v := strings.TrimSpace(flagVal); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", fmt.Errorf("cmd/server: -arc-bin %q: %w", v, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("cmd/server: -arc-bin %q not found: %w", abs, err)
		}
		return abs, nil
	}
	// Try next to this server binary first — handles `make build` layout
	// (./server and ./arc side-by-side under the repo root).
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), "arc")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	// Fall back to PATH.
	if found, err := exec.LookPath("arc"); err == nil {
		return found, nil
	}
	return "", errors.New(
		"cmd/server: arc binary not found next to ./server or in $PATH; " +
			"pass -arc-bin <path> explicitly")
}

// waitForSocket polls until path is a Unix domain socket or the deadline
// elapses. Used after spawning the arc daemon — we cannot dial until the
// socket is bound, but the daemon's bootstrap takes a few hundred ms.
func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if fi, err := os.Stat(path); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// resolveSocket returns the arc daemon socket path the gateway should dial
// in ATTACH MODE. Spawn mode uses spawnMode() instead. See the "Process
// isolation contract" section at the top of this file.
//
// Precedence: flag value > env value. Both empty is an error — the user
// must be explicit about which daemon they are attaching to.
//
// As a final safety net, if the resolved path lives directly under
// $HOME/.agent-reactor (the well-known default location for the user's TUI
// arc daemon), reject it unless ARC_ALLOW_SHARED_DAEMON=1 is set.
func resolveSocket(flagVal, envVal string) (string, error) {
	v := strings.TrimSpace(flagVal)
	if v == "" {
		v = strings.TrimSpace(envVal)
	}
	if v == "" {
		return "", errors.New(
			"cmd/server: must specify either -data-dir <path> (spawn mode, RECOMMENDED) " +
				"or -arc-sock <path> / $ARC_SOCKET (attach mode); no implicit default — " +
				"the previous fallback to $HOME/.agent-reactor/arc.sock was the direct " +
				"cause of the 'POST /api/sessions killed my TUI session' incident")
	}
	if isSharedDaemonPath(v) && os.Getenv("ARC_ALLOW_SHARED_DAEMON") != "1" {
		return "", fmt.Errorf(
			"cmd/server: refusing to attach to %q because that path is the user's "+
				"shared arc daemon (TUI); a gateway-induced wedge would cascade "+
				"to every session inside it (e.g. the agent session that "+
				"launched this gateway); pass -data-dir <path> to spawn an isolated "+
				"daemon instead, or set ARC_ALLOW_SHARED_DAEMON=1 to override", v)
	}
	return v, nil
}

// isSharedDaemonPath reports whether path is the canonical TUI arc daemon
// socket — i.e. $HOME/.agent-reactor/arc.sock. Any other path (including
// scratch dirs under /tmp, /opt, etc.) is treated as gateway-owned and safe.
func isSharedDaemonPath(path string) bool {
	return matchesTUIDefault(path, "arc.sock")
}

// isSharedDataDir reports whether dir is the canonical TUI data dir
// ($HOME/.agent-reactor). Spawning arc there would collide with the TUI's
// pidfile lock and corrupt its sessions snapshot.
func isSharedDataDir(dir string) bool {
	return matchesTUIDefault(dir, "")
}

// matchesTUIDefault returns whether candidate resolves to the canonical TUI
// path $HOME/.agent-reactor/<suffix> (or just $HOME/.agent-reactor when
// suffix is empty).
//
// Returns false when:
//   - HOME is unavailable (missing HOME never causes the safety check to
//     misfire), or
//   - candidate is empty (filepath.Abs("") would resolve to the current
//     working directory; if a future caller passes "" while the cwd happens
//     to be $HOME/.agent-reactor, the safety check would yield a false
//     positive. Reject empty up-front so the helper is safe to call from
//     any context.)
func matchesTUIDefault(candidate, suffix string) bool {
	if candidate == "" {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	canonical := filepath.Join(home, ".agent-reactor")
	if suffix != "" {
		canonical = filepath.Join(canonical, suffix)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		abs = candidate
	}
	return abs == canonical
}
