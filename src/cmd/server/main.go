//go:build legacy_session

// Command server is the agent-reactor backend: a headless API that manages
// agent sessions over pty (tmux-free) and exposes them to any client over a
// REST + WebSocket interface. Sessions are host-owned — they keep running when
// a client disconnects, and several clients can attach to and share one session
// (e.g. one operator's claude-code driven by another). It serves no HTML; the
// web UI is a separate process (cmd/web) that connects here, as will future
// native clients. Auth is a bearer token; transport is TLS (self-signed by
// default, or -tls-cert/-tls-key, or -insecure for local dev).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/lib/tlsdev"
	"github.com/takezoh/agent-reactor/server/session"
	serverweb "github.com/takezoh/agent-reactor/server/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run wires and serves the backend. It returns an error rather than calling
// log.Fatal so its deferred cleanup (signal-context stop) always runs; main
// turns a non-nil error into the fatal exit.
func run() error {
	addr := flag.String("addr", ":8443", "listen address")
	tokenFlag := flag.String("token", "", "bearer token (generated and printed if empty)")
	certFile := flag.String("tls-cert", "", "TLS certificate file (self-signed if empty)")
	keyFile := flag.String("tls-key", "", "TLS key file")
	insecure := flag.Bool("insecure", false, "serve plain HTTP (no TLS) — local dev only")
	flag.Parse()

	token := *tokenFlag
	if token == "" {
		token = randToken()
	}

	svc := session.NewService(agentlaunch.DirectDispatcher{})
	srv := &http.Server{Addr: *addr, Handler: serverweb.NewMux(svc, token), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		svc.CloseAll(context.Background())
	}()

	scheme := "https"
	if *insecure {
		scheme = "http"
	}
	log.Printf("agent-reactor backend on %s://%s  token=%s", scheme, *addr, token)
	if err := tlsdev.Serve(srv, *insecure, *certFile, *keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func randToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
