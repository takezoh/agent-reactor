package stream

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/takezoh/agent-roost/state"
)

func (b *Backend) handleNotification(msg rpcMessage) {
	switch msg.Method {
	case "thread/started":
		b.handleThreadStarted(msg.Params)
	case "turn/started":
		b.emitToThread(extractThreadID(msg.Params), state.SubsystemTurnStarted, func(p *state.SubsystemPayload) {
			p.TurnID = extractTurnID(msg.Params)
		})
	case "turn/completed":
		b.handleTurnCompleted(msg.Params)
	case "turn/plan/updated":
		b.emitToThread(extractThreadID(msg.Params), state.SubsystemPlanUpdated, func(p *state.SubsystemPayload) {
			p.Plan = &state.SubsystemPlan{Summary: summarizePlan(msg.Params)}
		})
	case "turn/diff/updated":
		b.emitToThread(extractThreadID(msg.Params), state.SubsystemDiffUpdated, func(p *state.SubsystemPayload) {
			p.Diff = &state.SubsystemDiff{Summary: summarizeDiff(msg.Params), Paths: diffPaths(msg.Params)}
		})
	case "item/started":
		b.emitItemLifecycle("item/started", msg.Params)
	case "item/completed":
		b.emitItemLifecycle("item/completed", msg.Params)
	case "thread/status/changed":
		b.handleThreadStatusChanged(msg.Params)
	case "item/agentMessage/delta":
		b.handleAgentMessageDelta(msg.Params)
	case "error":
		slog.Error("stream backend: app-server error", "subsystem", b.subsystemID, "params", string(msg.Params))
	case "warning", "guardianWarning", "deprecationNotice":
		slog.Warn("stream backend: app-server notice", "method", msg.Method, "subsystem", b.subsystemID, "params", string(msg.Params))
	}
}

func (b *Backend) handleRequest(msg rpcMessage) {
	switch msg.Method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		threadID := extractThreadID(msg.Params)
		frameID := b.frameForThread(threadID)
		if frameID == "" {
			return
		}
		approval := approvalFromParams(msg.Method, msg.Params, b.autoApprove)
		b.emit(frameID, state.SubsystemApprovalRequested, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
		result := "accept"
		if b.autoApprove {
			result = "acceptForSession"
		}
		_ = b.reply(*msg.ID, result)
		approval.Resolved = true
		b.emit(frameID, state.SubsystemApprovalResolved, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
	default:
		slog.Warn("stream backend: rejecting unhandled server request",
			"method", msg.Method, "subsystem", b.subsystemID)
		if msg.ID != nil {
			_ = b.replyError(*msg.ID, "method not supported by roost")
		}
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
		binding.resumePhase = "attached"
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
	if binding != nil {
		binding.activeTurnID = ""
		if last != "" {
			binding.lastAssistant = last
			appendHistory(&binding.history, "assistant", last)
		}
	}
	history := append([]state.SubsystemTurn(nil), binding.history...)
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemTurnCompleted, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.LastAssistantMessage = last
		p.Message = &state.SubsystemMessage{RecentTurns: history}
	}))
}

func (b *Backend) handleAgentMessageDelta(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	text := extractText(raw)
	if text == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding != nil {
		binding.lastAssistant += text
	}
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
	prevStatus := binding.threadStatus
	prevWaiting := binding.waitApproval
	b.mu.Unlock()
	events, nextStatus, nextWaiting := threadStatusEvents(raw, threadID, prevStatus, prevWaiting)
	b.mu.Lock()
	binding = b.frames[frameID]
	if binding != nil {
		binding.threadStatus = nextStatus
		binding.waitApproval = nextWaiting
	}
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
