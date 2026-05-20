package codexclient_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
)

// pipeTransport wires two StdioTransports back-to-back for in-process tests.
func pipeTransport() (codexclient.Transport, codexclient.Transport) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	return codexclient.StdioTransport(pr1, pw2), codexclient.StdioTransport(pr2, pw1)
}

// discardWriteTransport lets writes succeed immediately and blocks reads forever.
func discardWriteTransport() codexclient.Transport {
	pr, _ := io.Pipe() // nobody writes to this end → reads block
	return codexclient.StdioTransport(pr, io.Discard)
}

func TestConn_RequestResponse(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// connB: echo params as result.
	go connB.Run(ctx, &echoHandler{conn: connB}) //nolint:errcheck
	// connA: needs a read loop to receive the response.
	go connA.Run(ctx, &noopHandler{}) //nolint:errcheck

	result, err := connA.Request("ping", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["x"] != float64(1) {
		t.Fatalf("got %v, want x=1", got)
	}
}

func TestConn_Notify(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	recv := make(chan string, 1)
	h := &notifyHandler{recv: recv}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go connB.Run(ctx, h) //nolint:errcheck

	if err := connA.Notify("hello", map[string]any{"msg": "world"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	select {
	case got := <-recv:
		if !strings.Contains(got, "world") {
			t.Fatalf("got %q, want world", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestConn_RequestTimeout(t *testing.T) {
	// peer accepts writes but never replies
	tr := discardWriteTransport()
	conn := codexclient.NewConn(tr, 100*time.Millisecond)
	go conn.Run(context.Background(), &noopHandler{}) //nolint:errcheck
	_, err := conn.Request("slow", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("got %q, want timeout", err)
	}
}

func TestConn_ReplyError(t *testing.T) {
	ta, tb := pipeTransport()
	connA := codexclient.NewConn(ta, time.Second)
	connB := codexclient.NewConn(tb, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go connB.Run(ctx, &replyErrorHandler{conn: connB, msg: "not supported"}) //nolint:errcheck
	go connA.Run(ctx, &noopHandler{})                                        //nolint:errcheck

	_, err := connA.Request("bad", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("got %q, want 'not supported'", err)
	}
}

// TestStdioTransport_RoundTrip verifies newline-delimited framing.
func TestStdioTransport_RoundTrip(t *testing.T) {
	pr, pw := io.Pipe()
	trW := codexclient.StdioTransport(io.NopCloser(strings.NewReader("")), pw)
	trR := codexclient.StdioTransport(pr, io.Discard)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg := []byte(`{"method":"hello"}`)
	// Write and read must be concurrent because io.Pipe is unbuffered.
	writeErr := make(chan error, 1)
	go func() { writeErr <- trW.WriteMessage(ctx, msg) }()

	got, err := trR.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if werr := <-writeErr; werr != nil {
		t.Fatalf("WriteMessage: %v", werr)
	}
	if string(got) != string(msg) {
		t.Fatalf("got %q, want %q", got, msg)
	}
}

// TestStdioTransport_MultiMessage verifies multiple messages in sequence.
func TestStdioTransport_MultiMessage(t *testing.T) {
	pr, pw := io.Pipe()
	scanner := bufio.NewScanner(pr)
	tr := codexclient.StdioTransport(io.NopCloser(strings.NewReader("")), pw)
	ctx := context.Background()

	msgs := []string{`{"id":1}`, `{"method":"ping"}`, `{"result":"ok"}`}
	go func() {
		for _, m := range msgs {
			_ = tr.WriteMessage(ctx, []byte(m))
		}
	}()

	for _, want := range msgs {
		if !scanner.Scan() {
			t.Fatal("scanner stopped early")
		}
		if scanner.Text() != want {
			t.Fatalf("got %q, want %q", scanner.Text(), want)
		}
	}
}

// --- test doubles ---

type echoHandler struct{ conn *codexclient.Conn }

func (h *echoHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *echoHandler) OnServerRequest(id int64, _ string, params json.RawMessage) {
	_ = h.conn.Reply(id, params)
}

type notifyHandler struct{ recv chan string }

func (h *notifyHandler) OnNotification(_ string, params json.RawMessage) {
	h.recv <- string(params)
}
func (h *notifyHandler) OnServerRequest(_ int64, _ string, _ json.RawMessage) {}

type noopHandler struct{}

func (h *noopHandler) OnNotification(_ string, _ json.RawMessage)           {}
func (h *noopHandler) OnServerRequest(_ int64, _ string, _ json.RawMessage) {}

type replyErrorHandler struct {
	conn *codexclient.Conn
	msg  string
}

func (h *replyErrorHandler) OnNotification(_ string, _ json.RawMessage) {}
func (h *replyErrorHandler) OnServerRequest(id int64, _ string, _ json.RawMessage) {
	_ = h.conn.ReplyError(id, h.msg)
}
