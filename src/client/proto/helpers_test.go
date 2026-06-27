package proto

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// roundTripWith replies to any incoming command with the given response.
func roundTripWith(t *testing.T, srv *fakeServer, resp Response) {
	t.Helper()
	env := srv.recv()
	wire, _ := EncodeResponse(env.ReqID, resp)
	srv.send(wire)
}

func TestSendHookEvent(t *testing.T) {
	c, srv := newFakeServer(t)
	defer c.Close()
	go roundTripWith(t, srv, RespOK{})
	if err := c.SendHookEvent("tok", "PreToolUse", time.Now(), json.RawMessage(`{}`)); err != nil {
		t.Errorf("SendHookEvent: %v", err)
	}
}

func TestSendEvent(t *testing.T) {
	c, srv := newFakeServer(t)
	defer c.Close()
	go roundTripWith(t, srv, RespOK{})
	if err := c.SendEvent("evt", time.Now(), "sender", nil); err != nil {
		t.Errorf("SendEvent: %v", err)
	}
}

func TestSendSubsystemEvent(t *testing.T) {
	c, srv := newFakeServer(t)
	defer c.Close()
	go roundTripWith(t, srv, RespOK{})
	if err := c.SendSubsystemEvent("tok", "f", "stream", "msg", time.Now(), nil); err != nil {
		t.Errorf("SendSubsystemEvent: %v", err)
	}
}

func TestSendNoWait(t *testing.T) {
	c, srv := newFakeServer(t)
	defer c.Close()
	done := make(chan struct{})
	go func() {
		srv.recv()
		close(done)
	}()
	if err := c.SendNoWait(CmdEvent{Event: "x"}); err != nil {
		t.Errorf("SendNoWait: %v", err)
	}
	<-done
}

func TestSendContextCancelledMidFlight(t *testing.T) {
	c, srv := newFakeServer(t)
	defer c.Close()
	// drain raw bytes so writeFrame doesn't block on the pipe
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := srv.conn.Read(buf); err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := c.Send(ctx, CmdEvent{Event: "x"}); err == nil {
		t.Error("expected ctx error (no reply scripted)")
	}
}

func TestSendAfterClose(t *testing.T) {
	c, _ := newFakeServer(t)
	c.Close()
	if _, err := c.Send(context.Background(), CmdEvent{Event: "x"}); err == nil {
		t.Error("expected closed error")
	}
}
