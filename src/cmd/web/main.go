// Command web is the agent-reactor web-client host: it serves the embedded
// browser UI (xterm.js) and reverse-proxies the data plane (/api, /ws) to the
// backend (cmd/server). The browser talks only to this origin, so the page's
// strict CSP and the WebSocket origin check hold while the backend stays a
// headless API. Run it alongside cmd/server — see scripts/run-dev.sh.
//
// -server must point at an http backend (local dev: `cmd/server -insecure`) or a
// real-certificate https backend; a self-signed https backend cannot be proxied
// (the proxy verifies upstream TLS).
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	clientweb "github.com/takezoh/agent-reactor/client/web"
	"github.com/takezoh/agent-reactor/platform/lib/tlsdev"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("addr", ":8080", "listen address")
	backend := flag.String("server", "http://127.0.0.1:8443", "backend base URL to proxy /api and /ws to")
	certFile := flag.String("tls-cert", "", "TLS certificate file (self-signed if empty)")
	keyFile := flag.String("tls-key", "", "TLS key file")
	insecure := flag.Bool("insecure", false, "serve plain HTTP (no TLS) — local dev only")
	flag.Parse()

	handler, err := clientweb.Handler(*backend)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

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
	log.Printf("agent-reactor web client on %s://%s  → backend %s", scheme, *addr, *backend)
	if err := tlsdev.Serve(srv, *insecure, *certFile, *keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
