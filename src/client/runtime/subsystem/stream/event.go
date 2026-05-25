package stream

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

func (b *Backend) handleNotification(method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodThreadStarted:
		b.handleThreadStarted(params)
	case codexschema.MethodTurnStarted:
		b.emitToThread(extractThreadID(params), state.SubsystemTurnStarted, func(p *state.SubsystemPayload) {
			p.TurnID = extractTurnID(params)
		})
	case codexschema.MethodTurnCompleted:
		b.handleTurnCompleted(params)
	case codexschema.MethodTurnPlanUpdated:
		b.emitToThread(extractThreadID(params), state.SubsystemPlanUpdated, func(p *state.SubsystemPayload) {
			p.Plan = &state.SubsystemPlan{Summary: summarizePlan(params)}
		})
	case codexschema.MethodTurnDiffUpdated:
		b.emitToThread(extractThreadID(params), state.SubsystemDiffUpdated, func(p *state.SubsystemPayload) {
			p.Diff = &state.SubsystemDiff{Summary: summarizeDiff(params), Paths: diffPaths(params)}
		})
	case codexschema.MethodItemStarted:
		b.emitItemLifecycle(codexschema.MethodItemStarted, params)
	case codexschema.MethodItemCompleted:
		b.emitItemLifecycle(codexschema.MethodItemCompleted, params)
	case codexschema.MethodThreadStatusChanged:
		b.handleThreadStatusChanged(params)
	case codexschema.MethodItemAgentMessageDelta:
		b.handleAgentMessageDelta(params)
	case codexschema.MethodError:
		slog.Error("stream backend: app-server error", "subsystem", b.subsystemID, "params", string(params))
	case codexschema.MethodWarning, codexschema.MethodGuardianWarning, codexschema.MethodDeprecationNotice:
		slog.Warn("stream backend: app-server notice", "method", method, "subsystem", b.subsystemID, "params", string(params))
	}
}

func (b *Backend) handleRequest(id int64, method string, params json.RawMessage) {
	switch method {
	case codexschema.MethodItemCommandExecutionRequestApproval, codexschema.MethodItemFileChangeRequestApproval:
		threadID := extractThreadID(params)
		frameID := b.frameForThread(threadID)
		if frameID == "" {
			return
		}
		approval := approvalFromParams(method, params, b.autoApprove)
		b.emit(frameID, state.SubsystemApprovalRequested, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
		result := codexschema.ApprovalAccept
		if b.autoApprove {
			result = codexschema.ApprovalAcceptForSession
		}
		_ = b.conn.Reply(id, result)
		approval.Resolved = true
		b.emit(frameID, state.SubsystemApprovalResolved, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
	default:
		slog.Warn("stream backend: rejecting unhandled server request",
			"method", method, "subsystem", b.subsystemID)
		_ = b.conn.ReplyError(id, "method not supported by roost")
	}
}

func (b *Backend) handleThreadStarted(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.resolveFrameForStartedThread(threadID, extractThreadCWD(raw))
	if frameID == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding != nil {
		binding.threadID = threadID
		binding.requestedID = threadID
		binding.observedID = threadID
		binding.resumePhase = resumePhaseAttached
		b.threads[threadID] = frameID
	}
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemSessionReady, b.payload(frameID))
}

func (b *Backend) handleTurnCompleted(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	last := strings.TrimSpace(extractText(raw))
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil {
		b.mu.Unlock()
		return
	}
	binding.activeTurnID = ""
	if last != "" {
		binding.lastAssistant = last
		appendHistory(&binding.history, "assistant", last)
	}
	history := append([]state.SubsystemTurn(nil), binding.history...)
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemTurnCompleted, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.LastAssistantMessage = last
		p.Message = &state.SubsystemMessage{RecentTurns: history}
	}))
}

func (b *Backend) handleAgentMessageDelta(raw json.RawMessage) {
	// Hot path: single unmarshal instead of two separate extract calls.
	var params struct {
		ThreadID string `json:"threadId"`
		Delta    string `json:"delta"`
		Text     string `json:"text"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return
	}
	text := params.Delta
	if text == "" {
		text = params.Text
	}
	if text == "" {
		return
	}
	frameID := b.frameForThread(params.ThreadID)
	if frameID == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil {
		b.mu.Unlock()
		return
	}
	binding.lastAssistant += text
	last := binding.lastAssistant
	history := append([]state.SubsystemTurn(nil), binding.history...)
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemMessageUpdated, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.LastAssistantMessage = last
		p.Message = &state.SubsystemMessage{RecentTurns: history}
	}))
}

func (b *Backend) resolveFrameForStartedThread(threadID, cwd string) state.FrameID {
	if threadID == "" {
		return ""
	}
	if frameID := b.frameForThread(threadID); frameID != "" {
		return frameID
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var candidates []state.FrameID
	for frameID, binding := range b.frames {
		if binding.threadID != "" {
			continue
		}
		if binding.startDir == cwd {
			candidates = append(candidates, frameID)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	active := b.activeLookup()
	if active != "" {
		if _, ok := b.frames[active]; ok {
			return active
		}
	}
	return ""
}

func (b *Backend) handleThreadStatusChanged(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil {
		b.mu.Unlock()
		return
	}
	prevStatus, prevWaiting := binding.threadStatus, binding.waitApproval
	events, nextStatus, nextWaiting := threadStatusEvents(raw, threadID, prevStatus, prevWaiting)
	binding.threadStatus = nextStatus
	binding.waitApproval = nextWaiting
	b.mu.Unlock()
	for _, ev := range events {
		ev.payload = b.withTracking(frameID, ev.payload)
		b.emit(frameID, ev.kind, ev.payload)
	}
}

func (b *Backend) emitItemLifecycle(method string, raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	for _, ev := range itemLifecycleEvents(method, raw, threadID) {
		ev.payload = b.withTracking(frameID, ev.payload)
		b.emit(frameID, ev.kind, ev.payload)
	}
}

func (b *Backend) emitToThread(threadID string, kind state.SubsystemEventKind, mutate func(*state.SubsystemPayload)) {
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	b.emit(frameID, kind, b.payloadWith(frameID, mutate))
}

func (b *Backend) payload(frameID state.FrameID) state.SubsystemPayload {
	return b.payloadWith(frameID, nil)
}

func (b *Backend) payloadWith(frameID state.FrameID, mutate func(*state.SubsystemPayload)) state.SubsystemPayload {
	b.mu.Lock()
	binding := b.frames[frameID]
	payload := state.SubsystemPayload{}
	if binding != nil {
		payload = state.SubsystemPayload{
			SessionID:         binding.threadID,
			TargetID:          binding.threadID,
			RequestedTargetID: binding.requestedID,
			ObservedTargetID:  binding.observedID,
			ResumePhase:       binding.resumePhase,
		}
	}
	b.mu.Unlock()
	if mutate != nil {
		mutate(&payload)
	}
	return payload
}

func (b *Backend) withTracking(frameID state.FrameID, payload state.SubsystemPayload) state.SubsystemPayload {
	base := b.payload(frameID)
	if payload.SessionID == "" {
		payload.SessionID = base.SessionID
	}
	if payload.TargetID == "" {
		payload.TargetID = base.TargetID
	}
	payload.RequestedTargetID = base.RequestedTargetID
	payload.ObservedTargetID = base.ObservedTargetID
	payload.ResumePhase = base.ResumePhase
	return payload
}

func (b *Backend) failFrame(frameID state.FrameID, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil || binding.failureReported {
		b.mu.Unlock()
		return
	}
	binding.failureReported = true
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemFailed, state.SubsystemPayload{Error: msg})
}
