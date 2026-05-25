package streamjson

import (
	"strings"
	"testing"
)

func TestParse_SystemInit(t *testing.T) {
	line := `{"type":"system","subtype":"init","session_id":"sess-abc123","tools":[]}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(SystemInit)
	if !ok {
		t.Fatalf("got %T, want SystemInit", ev)
	}
	if got.SessionID != "sess-abc123" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-abc123")
	}
}

func TestParse_SystemNonInit(t *testing.T) {
	line := `{"type":"system","subtype":"info","content":"some message"}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ev.(Unknown); !ok {
		t.Fatalf("got %T, want Unknown for non-init system event", ev)
	}
}

func TestParse_AssistantText(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(AssistantMessage)
	if !ok {
		t.Fatalf("got %T, want AssistantMessage", ev)
	}
	if got.Text != "Hello world" {
		t.Errorf("Text = %q, want %q", got.Text, "Hello world")
	}
	if len(got.ToolUses) != 0 {
		t.Errorf("ToolUses = %v, want empty", got.ToolUses)
	}
}

func TestParse_AssistantToolUse(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu1","name":"Bash","input":{"command":"ls"}}]}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(AssistantMessage)
	if !ok {
		t.Fatalf("got %T, want AssistantMessage", ev)
	}
	if len(got.ToolUses) != 1 {
		t.Fatalf("ToolUses len = %d, want 1", len(got.ToolUses))
	}
	if got.ToolUses[0].ID != "tu1" || got.ToolUses[0].Name != "Bash" {
		t.Errorf("ToolUse = %+v, want id=tu1 name=Bash", got.ToolUses[0])
	}
}

func TestParse_ToolResult(t *testing.T) {
	line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","is_error":false,"content":"ok"}]}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(ToolResult)
	if !ok {
		t.Fatalf("got %T, want ToolResult", ev)
	}
	if got.ToolUseID != "tu1" {
		t.Errorf("ToolUseID = %q, want %q", got.ToolUseID, "tu1")
	}
	if got.IsError {
		t.Error("IsError = true, want false")
	}
	if got.Content != "ok" {
		t.Errorf("Content = %q, want %q", got.Content, "ok")
	}
}

func TestParse_ResultSuccess(t *testing.T) {
	line := `{"type":"result","subtype":"success","result":"done","is_error":false,"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(Result)
	if !ok {
		t.Fatalf("got %T, want Result", ev)
	}
	if got.Subtype != "success" {
		t.Errorf("Subtype = %q, want %q", got.Subtype, "success")
	}
	if got.IsError {
		t.Error("IsError = true, want false")
	}
	if got.Usage.InputTokens != 100 || got.Usage.OutputTokens != 50 {
		t.Errorf("Usage = %+v, want input=100 output=50", got.Usage)
	}
	if got.Usage.Total() != 150 {
		t.Errorf("Total() = %d, want 150", got.Usage.Total())
	}
}

func TestParse_ResultSuccessNoTotal(t *testing.T) {
	// total_tokens absent — Total() must compute input+output
	line := `{"type":"result","subtype":"success","result":"done","is_error":false,"usage":{"input_tokens":80,"output_tokens":20}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := ev.(Result)
	if got.Usage.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0 (absent)", got.Usage.TotalTokens)
	}
	if got.Usage.Total() != 100 {
		t.Errorf("Total() = %d, want 100", got.Usage.Total())
	}
}

func TestParse_ResultError(t *testing.T) {
	line := `{"type":"result","subtype":"error_interrupted","result":"","is_error":true,"usage":{"input_tokens":10,"output_tokens":0}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(Result)
	if !ok {
		t.Fatalf("got %T, want Result", ev)
	}
	if !got.IsError {
		t.Error("IsError = false, want true")
	}
	if got.Subtype != "error_interrupted" {
		t.Errorf("Subtype = %q, want %q", got.Subtype, "error_interrupted")
	}
}

func TestParse_Unknown(t *testing.T) {
	line := `{"type":"debug","payload":{"x":1}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(Unknown)
	if !ok {
		t.Fatalf("got %T, want Unknown", ev)
	}
	if got.Type != "debug" {
		t.Errorf("Type = %q, want %q", got.Type, "debug")
	}
}

func TestParse_EmptyLine(t *testing.T) {
	for _, line := range []string{"", "   ", "\t"} {
		ev, err := Parse([]byte(line))
		if err != nil {
			t.Errorf("line %q: unexpected error: %v", line, err)
		}
		if ev != nil {
			t.Errorf("line %q: got %T, want nil", line, ev)
		}
	}
}

func TestParse_MalformedJSON(t *testing.T) {
	line := `{"type":"result", broken`
	ev, err := Parse([]byte(line))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if ev != nil {
		t.Errorf("got event %T on error, want nil", ev)
	}
	if !strings.HasPrefix(err.Error(), "streamjson: ") {
		t.Errorf("error prefix wrong: %v", err)
	}
}

func TestParse_MultipleTextBlocks(t *testing.T) {
	// multiple text blocks are concatenated
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"foo"},{"type":"text","text":"bar"}]}}`
	ev, err := Parse([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := ev.(AssistantMessage)
	if got.Text != "foobar" {
		t.Errorf("Text = %q, want %q", got.Text, "foobar")
	}
}
