package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/lib/claude/streamjson"
)

// turnReq carries a decoded turn/start notification payload.
type turnReq struct {
	threadID       string // empty on first turn; shim generates one
	cwd            string
	prompt         string
	approvalPolicy string // logged but not enforced; container is the boundary
	sandboxPolicy  string // logged but not enforced
}

// turnRunner processes turns sequentially. It holds the mutable shim state.
type turnRunner struct {
	ctx      context.Context
	srv      *codexclient.Server
	writeMu  *sync.Mutex
	threads  map[string]string           // threadID → claude session_id
	cumUsage map[string]streamjson.Usage // threadID → cumulative token usage
	mu       sync.Mutex
	launch   claudeLauncher
	newID    func() string
}

func (r *turnRunner) run(turns <-chan turnReq, stopCh <-chan struct{}) {
	for {
		select {
		case req, ok := <-turns:
			if !ok {
				return
			}
			r.runTurn(req)
		case <-stopCh:
			return
		}
	}
}

func (r *turnRunner) runTurn(req turnReq) {
	threadID := req.threadID
	isNewThread := threadID == ""
	if isNewThread {
		threadID = r.newID()
	}
	turnID := r.newID()
	sessionID := threadID + "-" + turnID

	if req.approvalPolicy != "" || req.sandboxPolicy != "" {
		slog.Warn("approval/sandbox policy received but not enforced by shim; container is the boundary",
			"approvalPolicy", req.approvalPolicy,
			"sandboxPolicy", req.sandboxPolicy,
		)
	}

	if isNewThread {
		if err := r.emit(func() error { return r.srv.EmitThreadStarted(threadID, req.cwd) }); err != nil {
			slog.Error("emit thread/started", "err", err)
			return
		}
	}
	if err := r.emit(func() error {
		return r.srv.Conn().Notify(codexschema.MethodTurnStarted, map[string]any{
			"threadId": threadID, "turnId": turnID, "sessionId": sessionID,
		})
	}); err != nil {
		slog.Error("emit turn/started", "err", err)
		return
	}

	r.mu.Lock()
	resumeID := r.threads[threadID]
	r.mu.Unlock()

	stdout, wait, err := r.launch(r.ctx, req.cwd, resumeID, req.prompt)
	if err != nil {
		slog.Error("launch claude", "err", err)
		_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, err.Error()) })
		return
	}

	resultReceived := r.scanStream(threadID, turnID, sessionID, streamjson.NewScanner(stdout))
	if err := wait(); err != nil && !resultReceived {
		msg := fmt.Sprintf("claude exited: %v", err)
		_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, msg) })
	}
}

// scanStream processes the claude stream-json events for one turn and emits
// the corresponding Codex protocol notifications. Returns true when a result
// event was received (success or error).
func (r *turnRunner) scanStream(threadID, turnID, sessionID string, sc *streamjson.Scanner) bool { //nolint:cyclop
	toolNames := map[string]string{} // toolUseID → name for item/completed correlation
	resultReceived := false
	for sc.Scan() {
		switch ev := sc.Event().(type) {
		case streamjson.SystemInit:
			r.mu.Lock()
			r.threads[threadID] = ev.SessionID
			r.mu.Unlock()

		case streamjson.AssistantMessage:
			if ev.Text != "" {
				_ = r.emit(func() error { return r.srv.EmitAgentMessageDelta(threadID, ev.Text) })
			}
			for _, tu := range ev.ToolUses {
				toolNames[tu.ID] = tu.Name
				id, name, input := tu.ID, tu.Name, tu.Input
				_ = r.emit(func() error {
					return r.srv.EmitItemStarted(threadID, turnID, map[string]any{
						"id": id, "type": "dynamicToolCall", "tool": name, "arguments": input,
					})
				})
			}

		case streamjson.ToolResult:
			status := "completed"
			if ev.IsError {
				status = "failed"
			}
			id, tool, content := ev.ToolUseID, toolNames[ev.ToolUseID], ev.Content
			_ = r.emit(func() error {
				return r.srv.EmitItemCompleted(threadID, turnID, map[string]any{
					"id": id, "type": "dynamicToolCall", "tool": tool, "status": status, "output": content,
				})
			})

		case streamjson.Result:
			resultReceived = true
			if ev.IsError {
				_ = r.emit(func() error { return r.srv.EmitTurnFailed(threadID, ev.ResultText) })
			} else {
				r.emitUsageAndComplete(threadID, turnID, sessionID, ev.Usage, ev.ResultText)
			}
		}
	}
	if err := sc.Err(); err != nil {
		slog.Error("stream scan", "err", err)
	}
	return resultReceived
}

func (r *turnRunner) emitUsageAndComplete(threadID, turnID, sessionID string, u streamjson.Usage, text string) {
	r.mu.Lock()
	cum := r.cumUsage[threadID]
	cum.InputTokens += u.InputTokens
	cum.OutputTokens += u.OutputTokens
	cum.TotalTokens = cum.InputTokens + cum.OutputTokens
	r.cumUsage[threadID] = cum
	r.mu.Unlock()

	last, total := usageBreakdown(u), usageBreakdown(cum)
	_ = r.emit(func() error { return r.srv.EmitTokenUsage(threadID, turnID, last, total) })
	_ = r.emit(func() error {
		return r.srv.Conn().Notify(codexschema.MethodTurnCompleted, map[string]any{
			"threadId": threadID, "turnId": turnID, "sessionId": sessionID, "text": text,
		})
	})
}

// emit serializes all conn writes through writeMu.
func (r *turnRunner) emit(fn func() error) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return fn()
}

// usageBreakdown converts a streamjson.Usage to the codex TokenUsageBreakdown map shape.
func usageBreakdown(u streamjson.Usage) map[string]any {
	return map[string]any{
		"inputTokens":           u.InputTokens,
		"outputTokens":          u.OutputTokens,
		"totalTokens":           u.Total(),
		"cachedInputTokens":     0,
		"reasoningOutputTokens": 0,
	}
}

// parseTurnStart decodes the turn/start notification params.
func parseTurnStart(params json.RawMessage) turnReq {
	var p struct {
		ThreadID       string          `json:"threadId"`
		CWD            string          `json:"cwd"`
		Message        string          `json:"message"`
		ApprovalPolicy json.RawMessage `json:"approvalPolicy"`
		SandboxPolicy  json.RawMessage `json:"sandboxPolicy"`
	}
	_ = json.Unmarshal(params, &p)
	return turnReq{
		threadID:       p.ThreadID,
		cwd:            p.CWD,
		prompt:         p.Message,
		approvalPolicy: string(p.ApprovalPolicy),
		sandboxPolicy:  string(p.SandboxPolicy),
	}
}
