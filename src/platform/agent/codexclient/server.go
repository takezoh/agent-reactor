package codexclient

import (
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

// Server wraps a Conn for the server role (e.g. claude-app-server shim).
// It provides convenience emit helpers for common Codex protocol events.
type Server struct {
	conn *Conn
}

// NewServer wraps conn in a Server.
func NewServer(conn *Conn) *Server { return &Server{conn: conn} }

// Conn returns the underlying Conn, e.g. to call Reply/ReplyError directly.
func (s *Server) Conn() *Conn { return s.conn }

// EmitNotification sends an arbitrary server-initiated notification.
func (s *Server) EmitNotification(method string, params any) error {
	return s.conn.Notify(method, params)
}

// EmitThreadStarted emits `thread/started` with the given thread metadata.
func (s *Server) EmitThreadStarted(threadID, cwd string) error {
	return s.conn.Notify(codexschema.MethodThreadStarted, map[string]any{
		"thread": map[string]any{"id": threadID, "cwd": cwd},
	})
}

// EmitTurnStarted emits `turn/started`.
func (s *Server) EmitTurnStarted(threadID, turnID string) error {
	return s.conn.Notify(codexschema.MethodTurnStarted, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
	})
}

// EmitTurnCompleted emits `turn/completed`.
func (s *Server) EmitTurnCompleted(threadID, turnID, text string) error {
	return s.conn.Notify(codexschema.MethodTurnCompleted, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"text":     text,
	})
}

// EmitTurnFailed emits `error` to signal a failed turn.
func (s *Server) EmitTurnFailed(threadID, message string) error {
	return s.conn.Notify(codexschema.MethodError, map[string]any{
		"threadId": threadID,
		"message":  message,
	})
}

// EmitAgentMessageDelta emits `item/agentMessage/delta` for streaming text.
func (s *Server) EmitAgentMessageDelta(threadID, delta string) error {
	return s.conn.Notify(codexschema.MethodItemAgentMessageDelta, map[string]any{
		"threadId": threadID,
		"delta":    delta,
	})
}

// EmitItemStarted emits `item/started` for a tool invocation.
func (s *Server) EmitItemStarted(threadID, turnID string, item map[string]any) error {
	return s.conn.Notify(codexschema.MethodItemStarted, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"item":     item,
	})
}

// EmitItemCompleted emits `item/completed` for a finished tool invocation.
func (s *Server) EmitItemCompleted(threadID, turnID string, item map[string]any) error {
	return s.conn.Notify(codexschema.MethodItemCompleted, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"item":     item,
	})
}

// EmitTokenUsage emits `thread/tokenUsage/updated`.
// last is this turn's breakdown; total is the cumulative thread total.
// Both must be maps with TokenUsageBreakdown fields (inputTokens, outputTokens, etc.).
func (s *Server) EmitTokenUsage(threadID, turnID string, last, total map[string]any) error {
	return s.conn.Notify(codexschema.MethodThreadTokenUsageUpdated, map[string]any{
		"threadId": threadID,
		"turnId":   turnID,
		"tokenUsage": map[string]any{
			"last":               last,
			"total":              total,
			"modelContextWindow": nil,
		},
	})
}
