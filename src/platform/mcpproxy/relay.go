package mcpproxy

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
)

// rpcMessage is a minimal JSON-RPC 2.0 envelope.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolsCallParams struct {
	Name string `json:"name"`
}

type toolsListResult struct {
	Tools []toolEntry `json:"tools"`
}

type toolEntry struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// forwardRequests reads newline-delimited JSON-RPC lines from src, applies
// tool-level policy for tools/call requests, and writes permitted messages to
// dst. Denied tools/call requests are answered directly to out.
func forwardRequests(src io.Reader, dst io.Writer, out *syncWriter, policy *Policy, project string) {
	sc := bufio.NewScanner(src)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// not valid JSON; pass through unchanged
			_, _ = dst.Write(append(line, '\n'))
			continue
		}
		if msg.Method == "tools/call" {
			var p toolsCallParams
			if err := json.Unmarshal(msg.Params, &p); err == nil {
				if err := policy.CheckTool(p.Name); err != nil {
					slog.Warn("mcpproxy: tool call denied", "project", project, "tool", p.Name)
					errResp, _ := json.Marshal(rpcMessage{
						JSONRPC: "2.0",
						ID:      msg.ID,
						Error:   &rpcError{Code: -32601, Message: err.Error()},
					})
					_, _ = out.Write(append(errResp, '\n'))
					continue
				}
			}
		}
		_, _ = dst.Write(append(line, '\n'))
	}
}

// forwardResponses reads newline-delimited JSON-RPC lines from src, filters
// tools/list results by policy, and writes to dst.
func forwardResponses(src io.Reader, dst *syncWriter, policy *Policy) {
	sc := bufio.NewScanner(src)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			_, _ = dst.Write(append(line, '\n'))
			continue
		}
		if msg.Result != nil && msg.Method == "" {
			filtered, ok := filterToolsList(msg, policy)
			if ok {
				out, _ := json.Marshal(filtered)
				_, _ = dst.Write(append(out, '\n'))
				continue
			}
		}
		_, _ = dst.Write(append(line, '\n'))
	}
}

// filterToolsList rewrites a tools/list response, removing disallowed tools.
// Returns the rewritten message and true if the result contained a tools array.
func filterToolsList(msg rpcMessage, policy *Policy) (rpcMessage, bool) {
	var res toolsListResult
	if err := json.Unmarshal(msg.Result, &res); err != nil || res.Tools == nil {
		return msg, false
	}
	allowed := make([]toolEntry, 0, len(res.Tools))
	for _, t := range res.Tools {
		if policy.CheckTool(t.Name) == nil {
			allowed = append(allowed, t)
		}
	}
	res.Tools = allowed
	newResult, err := json.Marshal(res)
	if err != nil {
		return msg, false
	}
	msg.Result = newResult
	return msg, true
}
