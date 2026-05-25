package mcpproxy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestForwardRequests_passthrough(t *testing.T) {
	pol, _ := CompilePolicy([]string{"list_*"}, nil)

	src := strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}` + "\n")
	dst := &bytes.Buffer{}
	out := &syncWriter{w: &bytes.Buffer{}}

	forwardRequests(src, dst, out, pol, "/proj")

	if !strings.Contains(dst.String(), "initialize") {
		t.Errorf("initialize message should be forwarded, got: %q", dst.String())
	}
}

func TestForwardRequests_toolsCall_allowed(t *testing.T) {
	pol, _ := CompilePolicy([]string{"list_logs"}, nil)

	msg := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_logs","arguments":{}},"id":2}` + "\n"
	dst := &bytes.Buffer{}
	out := &syncWriter{w: &bytes.Buffer{}}

	forwardRequests(strings.NewReader(msg), dst, out, pol, "/proj")

	if !strings.Contains(dst.String(), "tools/call") {
		t.Errorf("allowed tools/call should be forwarded, got: %q", dst.String())
	}
	if out.w.(*bytes.Buffer).Len() > 0 {
		t.Errorf("no denial response expected, got: %q", out.w.(*bytes.Buffer).String())
	}
}

func TestForwardRequests_toolsCall_denied(t *testing.T) {
	pol, _ := CompilePolicy([]string{"list_*"}, nil)

	msg := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"delete_bucket","arguments":{}},"id":3}` + "\n"
	dst := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	out := &syncWriter{w: errBuf}

	forwardRequests(strings.NewReader(msg), dst, out, pol, "/proj")

	if dst.Len() > 0 {
		t.Errorf("denied tools/call must not be forwarded to host MCP, got: %q", dst.String())
	}
	var resp rpcMessage
	if err := json.Unmarshal(bytes.TrimRight(errBuf.Bytes(), "\n"), &resp); err != nil {
		t.Fatalf("denial response is not valid JSON: %v, raw: %q", err, errBuf.String())
	}
	if resp.Error == nil {
		t.Error("denial response should contain error field")
	}
	var id float64
	if err := json.Unmarshal(resp.ID, &id); err != nil || id != 3 {
		t.Errorf("denial response ID should be 3, got raw: %s", resp.ID)
	}
}

func TestForwardResponses_toolsList_filtered(t *testing.T) {
	pol, _ := CompilePolicy([]string{"list_logs", "get_metrics"}, nil)

	result := toolsListResult{
		Tools: []toolEntry{
			{Name: "list_logs"},
			{Name: "delete_bucket"},
			{Name: "get_metrics"},
		},
	}
	resultJSON, _ := json.Marshal(result)
	msg := rpcMessage{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: resultJSON}
	line, _ := json.Marshal(msg)

	outBuf := &bytes.Buffer{}
	out := &syncWriter{w: outBuf}
	forwardResponses(strings.NewReader(string(line)+"\n"), out, pol)

	var got rpcMessage
	if err := json.Unmarshal(bytes.TrimRight(outBuf.Bytes(), "\n"), &got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	var gotResult toolsListResult
	if err := json.Unmarshal(got.Result, &gotResult); err != nil {
		t.Fatalf("result unmarshal: %v", err)
	}
	if len(gotResult.Tools) != 2 {
		t.Errorf("expected 2 tools after filter, got %d: %v", len(gotResult.Tools), gotResult.Tools)
	}
	for _, tool := range gotResult.Tools {
		if tool.Name == "delete_bucket" {
			t.Error("delete_bucket should be filtered out")
		}
	}
}

func TestForwardResponses_nonToolsList_passthrough(t *testing.T) {
	pol, _ := CompilePolicy([]string{"list_logs"}, nil)

	result := `{"content":[{"type":"text","text":"hello"}]}`
	msg := rpcMessage{JSONRPC: "2.0", ID: json.RawMessage(`2`), Result: json.RawMessage(result)}
	line, _ := json.Marshal(msg)

	outBuf := &bytes.Buffer{}
	out := &syncWriter{w: outBuf}
	forwardResponses(strings.NewReader(string(line)+"\n"), out, pol)

	if !strings.Contains(outBuf.String(), "hello") {
		t.Errorf("non-tools/list response should pass through unchanged, got: %q", outBuf.String())
	}
}
