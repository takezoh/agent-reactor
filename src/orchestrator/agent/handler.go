package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/lineargql"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	schemav2 "github.com/takezoh/agent-roost/platform/agent/codexschema/v2"
	"github.com/takezoh/agent-roost/platform/metrics"
)

type sessionIDs struct {
	threadID string
	turnID   string
}

func (s sessionIDs) sessionID() string { return s.threadID + "-" + s.turnID }

type turnResult struct {
	failed bool
	err    error
}

// toolCallParams is the shape of DynamicToolCallParams from the codex protocol.
type toolCallParams struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	CallID    string          `json:"callId"`
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
}

// toolCallReply is the Symphony-compatible item/tool/call response shape.
// output holds the JSON-encoded tool result as a string so the agent can read it.
type toolCallReply struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

// linearArgs is the §10.5 input shape for the linear_graphql tool.
type linearArgs struct {
	Query     string          `json:"query"`
	Variables json.RawMessage `json:"variables"`
}

// turnHandler dispatches codex protocol notifications to the spawn goroutine.
type turnHandler struct {
	conn          *codexclient.Conn
	linearClient  *lineargql.Client // nil when linear_graphql is not configured
	mu            sync.Mutex
	threadID      string
	turnStartedAt time.Time // protected by mu; set on turn/started, cleared on turn/completed
	sessionReady  chan<- sessionIDs
	turnDone      chan<- turnResult
	issueID       string
	report        func(scheduler.CodexActivity)
}

func (h *turnHandler) OnNotification(method string, params json.RawMessage) {
	now := time.Now()
	act := scheduler.CodexActivity{IssueID: h.issueID, Event: method, Timestamp: now}

	// Session management.
	switch method {
	case codexschema.MethodThreadStarted:
		h.mu.Lock()
		h.threadID = extractThreadID(params)
		h.mu.Unlock()

	case codexschema.MethodTurnStarted:
		turnID := extractString(params, "turnId")
		tid := extractString(params, "threadId")
		h.mu.Lock()
		if tid != "" {
			// With explicit thread/start the thread id arrives here even if no
			// thread/started notification preceded it; keep h.threadID current.
			h.threadID = tid
		}
		threadID := h.threadID
		h.turnStartedAt = now
		h.mu.Unlock()
		select {
		case h.sessionReady <- sessionIDs{threadID: threadID, turnID: turnID}:
		default:
		}

	case codexschema.MethodTurnCompleted:
		h.mu.Lock()
		started := h.turnStartedAt
		h.turnStartedAt = time.Time{}
		h.mu.Unlock()
		if !started.IsZero() {
			d := now.Sub(started)
			act.TurnDuration = &d
		}
		act.TurnCompleted = true
		select {
		case h.turnDone <- turnResult{}:
		default:
		}

	case codexschema.MethodError:
		msg := extractString(params, "message")
		select {
		case h.turnDone <- turnResult{failed: true, err: errors.New(msg)}:
		default:
		}
	}

	// Metrics enrichment.
	switch method {
	case codexschema.MethodItemAgentMessageDelta:
		act.Message = extractString(params, "delta")

	case codexschema.MethodThreadTokenUsageUpdated:
		var n schemav2.ThreadTokenUsageUpdatedNotification
		if err := json.Unmarshal(params, &n); err == nil {
			u := metrics.Usage{
				ThreadID: n.ThreadID,
				Input:    n.TokenUsage.Total.InputTokens,
				Output:   n.TokenUsage.Total.OutputTokens,
				Total:    n.TokenUsage.Total.TotalTokens,
			}
			act.Usage = &u
		}

	case codexschema.MethodAccountRateLimitsUpdated:
		var n schemav2.AccountRateLimitsUpdatedNotification
		if err := json.Unmarshal(params, &n); err == nil {
			rl := mapRateLimit(n.RateLimits)
			act.RateLimit = &rl
		}
	}

	if h.report != nil {
		h.report(act)
	}
}

func mapRateLimit(rl schemav2.AccountRateLimitsUpdatedNotificationRateLimits) metrics.RateLimitSnapshot {
	var snap metrics.RateLimitSnapshot
	if rl.Primary != nil {
		snap.PrimaryUsedPercent = rl.Primary.UsedPercent
		if rl.Primary.ResetsAt != nil {
			snap.PrimaryResetsAt = *rl.Primary.ResetsAt
		}
	}
	if rl.Secondary != nil {
		snap.SecondaryUsedPercent = rl.Secondary.UsedPercent
		if rl.Secondary.ResetsAt != nil {
			snap.SecondaryResetsAt = *rl.Secondary.ResetsAt
		}
	}
	return snap
}

func (h *turnHandler) OnServerRequest(id int64, method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodItemCommandExecutionRequestApproval,
		codexschema.MethodItemFileChangeRequestApproval:
		_ = h.conn.Reply(id, map[string]any{"decision": codexschema.ApprovalAcceptForSession})
	case codexschema.MethodItemToolCall:
		h.handleToolCall(id, params)
	default:
		_ = h.conn.ReplyError(id, "unsupported")
	}
}

func (h *turnHandler) handleToolCall(id int64, params json.RawMessage) {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		_ = h.conn.ReplyError(id, "invalid tool call params")
		return
	}
	if p.Tool != "linear_graphql" || h.linearClient == nil {
		_ = h.conn.ReplyError(id, "unknown tool: "+p.Tool)
		return
	}

	var args linearArgs
	if err := json.Unmarshal(p.Arguments, &args); err != nil {
		_ = h.conn.ReplyError(id, "invalid linear_graphql arguments")
		return
	}

	result, err := h.linearClient.Execute(context.Background(), args.Query, args.Variables)
	if err != nil {
		_ = h.conn.ReplyError(id, "linear_graphql internal error")
		return
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		_ = h.conn.ReplyError(id, "linear_graphql result encode error")
		return
	}
	_ = h.conn.Reply(id, toolCallReply{Success: result.Success, Output: string(resultJSON)})
}

func extractThreadID(params json.RawMessage) string {
	var p struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(params, &p)
	return p.Thread.ID
}

func extractString(params json.RawMessage, key string) string {
	var p map[string]json.RawMessage
	if err := json.Unmarshal(params, &p); err != nil {
		return ""
	}
	var s string
	_ = json.Unmarshal(p[key], &s)
	return s
}
