package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
)

// Config holds the bind address and port for the HTTP server.
type Config struct {
	Bind string // default 127.0.0.1
	Port int    // positive = listen on that port; 0 = ephemeral
}

// Server is the observability HTTP server (§13.7).
type Server struct {
	srv *http.Server
	ln  net.Listener
}

// New creates a Server that is ready to listen. Call Serve to start accepting.
func New(cfg Config, handler http.Handler) (*Server, error) {
	bind := cfg.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	addr := net.JoinHostPort(bind, strconv.Itoa(cfg.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("httpserver listen %s: %w", addr, err)
	}
	slog.Info("httpserver listening", "addr", ln.Addr().String())
	return &Server{
		srv: &http.Server{Handler: handler},
		ln:  ln,
	}, nil
}

// Addr returns the actual listen address (useful when port was 0).
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Serve starts accepting connections and blocks until the context is cancelled.
// After the context is cancelled a graceful shutdown is attempted.
func (s *Server) Serve(ctx context.Context) {
	go func() {
		<-ctx.Done()
		_ = s.srv.Shutdown(context.Background())
	}()
	if err := s.srv.Serve(s.ln); err != nil && err != http.ErrServerClosed {
		slog.Error("httpserver error", "err", err)
	}
}
