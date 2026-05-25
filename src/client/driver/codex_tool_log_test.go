package driver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/client/state"
)

func toolStartedEv(id, name, cmd string, ts time.Time) state.DEvSubsystem {
	return state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemToolStarted,
		Timestamp: ts,
		Payload: state.SubsystemPayload{
			Tool: &state.SubsystemTool{ID: id, Name: name, Command: cmd},
		},
	}
}

func toolCompletedEv(id, name, cmd, errStr string, ts time.Time) state.DEvSubsystem {
	return state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemToolCompleted,
		Timestamp: ts,
		Payload: state.SubsystemPayload{
			Tool: &state.SubsystemTool{ID: id, Name: name, Command: cmd, Error: errStr},
		},
	}
}

func TestCodexSubsystemToolCompletedEmitsToolLog(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	cs, _ = d.handleSubsystem(cs, ctx, toolStartedEv("tool-1", "Bash", "echo hi", now))

	next, effs := d.handleSubsystem(cs, ctx, toolCompletedEv("tool-1", "Bash", "echo hi", "", now.Add(2*time.Second)))

	if next.CurrentTool != "" {
		t.Fatalf("CurrentTool = %q, want empty", next.CurrentTool)
	}

	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}
	if appendEff.Namespace != CodexDriverName {
		t.Fatalf("Namespace = %q, want %q", appendEff.Namespace, CodexDriverName)
	}

	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	if entry.Kind != "auto" {
		t.Fatalf("Kind = %q, want auto", entry.Kind)
	}
	if entry.ToolName != "Bash" {
		t.Fatalf("ToolName = %q, want Bash", entry.ToolName)
	}
	if entry.DurationMs != 2000 {
		t.Fatalf("DurationMs = %d, want 2000", entry.DurationMs)
	}
}

func TestCodexSubsystemToolCompletedWithoutStartIsOrphan(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	_, effs := d.handleSubsystem(cs, ctx, toolCompletedEv("missing-id", "Read", "", "", now))

	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}

	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	if entry.Kind != "orphan" {
		t.Fatalf("Kind = %q, want orphan", entry.Kind)
	}
}

func TestCodexSubsystemToolCompletedSummarisesToolInput(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ctx := state.FrameContext{IsRoot: true}

	longCmd := strings.Repeat("x", 240)
	_, effs := d.handleSubsystem(cs, ctx, toolCompletedEv("tool-2", "Bash", longCmd, "", now))

	appendEff, ok := findCodexEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}

	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	cmd, _ := entry.ToolInput["command"].(string)
	if len([]rune(cmd)) > 201 {
		t.Fatalf("command too long: %d runes", len([]rune(cmd)))
	}
	if !strings.HasSuffix(cmd, "…") {
		t.Fatalf("command = %q, want truncated suffix", cmd)
	}
}
