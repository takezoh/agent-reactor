package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, codexclient.DefaultStdioTransport())
	stop()
	os.Exit(code)
}

type appHandler struct {
	conn *codexclient.Conn
}

func (h *appHandler) OnServerRequest(id int64, method string, _ json.RawMessage) {
	switch method {
	case codexschema.MethodInitialize:
		_ = h.conn.Reply(id, map[string]any{
			"serverInfo":   map[string]any{"name": "claude-app-server", "version": "0"},
			"capabilities": map[string]any{"experimentalApi": true},
		})
	default:
		_ = h.conn.ReplyError(id, fmt.Sprintf("method %q not implemented", method))
	}
}

func (h *appHandler) OnNotification(method string, _ json.RawMessage) {
	slog.Debug("notification received", "method", method)
}

func run(ctx context.Context, t codexclient.Transport) int {
	if err := logger.Init("info"); err != nil {
		fmt.Fprintf(os.Stderr, "claude-app-server: logger init: %v\n", err)
		return 1
	}
	defer logger.Close()

	conn := codexclient.NewConn(t, 0)
	h := &appHandler{conn: conn}

	done := make(chan error, 1)
	go func() { done <- conn.Run(ctx, h) }()

	select {
	case <-ctx.Done():
		slog.Info("claude-app-server stopping")
	case <-done:
		slog.Info("claude-app-server stopped")
	}
	return 0
}
