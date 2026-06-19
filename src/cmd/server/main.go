// Command server is the agent-reactor backend HTTP/WS gateway. It is a
// browser-facing HTTPS endpoint that fronts a long-lived arc daemon over its
// Unix socket. Sessions live in the daemon; this binary only proxies.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/takezoh/agent-reactor/platform/lib/tlsdev"
	"github.com/takezoh/agent-reactor/platform/socketpath"
	serverweb "github.com/takezoh/agent-reactor/server/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("addr", ":8443", "listen address")
	tokenFlag := flag.String("token", "", "bearer token (generated and printed if empty)")
	certFile := flag.String("tls-cert", "", "TLS certificate file (self-signed if empty)")
	keyFile := flag.String("tls-key", "", "TLS key file")
	insecure := flag.Bool("insecure", false, "serve plain HTTP (no TLS) — local dev only")
	arcSock := flag.String("arc-sock", "", "path to arc daemon Unix socket (overrides $ARC_SOCKET and the default ~/.agent-reactor/arc.sock)")
	flag.Parse()

	token := *tokenFlag
	if token == "" {
		token = randToken()
	}

	sockPath := socketpath.ResolveDaemonSocket(*arcSock, "ARC_SOCKET", "arc.sock")
	daemon := serverweb.NewDaemonClient(sockPath)
	defer daemon.Close()

	mux := http.NewServeMux()
	mux.Handle("/", serverweb.NewMux(daemon, token))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeHealth(w, daemon)
	})

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	scheme := "https"
	if *insecure {
		scheme = "http"
	}
	log.Printf("agent-reactor backend on %s://%s  arc-sock=%s  token=%s", scheme, *addr, sockPath, token)
	if err := tlsdev.Serve(srv, *insecure, *certFile, *keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
