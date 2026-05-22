package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/orchestrator/lineargql"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// --- Activity tracking tests (§13.5 / §8.5) ---

func TestTurnHandler_ActivityReportedOnEveryNotification(t *testing.T) {
	var got []scheduler.CodexActivity
	h := &turnHandler{
		issueID:      "iss-1",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		report:       func(a scheduler.CodexActivity) { got = append(got, a) },
	}

	before := time.Now()
	h.OnNotification("some/method", nil)
	after := time.Now()

	require.Len(t, got, 1)
	require.Equal(t, "iss-1", got[0].IssueID)
	require.Equal(t, "some/method", got[0].Event)
	require.False(t, got[0].Timestamp.Before(before))
	require.False(t, got[0].Timestamp.After(after))
	require.Nil(t, got[0].Usage)
	require.Nil(t, got[0].RateLimit)
}

// TestTurnHandler_UsageUsesTotalIgnoresLastPayload verifies §13.5 (a):
// the delta-style "last" payload must be ignored; only "total" (absolute) is used.
func TestTurnHandler_UsageUsesTotalIgnoresLastPayload(t *testing.T) {
	var got []scheduler.CodexActivity
	h := &turnHandler{
		issueID:      "iss-2",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		report:       func(a scheduler.CodexActivity) { got = append(got, a) },
	}

	params, _ := json.Marshal(map[string]any{
		"threadId": "thread1",
		"turnId":   "turn1",
		"tokenUsage": map[string]any{
			"total": map[string]any{
				"inputTokens":  int64(100),
				"outputTokens": int64(50),
				"totalTokens":  int64(150),
			},
			"last": map[string]any{
				"inputTokens":  int64(10), // delta-style payload — must be ignored
				"outputTokens": int64(5),
				"totalTokens":  int64(15),
			},
		},
	})
	h.OnNotification("thread/tokenUsage/updated", params)

	require.Len(t, got, 1)
	require.NotNil(t, got[0].Usage)
	require.Equal(t, "thread1", got[0].Usage.ThreadID)
	require.Equal(t, int64(100), got[0].Usage.Input) // Total.inputTokens, not Last
	require.Equal(t, int64(50), got[0].Usage.Output)
	require.Equal(t, int64(150), got[0].Usage.Total)
}

func TestTurnHandler_RateLimitReported(t *testing.T) {
	var got []scheduler.CodexActivity
	h := &turnHandler{
		issueID:      "iss-3",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		report:       func(a scheduler.CodexActivity) { got = append(got, a) },
	}

	resetsAt := int64(1234567890000)
	params, _ := json.Marshal(map[string]any{
		"rateLimits": map[string]any{
			"primary": map[string]any{
				"usedPercent": int64(75),
				"resetsAt":    resetsAt,
			},
		},
	})
	h.OnNotification("account/rateLimits/updated", params)

	require.Len(t, got, 1)
	require.NotNil(t, got[0].RateLimit)
	require.Equal(t, int64(75), got[0].RateLimit.PrimaryUsedPercent)
	require.Equal(t, resetsAt, got[0].RateLimit.PrimaryResetsAt)
	require.Equal(t, int64(0), got[0].RateLimit.SecondaryUsedPercent)
}

func TestTurnHandler_TurnDurationTracked(t *testing.T) {
	var got []scheduler.CodexActivity
	h := &turnHandler{
		issueID:      "iss-4",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		report:       func(a scheduler.CodexActivity) { got = append(got, a) },
	}

	params, _ := json.Marshal(map[string]any{"turnId": "t1"})
	h.OnNotification("turn/started", params)
	time.Sleep(time.Millisecond)
	h.OnNotification("turn/completed", nil)

	require.Len(t, got, 2)
	require.Nil(t, got[0].TurnDuration, "turn/started must not carry TurnDuration")
	require.NotNil(t, got[1].TurnDuration, "turn/completed must carry TurnDuration")
	require.Greater(t, *got[1].TurnDuration, time.Duration(0))
}

func TestTurnHandler_TurnCompletedFlag(t *testing.T) {
	var got []scheduler.CodexActivity
	h := &turnHandler{
		issueID:      "iss-7",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		report:       func(a scheduler.CodexActivity) { got = append(got, a) },
	}

	// Non-completed event must not set TurnCompleted.
	h.OnNotification("turn/started", nil)
	require.Len(t, got, 1)
	require.False(t, got[0].TurnCompleted, "turn/started must not set TurnCompleted")

	// turn/completed must set TurnCompleted=true.
	h.OnNotification("turn/completed", nil)
	require.Len(t, got, 2)
	require.True(t, got[1].TurnCompleted, "turn/completed must set TurnCompleted=true")
}

func TestTurnHandler_OtherEventsDoNotSetTurnCompleted(t *testing.T) {
	var got []scheduler.CodexActivity
	h := &turnHandler{
		issueID:      "iss-8",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		report:       func(a scheduler.CodexActivity) { got = append(got, a) },
	}

	h.OnNotification("item/agentMessage/delta", nil)
	require.Len(t, got, 1)
	require.False(t, got[0].TurnCompleted, "non-turn/completed event must not set TurnCompleted")
}

func TestTurnHandler_AgentMessageDeltaRecorded(t *testing.T) {
	var got []scheduler.CodexActivity
	h := &turnHandler{
		issueID:      "iss-5",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		report:       func(a scheduler.CodexActivity) { got = append(got, a) },
	}

	params, _ := json.Marshal(map[string]any{
		"delta":    "hello world",
		"itemId":   "i1",
		"threadId": "t1",
		"turnId":   "u1",
	})
	h.OnNotification("item/agentMessage/delta", params)

	require.Len(t, got, 1)
	require.Equal(t, "hello world", got[0].Message)
}

func TestTurnHandler_NilReportNoPanic(t *testing.T) {
	h := &turnHandler{
		issueID:      "iss-6",
		sessionReady: make(chan sessionIDs, 1),
		turnDone:     make(chan turnResult, 1),
		// report is nil — must not panic
	}
	h.OnNotification("some/method", nil)
}

// --- linear_graphql tool call tests (SPEC §10.5) ---

// toolCallServer is a fake codex app-server that:
//  1. Replies to initialize.
//  2. On turn/start: emits thread/started + turn/started.
//  3. Sends one item/tool/call request to the orchestrator and captures the reply.
//  4. After capturing the reply, emits turn/completed to end the session.
type toolCallServer struct {
	srv      *codexclient.Server
	toolName string
	args     json.RawMessage
	reply    json.RawMessage
	replyErr string
}

func (s *toolCallServer) OnServerRequest(id int64, method string, _ json.RawMessage) {
	switch method {
	case codexschema.MethodInitialize:
		_ = s.srv.Conn().Reply(id, map[string]any{})
	case codexschema.MethodThreadStart:
		_ = s.srv.Conn().Reply(id, map[string]any{"thread": map[string]any{"id": testThreadID}})
	}
}

func (s *toolCallServer) OnNotification(method string, _ json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		return
	}
	_ = s.srv.EmitThreadStarted(testThreadID, "/ws")
	_ = s.srv.EmitTurnStarted(testThreadID, testTurnID)

	go func() {
		raw, err := s.srv.Conn().Request(codexschema.MethodItemToolCall, map[string]any{
			"tool":      s.toolName,
			"arguments": s.args,
			"callId":    "call-1",
			"threadId":  testThreadID,
			"turnId":    testTurnID,
		})
		if err != nil {
			s.replyErr = err.Error()
		} else {
			s.reply = raw
		}
		_ = s.srv.EmitTurnCompleted(testThreadID, testTurnID, "done")
	}()
}

// makeToolCallProc wires runner ↔ toolCallServer via io.Pipe, same pattern as
// makeFakeProc in runner_test.go.
func makeToolCallProc(ts *toolCallServer) procFunc {
	return func(ctx context.Context, _ string, _ map[string]string, _ string) (io.ReadCloser, io.WriteCloser, func(), error) {
		pr1, pw1 := io.Pipe()
		pr2, pw2 := io.Pipe()
		serverConn := codexclient.NewConn(codexclient.StdioTransport(pr2, pw1), 2*time.Second)
		ts.srv = codexclient.NewServer(serverConn)
		go func() {
			defer pw2.Close()
			_ = serverConn.Run(ctx, ts)
		}()
		go func() {
			<-ctx.Done()
			_ = pw1.Close()
		}()
		return pr1, pw2, func() {}, nil
	}
}

func makeLinearServer(t *testing.T, respBody string) *lineargql.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(respBody)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return lineargql.New(srv.URL, "test-token")
}

func makeRunnerWithLinear(t *testing.T, lc *lineargql.Client, proc procFunc) *Runner {
	t.Helper()
	r := makeRunner(t, "", proc)
	r.LinearClient = lc
	return r
}

func testIssue() tracker.Issue {
	return tracker.Issue{Identifier: "PROJ-H1", Title: "handler test issue"}
}

func spawnAndWaitForToolReply(t *testing.T, r *Runner, ts *toolCallServer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.spawnWith(ctx, testIssue(), 1, func(Event) {})
	require.NoError(t, err)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && ts.reply == nil && ts.replyErr == "" {
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHandleToolCall_linearGraphql_success(t *testing.T) {
	lc := makeLinearServer(t, `{"data":{"viewer":{"id":"u1"}}}`)
	args, _ := json.Marshal(map[string]any{"query": "{ viewer { id } }", "variables": nil})
	ts := &toolCallServer{toolName: "linear_graphql", args: args}

	r := makeRunnerWithLinear(t, lc, makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	require.Empty(t, ts.replyErr)
	require.NotEmpty(t, ts.reply)
	var reply toolCallReply
	require.NoError(t, json.Unmarshal(ts.reply, &reply))
	assert.True(t, reply.Success)
	// output is the JSON-encoded lineargql.Result
	var inner lineargql.Result
	require.NoError(t, json.Unmarshal([]byte(reply.Output), &inner))
	assert.True(t, inner.Success)
	assert.JSONEq(t, `{"viewer":{"id":"u1"}}`, string(inner.Data))
}

func TestHandleToolCall_linearGraphql_graphqlErrors(t *testing.T) {
	lc := makeLinearServer(t, `{"errors":[{"message":"not found"},{"message":"forbidden"}]}`)
	args, _ := json.Marshal(map[string]any{"query": "{ issue { id } }", "variables": nil})
	ts := &toolCallServer{toolName: "linear_graphql", args: args}

	r := makeRunnerWithLinear(t, lc, makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	require.Empty(t, ts.replyErr)
	require.NotEmpty(t, ts.reply)
	var reply toolCallReply
	require.NoError(t, json.Unmarshal(ts.reply, &reply))
	assert.False(t, reply.Success)
	assert.Contains(t, reply.Output, "not found")
	assert.Contains(t, reply.Output, "forbidden")
}

func TestHandleToolCall_unknownTool_replyError(t *testing.T) {
	lc := makeLinearServer(t, `{"data":{}}`)
	args, _ := json.Marshal(map[string]any{"query": "x"})
	ts := &toolCallServer{toolName: "nonexistent_tool", args: args}

	r := makeRunnerWithLinear(t, lc, makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	assert.NotEmpty(t, ts.replyErr, "unknown tool should return a JSON-RPC error")
}

func TestHandleToolCall_linearDisabled_replyError(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"query": "{ viewer { id } }"})
	ts := &toolCallServer{toolName: "linear_graphql", args: args}

	// runner with no LinearClient — tool is disabled
	r := makeRunner(t, "", makeToolCallProc(ts))
	spawnAndWaitForToolReply(t, r, ts)

	assert.NotEmpty(t, ts.replyErr, "disabled linear_graphql should return a JSON-RPC error")
}
