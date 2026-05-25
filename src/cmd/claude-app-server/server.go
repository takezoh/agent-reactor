package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/lib/claude/streamjson"
	"github.com/takezoh/agent-roost/platform/logger"
)

// appHandler implements codexclient.Handler for the shim server role.
type appHandler struct {
	conn    *codexclient.Conn
	writeMu sync.Mutex
	runner  *turnRunner
	turns   chan turnReq
}

func newAppHandler(conn *codexclient.Conn, appCtx context.Context, launch claudeLauncher, newID func() string) *appHandler {
	turns := make(chan turnReq, 8)
	srv := codexclient.NewServer(conn)
	h := &appHandler{
		conn:  conn,
		turns: turns,
		runner: &turnRunner{
			ctx:      appCtx,
			srv:      srv,
			writeMu:  nil, // set below after h is built
			threads:  make(map[string]string),
			cumUsage: make(map[string]streamjson.Usage),
			dynTools: make(map[string][]dynToolSpec),
			launch:   launch,
			newID:    newID,
		},
	}
	h.runner.writeMu = &h.writeMu
	return h
}

func (h *appHandler) OnServerRequest(id int64, method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodInitialize:
		h.writeMu.Lock()
		_ = h.conn.Reply(id, initializeResponse())
		h.writeMu.Unlock()
	case codexschema.MethodThreadStart:
		var p struct {
			CWD          string        `json:"cwd"`
			DynamicTools []dynToolSpec `json:"dynamicTools"`
		}
		_ = json.Unmarshal(params, &p)
		threadID := h.runner.startThread(p.DynamicTools)
		h.writeMu.Lock()
		_ = h.runner.srv.EmitThreadStarted(threadID, p.CWD)
		_ = h.conn.Reply(id, map[string]any{"thread": map[string]any{"id": threadID}})
		h.writeMu.Unlock()
	case codexschema.MethodThreadResume:
		var p struct {
			ThreadID string `json:"threadId"`
			CWD      string `json:"cwd"`
		}
		_ = json.Unmarshal(params, &p)
		h.runner.mu.Lock()
		claudeSessionID := h.runner.threads[p.ThreadID]
		h.runner.mu.Unlock()
		h.writeMu.Lock()
		_ = h.conn.Reply(id, map[string]any{
			"threadId":        p.ThreadID,
			"cwd":             p.CWD,
			"claudeSessionId": claudeSessionID,
		})
		h.writeMu.Unlock()
	default:
		h.writeMu.Lock()
		_ = h.conn.ReplyError(id, fmt.Sprintf("method %q not implemented", method))
		h.writeMu.Unlock()
	}
}

func (h *appHandler) OnNotification(method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodTurnStart:
		req := parseTurnStart(params)
		select {
		case h.turns <- req:
		default:
			slog.Warn("turn queue full, dropping turn/start")
		}
	default:
		slog.Debug("notification received", "method", method)
	}
}

func run(ctx context.Context, t codexclient.Transport) int {
	return runWith(ctx, t, realLaunch, func() string { return uuid.Must(uuid.NewV7()).String() })
}

func runWith(ctx context.Context, t codexclient.Transport, launch claudeLauncher, newID func() string) int {
	if err := logger.Init("info"); err != nil {
		fmt.Fprintf(os.Stderr, "claude-app-server: logger init: %v\n", err)
		return 1
	}
	defer logger.Close()

	appCtx, appCancel := context.WithCancel(ctx)
	defer appCancel()

	conn := codexclient.NewConn(t, 0)
	h := newAppHandler(conn, appCtx, launch, newID)

	stopTurns := make(chan struct{})
	turnDone := make(chan struct{})
	go func() {
		defer close(turnDone)
		h.runner.run(h.turns, stopTurns)
	}()

	runDone := make(chan error, 1)
	go func() { runDone <- conn.Run(ctx, h) }()

	select {
	case <-ctx.Done():
		slog.Info("claude-app-server stopping")
	case <-runDone:
		slog.Info("claude-app-server stopped")
	}

	appCancel()
	close(stopTurns)
	<-turnDone
	return 0
}
