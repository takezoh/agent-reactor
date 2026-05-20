package codexclient_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
)

// emitAndCapture is a helper that creates a Server backed by a write pipe
// and a scanner that captures the emitted lines.
func emitAndCapture(t *testing.T, emit func(*codexclient.Server)) map[string]any {
	t.Helper()
	pr, pw := io.Pipe()
	tr := codexclient.StdioTransport(io.NopCloser(io.LimitReader(nil, 0)), pw)
	conn := codexclient.NewConn(tr, time.Second)
	srv := codexclient.NewServer(conn)

	done := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(pr)
		if scanner.Scan() {
			done <- scanner.Text()
		}
	}()

	emit(srv)

	select {
	case line := <-done:
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for emitted message")
		return nil
	}
}

func TestServer_EmitTurnCompleted(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitTurnCompleted("tid1", "turn1", "hello"); err != nil {
			t.Fatalf("EmitTurnCompleted: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodTurnCompleted {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodTurnCompleted)
	}
	params, _ := msg["params"].(map[string]any)
	if params["threadId"] != "tid1" {
		t.Fatalf("params = %v", params)
	}
}

func TestServer_EmitTurnFailed(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitTurnFailed("tid1", "something broke"); err != nil {
			t.Fatalf("EmitTurnFailed: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodError {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodError)
	}
	params, _ := msg["params"].(map[string]any)
	if params["message"] != "something broke" {
		t.Fatalf("params = %v", params)
	}
}

func TestServer_EmitThreadStarted(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitThreadStarted("t1", "/work"); err != nil {
			t.Fatalf("EmitThreadStarted: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodThreadStarted {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodThreadStarted)
	}
}

func TestServer_EmitAgentMessageDelta(t *testing.T) {
	msg := emitAndCapture(t, func(s *codexclient.Server) {
		if err := s.EmitAgentMessageDelta("t1", "partial text"); err != nil {
			t.Fatalf("EmitAgentMessageDelta: %v", err)
		}
	})
	if msg["method"] != codexschema.MethodItemAgentMessageDelta {
		t.Fatalf("method = %v, want %v", msg["method"], codexschema.MethodItemAgentMessageDelta)
	}
	params, _ := msg["params"].(map[string]any)
	if params["delta"] != "partial text" {
		t.Fatalf("params = %v", params)
	}
}

func TestClient_Initialize(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go connB.Run(ctx, &initHandler{conn: connB}) //nolint:errcheck
	go connA.Run(ctx, &noopHandler{})            //nolint:errcheck

	if err := codexclient.Initialize(connA); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
}

type initHandler struct{ conn *codexclient.Conn }

func (h *initHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *initHandler) OnServerRequest(id int64, _ string, _ json.RawMessage) {
	_ = h.conn.Reply(id, map[string]any{})
}
