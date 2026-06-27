package sessions_test

import (
	"testing"

	"github.com/takezoh/agent-reactor/client/proto"
)

func reply(t *testing.T, srv *fakeServer, resp proto.Response) {
	t.Helper()
	env := srv.recv()
	wire, _ := proto.EncodeResponse(env.ReqID, resp)
	srv.send(wire)
}

func TestSubscribe(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.Subscribe(); err != nil {
		t.Errorf("Subscribe: %v", err)
	}
}

func TestListSessions(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespSessions{
		Sessions:        []proto.SessionInfo{{ID: "s1"}},
		ActiveSessionID: "s1",
		Features:        []string{"f1"},
	})
	sessions, active, features, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || active != "s1" || len(features) != 1 {
		t.Errorf("unexpected: %+v %q %v", sessions, active, features)
	}
}

func TestPreviewSession(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespActiveSession{ActiveSessionID: "s1"})
	got, err := c.PreviewSession("s1")
	if err != nil || got != "s1" {
		t.Errorf("got %q err %v", got, err)
	}
}

func TestSwitchSession(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespActiveSession{ActiveSessionID: "s1"})
	got, err := c.SwitchSession("s1")
	if err != nil || got != "s1" {
		t.Errorf("got %q err %v", got, err)
	}
}

func TestPreviewProject(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.PreviewProject("/p"); err != nil {
		t.Errorf("PreviewProject: %v", err)
	}
}

func TestShutdown(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.Shutdown(); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestActivateFrame(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.ActivateFrame("s", "f"); err != nil {
		t.Errorf("ActivateFrame: %v", err)
	}
}

func TestForkSession(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespCreateSession{SessionID: "new"})
	id, err := c.ForkSession("orig")
	if err != nil || id != "new" {
		t.Errorf("got %q err %v", id, err)
	}
}
