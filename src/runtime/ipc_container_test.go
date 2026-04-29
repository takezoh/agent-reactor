package runtime

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/proto"
	"github.com/takezoh/agent-roost/state"
)

func newTestContainerEndpoint(t *testing.T) (ep *containerEndpoint, sockPath string, events chan state.Event) {
	t.Helper()
	dir := t.TempDir()
	sockPath = filepath.Join(dir, "roost.sock")
	events = make(chan state.Event, 8)

	var tokens tokenStore
	frameID := state.FrameID("test-frame")
	token, err := tokens.Generate(frameID)
	if err != nil {
		t.Fatalf("token generate: %v", err)
	}
	t.Logf("test token: %s → %s", token, frameID)

	ep, err = startContainerEndpoint(sockPath, &tokens, func(ev state.Event) {
		events <- ev
	}, nil)
	if err != nil {
		t.Fatalf("startContainerEndpoint: %v", err)
	}
	t.Cleanup(func() { ep.close() })

	return ep, sockPath, events
}

func sendRawCommand(t *testing.T, sockPath string, cmd proto.Command) (proto.Envelope, error) {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return proto.Envelope{}, err
	}
	defer conn.Close()

	wire, err := proto.EncodeCommand("req-1", cmd)
	if err != nil {
		return proto.Envelope{}, err
	}

	if _, err := conn.Write(append(wire, '\n')); err != nil {
		return proto.Envelope{}, err
	}

	dec := json.NewDecoder(conn)
	var env proto.Envelope
	if err := dec.Decode(&env); err != nil {
		return proto.Envelope{}, err
	}
	return env, nil
}

func TestContainerEndpointAcceptsHookEvent(t *testing.T) {
	_, sockPath, events := newTestContainerEndpoint(t)

	var tokens tokenStore
	frameID := state.FrameID("test-frame")
	token, _ := tokens.Generate(frameID)

	// Use the real token stored in the endpoint's tokenStore by re-reading it.
	// Recreate: we need the token that was generated in newTestContainerEndpoint.
	// Simplest approach: create fresh endpoint with known token.
	dir := t.TempDir()
	sp2 := filepath.Join(dir, "roost.sock")
	var ts2 tokenStore
	fid2 := state.FrameID("frame-hook")
	tok2, _ := ts2.Generate(fid2)
	evCh := make(chan state.Event, 4)
	ep2, err := startContainerEndpoint(sp2, &ts2, func(ev state.Event) { evCh <- ev }, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(ep2.close)
	_ = token
	_ = sockPath
	_ = events

	payload, _ := json.Marshal(map[string]string{"result": "ok"})
	env, err := sendRawCommand(t, sp2, proto.CmdHookEvent{
		Token:     tok2,
		Hook:      "Stop",
		Timestamp: time.Now(),
		Payload:   json.RawMessage(payload),
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if env.Status != proto.StatusOK {
		t.Fatalf("expected ok, got status=%q error=%v", env.Status, env.Error)
	}

	select {
	case ev := <-evCh:
		de, ok := ev.(state.EvDriverEvent)
		if !ok {
			t.Fatalf("expected EvDriverEvent, got %T", ev)
		}
		if de.Event != "Stop" {
			t.Fatalf("hook name: got %q, want %q", de.Event, "Stop")
		}
		if de.SenderID != fid2 {
			t.Fatalf("sender: got %q, want %q", de.SenderID, fid2)
		}
		if de.ConnID != 0 {
			t.Fatalf("ConnID: got %v, want 0", de.ConnID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout: no event received")
	}
}

func TestContainerEndpointRejectsUnknownCommands(t *testing.T) {
	rejectedCmds := []struct {
		name string
		cmd  proto.Command
	}{
		{"event", proto.CmdEvent{Event: "push-driver"}},
		{"subscribe", proto.CmdSubscribe{}},
		{"surface.send_text", proto.CmdSurfaceSendText{SessionID: "s1", Text: "hi"}},
		{"surface.send_key", proto.CmdSurfaceSendKey{SessionID: "s1", Key: "C-c"}},
		{"surface.read_text", proto.CmdSurfaceReadText{SessionID: "s1"}},
		{"peer.send", proto.CmdPeerSend{FromFrameID: "f1", ToFrameID: "f2", Text: "x"}},
		{"driver.list", proto.CmdDriverList{}},
	}

	dir := t.TempDir()
	sp := filepath.Join(dir, "roost.sock")
	var ts tokenStore
	ep, err := startContainerEndpoint(sp, &ts, func(state.Event) {}, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(ep.close)

	for _, tc := range rejectedCmds {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			env, err := sendRawCommand(t, sp, tc.cmd)
			if err != nil {
				t.Fatalf("send: %v", err)
			}
			if env.Status == proto.StatusOK {
				t.Fatalf("expected error response, got ok")
			}
			if env.Error == nil || env.Error.Code != proto.ErrUnsupported {
				t.Fatalf("expected ErrUnsupported, got %v", env.Error)
			}
		})
	}
}

func TestContainerEndpointRejectsInvalidToken(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "roost.sock")
	var ts tokenStore
	ts.Generate(state.FrameID("f1"))
	ep, err := startContainerEndpoint(sp, &ts, func(state.Event) {}, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(ep.close)

	env, err := sendRawCommand(t, sp, proto.CmdHookEvent{
		Token: "bad-token-value",
		Hook:  "Stop",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if env.Status == proto.StatusOK {
		t.Fatal("expected error, got ok")
	}
	if env.Error == nil || env.Error.Code != proto.ErrInvalidArgument {
		t.Fatalf("expected ErrInvalidArgument, got %v", env.Error)
	}
}

func TestContainerEndpointRejectsRevokedToken(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "roost.sock")
	var ts tokenStore
	fid := state.FrameID("frame-revoked")
	tok, _ := ts.Generate(fid)
	ts.Revoke(fid)

	ep, err := startContainerEndpoint(sp, &ts, func(state.Event) {}, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(ep.close)

	env, err := sendRawCommand(t, sp, proto.CmdHookEvent{
		Token: tok,
		Hook:  "Stop",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if env.Status == proto.StatusOK {
		t.Fatal("expected error, got ok")
	}
}

func TestTranslatePayloadPaths(t *testing.T) {
	ms := pathmap.Mounts{
		{Host: "/home/u/myapp", Container: "/workspaces/myapp"},
	}
	frameID := state.FrameID("f1")
	getMounts := func(id state.FrameID) (pathmap.Mounts, bool) {
		if id == frameID {
			return ms, true
		}
		return nil, false
	}
	ep := &containerEndpoint{getMounts: getMounts}

	t.Run("cwd translated to host and container_cwd preserved", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{
			"cwd":             "/workspaces/myapp/backend",
			"hook_event_name": "SessionStart",
		})
		out := ep.translatePayloadPaths(frameID, json.RawMessage(payload))
		var fields map[string]string
		if err := json.Unmarshal(out, &fields); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if fields["cwd"] != "/home/u/myapp/backend" {
			t.Errorf("cwd = %q, want /home/u/myapp/backend", fields["cwd"])
		}
		if fields["container_cwd"] != "/workspaces/myapp/backend" {
			t.Errorf("container_cwd = %q, want /workspaces/myapp/backend", fields["container_cwd"])
		}
	})

	t.Run("transcript_path translated to host", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{
			"transcript_path": "/workspaces/myapp/.claude/session.jsonl",
		})
		out := ep.translatePayloadPaths(frameID, json.RawMessage(payload))
		var fields map[string]string
		if err := json.Unmarshal(out, &fields); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if fields["transcript_path"] != "/home/u/myapp/.claude/session.jsonl" {
			t.Errorf("transcript_path = %q, want /home/u/myapp/.claude/session.jsonl", fields["transcript_path"])
		}
	})

	t.Run("transcript_path outside mount cleared", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{
			"transcript_path": "/home/ubuntu/.claude/projects/foo/session.jsonl",
		})
		out := ep.translatePayloadPaths(frameID, json.RawMessage(payload))
		var fields map[string]string
		if err := json.Unmarshal(out, &fields); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if fields["transcript_path"] != "" {
			t.Errorf("transcript_path = %q, want empty (not reachable from host)", fields["transcript_path"])
		}
	})

	t.Run("cwd outside mount cleared", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{"cwd": "/var/log"})
		out := ep.translatePayloadPaths(frameID, json.RawMessage(payload))
		var fields map[string]string
		if err := json.Unmarshal(out, &fields); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if fields["cwd"] != "" {
			t.Errorf("cwd = %q, want empty", fields["cwd"])
		}
	})

	t.Run("no mounts for frame: payload unchanged", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{"cwd": "/workspaces/myapp"})
		unknownFrame := state.FrameID("unknown")
		out := ep.translatePayloadPaths(unknownFrame, json.RawMessage(payload))
		var fields map[string]string
		if err := json.Unmarshal(out, &fields); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if fields["cwd"] != "/workspaces/myapp" {
			t.Errorf("cwd = %q, want /workspaces/myapp (unchanged)", fields["cwd"])
		}
	})

	t.Run("payload without cwd: untouched", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{"hook_event_name": "Stop"})
		out := ep.translatePayloadPaths(frameID, json.RawMessage(payload))
		if string(out) == string(payload) {
			// unchanged is fine — but check it at least parses
		}
		var fields map[string]string
		if err := json.Unmarshal(out, &fields); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, has := fields["cwd"]; has {
			t.Error("cwd should not be present")
		}
	})
}

// TestContainerEndpointTranslatesCwd verifies that a hook event arriving
// with a container-absolute cwd is enqueued with the host-absolute path.
func TestContainerEndpointTranslatesCwd(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "roost.sock")

	var ts tokenStore
	frameID := state.FrameID("frame-translate")
	tok, _ := ts.Generate(frameID)

	ms := pathmap.Mounts{
		{Host: "/home/u/proj", Container: "/workspaces/proj"},
	}
	getMounts := func(id state.FrameID) (pathmap.Mounts, bool) {
		if id == frameID {
			return ms, true
		}
		return nil, false
	}

	evCh := make(chan state.Event, 4)
	ep, err := startContainerEndpoint(sp, &ts, func(ev state.Event) { evCh <- ev }, getMounts)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(ep.close)

	payload, _ := json.Marshal(map[string]string{
		"hook_event_name": "SessionStart",
		"cwd":             "/workspaces/proj/src",
	})
	if _, err := sendRawCommand(t, sp, proto.CmdHookEvent{
		Token:     tok,
		Hook:      "SessionStart",
		Timestamp: time.Now(),
		Payload:   json.RawMessage(payload),
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case ev := <-evCh:
		de := ev.(state.EvDriverEvent)
		var fields map[string]string
		if err := json.Unmarshal(de.Payload, &fields); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if fields["cwd"] != "/home/u/proj/src" {
			t.Errorf("translated cwd = %q, want /home/u/proj/src", fields["cwd"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout: no event received")
	}
}

func TestContainerEndpointSocketPermissions(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "roost.sock")
	var ts tokenStore
	ep, err := startContainerEndpoint(sp, &ts, func(state.Event) {}, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer ep.close()

	info, err := os.Stat(sp)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// chmod 0o666 — world-writable so all container processes can connect.
	// Bearer token is the real auth boundary.
	if perm := info.Mode().Perm(); perm != 0o666 {
		t.Fatalf("expected 0666 permissions, got %04o", perm)
	}
}
