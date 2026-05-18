package stream

import (
	"encoding/json"
	"strings"

	"github.com/takezoh/agent-roost/state"
)

type subsystemEmission struct {
	kind    state.SubsystemEventKind
	payload state.SubsystemPayload
}

func extractThreadID(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["threadId"].(string); s != "" {
		return s
	}
	if thread, ok := data["thread"].(map[string]any); ok {
		if s, _ := thread["id"].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractTurnID(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["turnId"].(string); s != "" {
		return s
	}
	if turn, ok := data["turn"].(map[string]any); ok {
		if s, _ := turn["id"].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractThreadCWD(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["cwd"].(string); s != "" {
		return s
	}
	if thread, ok := data["thread"].(map[string]any); ok {
		if s, _ := thread["cwd"].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractText(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	for _, key := range []string{"text", "delta"} {
		if s, _ := data[key].(string); s != "" {
			return s
		}
	}
	if item, ok := data["item"].(map[string]any); ok {
		for _, key := range []string{"text", "content"} {
			if s, _ := item[key].(string); s != "" {
				return s
			}
		}
	}
	return ""
}

func nestedString(raw json.RawMessage, key string) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data[key].(string); s != "" {
		return s
	}
	if item, ok := data["item"].(map[string]any); ok {
		if s, _ := item[key].(string); s != "" {
			return s
		}
	}
	return ""
}

func extractThreadStatus(raw json.RawMessage) (string, bool, string) {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return "", false, ""
	}
	threadID, _ := data["threadId"].(string)
	status, _ := data["status"].(map[string]any)
	if thread, ok := data["thread"].(map[string]any); ok {
		if threadID == "" {
			threadID, _ = thread["id"].(string)
		}
		if status == nil {
			status, _ = thread["status"].(map[string]any)
		}
	}
	if status == nil {
		return "", false, threadID
	}
	statusType, _ := status["type"].(string)
	flags, _ := status["activeFlags"].([]any)
	waitingApproval := false
	for _, flag := range flags {
		s, _ := flag.(string)
		if s == "waitingOnApproval" {
			waitingApproval = true
			break
		}
	}
	return statusType, waitingApproval, threadID
}

func threadStatusEvents(raw json.RawMessage, currentThreadID, prevStatus string, prevWaitingApproval bool) ([]subsystemEmission, string, bool) {
	statusType, waitingApproval, threadID := extractThreadStatus(raw)
	if threadID == "" {
		threadID = currentThreadID
	}
	if currentThreadID != "" && threadID != "" && threadID != currentThreadID {
		return nil, prevStatus, prevWaitingApproval
	}
	if statusType == "" {
		return nil, prevStatus, prevWaitingApproval
	}
	var out []subsystemEmission
	switch statusType {
	case "active":
		if prevStatus != "active" {
			out = append(out, subsystemEmission{kind: state.SubsystemTurnStarted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID}})
		}
		if waitingApproval && !prevWaitingApproval {
			out = append(out, subsystemEmission{kind: state.SubsystemApprovalRequested, payload: state.SubsystemPayload{
				SessionID: threadID,
				TargetID:  threadID,
				Approval:  &state.SubsystemApproval{Kind: "command"},
			}})
		}
		if !waitingApproval && prevWaitingApproval {
			out = append(out, subsystemEmission{kind: state.SubsystemApprovalResolved, payload: state.SubsystemPayload{
				SessionID: threadID,
				TargetID:  threadID,
				Approval:  &state.SubsystemApproval{Kind: "command", Resolved: true},
			}})
		}
	case "idle":
		if prevWaitingApproval {
			out = append(out, subsystemEmission{kind: state.SubsystemApprovalResolved, payload: state.SubsystemPayload{
				SessionID: threadID,
				TargetID:  threadID,
				Approval:  &state.SubsystemApproval{Kind: "command", Resolved: true},
			}})
		}
		if prevStatus != "idle" {
			out = append(out, subsystemEmission{kind: state.SubsystemTurnCompleted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID}})
		}
	}
	return out, statusType, waitingApproval
}

func itemType(raw json.RawMessage) string { return nestedString(raw, "type") }

func itemLifecycleEvents(method string, raw json.RawMessage, currentThreadID string) []subsystemEmission {
	threadID := firstNonEmpty(extractThreadID(raw), currentThreadID)
	switch method {
	case "item/started":
		switch itemType(raw) {
		case "commandExecution":
			tool := commandTool(raw)
			return []subsystemEmission{{kind: state.SubsystemToolStarted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		case "fileChange":
			tool := fileChangeTool(raw)
			return []subsystemEmission{{kind: state.SubsystemToolStarted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		}
	case "item/completed":
		switch itemType(raw) {
		case "commandExecution":
			tool := commandTool(raw)
			tool.Error = nestedString(raw, "error")
			return []subsystemEmission{{kind: state.SubsystemToolCompleted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		case "fileChange":
			tool := fileChangeTool(raw)
			return []subsystemEmission{{kind: state.SubsystemToolCompleted, payload: state.SubsystemPayload{SessionID: threadID, TargetID: threadID, Tool: &tool}}}
		}
	}
	return nil
}

func commandTool(raw json.RawMessage) state.SubsystemTool {
	return state.SubsystemTool{
		ID:      nestedString(raw, "itemId"),
		Name:    "command",
		Command: nestedString(raw, "command"),
		Path:    nestedString(raw, "cwd"),
	}
}

func fileChangeTool(raw json.RawMessage) state.SubsystemTool {
	return state.SubsystemTool{
		ID:   nestedString(raw, "itemId"),
		Name: "file_change",
		Path: nestedString(raw, "path"),
	}
}

func summarizePlan(raw json.RawMessage) string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	if s, _ := data["summary"].(string); s != "" {
		return s
	}
	if items, ok := data["items"].([]any); ok {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			m, _ := item.(map[string]any)
			step, _ := m["step"].(string)
			status, _ := m["status"].(string)
			if step != "" {
				parts = append(parts, strings.TrimSpace(step+" "+status))
			}
		}
		return strings.Join(parts, " | ")
	}
	return ""
}

func summarizeDiff(raw json.RawMessage) string {
	paths := diffPaths(raw)
	if len(paths) == 0 {
		return ""
	}
	return strings.Join(paths, ", ")
}

func diffPaths(raw json.RawMessage) []string {
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return nil
	}
	list, ok := data["paths"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, _ := item.(string); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func approvalFromParams(method string, raw json.RawMessage, auto bool) state.SubsystemApproval {
	kind := "command"
	if strings.Contains(method, "fileChange") {
		kind = "file_change"
	}
	return state.SubsystemApproval{
		ID:          nestedString(raw, "itemId"),
		Kind:        kind,
		Command:     nestedString(raw, "command"),
		Path:        nestedString(raw, "path"),
		Reason:      nestedString(raw, "reason"),
		AutoApprove: auto,
	}
}

func appendHistory(history *[]state.SubsystemTurn, role, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	*history = append(*history, state.SubsystemTurn{Role: role, Text: text})
	if len(*history) > 6 {
		*history = (*history)[len(*history)-6:]
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
