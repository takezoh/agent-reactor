//go:build legacy_session

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

func TestEncodeEvent(t *testing.T) {
	out := encodeEvent(1.5, termvt.Event{Kind: termvt.EventOutput, Data: []byte("hi")})
	var arr []any
	if err := json.Unmarshal(out, &arr); err != nil || len(arr) != 3 || arr[1] != "o" || arr[2] != "hi" {
		t.Fatalf("output frame = %s (err %v)", out, err)
	}

	ctl := encodeEvent(0, termvt.Event{Kind: termvt.EventControl, Ctl: termvt.Control{Kind: "osc", Code: 9, Data: "n"}})
	var m controlMsg
	if err := json.Unmarshal(ctl, &m); err != nil || m.K != "osc" || m.Code != 9 || m.Data != "n" {
		t.Fatalf("control frame = %s (err %v)", ctl, err)
	}

	ex := encodeEvent(0, termvt.Event{Kind: termvt.EventExit})
	if err := json.Unmarshal(ex, &m); err != nil || m.K != "exit" {
		t.Fatalf("exit frame = %s (err %v)", ex, err)
	}
}

// TestAttachWSEcho exercises the full http → websocket → termvt → frame path:
// attach to a cat session, send input, read the echoed output frame.
func TestAttachWSEcho(t *testing.T) {
	sess, err := termvt.NewSession(termvt.Spec{Argv: []string{"cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_ = AttachWS(r.Context(), sess, c)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"k":"i","d":"wshello\n"}`)); err != nil {
		t.Fatal(err)
	}
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var arr []any
		if json.Unmarshal(data, &arr) == nil && len(arr) == 3 && arr[1] == "o" &&
			strings.Contains(arr[2].(string), "wshello") {
			return
		}
	}
}

func TestAttachWSResize(t *testing.T) {
	sess, err := termvt.NewSession(termvt.Spec{Argv: []string{"sleep", "2"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_ = AttachWS(r.Context(), sess, c)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"k":"r","cols":120,"rows":40}`)); err != nil {
		t.Fatal(err)
	}
	// Give the reader goroutine time to apply the resize.
	time.Sleep(200 * time.Millisecond)
	if cols, rows := sess.Size(); cols != 120 || rows != 40 {
		t.Fatalf("resize not applied via ws: got %dx%d", cols, rows)
	}
}
