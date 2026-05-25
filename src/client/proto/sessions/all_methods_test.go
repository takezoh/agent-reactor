package sessions_test

import (
	"testing"

	"github.com/takezoh/agent-roost/client/proto"
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
		ActiveOccupant:  "main",
		Features:        []string{"f1"},
	})
	sessions, active, occ, _, features, err := c.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || active != "s1" || occ != "main" || len(features) != 1 {
		t.Errorf("unexpected: %+v %q %q %v", sessions, active, occ, features)
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

func TestFocusPane(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.FocusPane("0.1"); err != nil {
		t.Errorf("FocusPane: %v", err)
	}
}

func TestLaunchTool(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.LaunchTool("git", map[string]string{"arg": "v"}); err != nil {
		t.Errorf("LaunchTool: %v", err)
	}
}

func TestShutdown(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.Shutdown(); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestDetach(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.Detach(); err != nil {
		t.Errorf("Detach: %v", err)
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

func TestActivateOccupantAndAliases(t *testing.T) {
	c, srv := newFakeClient(t)
	go func() {
		for range 3 {
			reply(t, srv, proto.RespOK{})
		}
	}()
	if err := c.ActivateOccupant("frame", "s", "f"); err != nil {
		t.Errorf("ActivateOccupant: %v", err)
	}
	if err := c.ActivateLog(); err != nil {
		t.Errorf("ActivateLog: %v", err)
	}
	if err := c.ActivateMain(); err != nil {
		t.Errorf("ActivateMain: %v", err)
	}
}

func TestStatusLineClick(t *testing.T) {
	c, srv := newFakeClient(t)
	go reply(t, srv, proto.RespOK{})
	if err := c.StatusLineClick("region"); err != nil {
		t.Errorf("StatusLineClick: %v", err)
	}
}
