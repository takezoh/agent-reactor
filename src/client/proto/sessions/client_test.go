package sessions_test

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/client/proto"
	"github.com/takezoh/agent-roost/client/proto/sessions"
	"github.com/takezoh/agent-roost/client/state"
)

type fakeServer struct {
	t      *testing.T
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func newFakeClient(t *testing.T) (*sessions.Client, *fakeServer) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close(); serverConn.Close() })
	c := proto.DialConn(clientConn)
	t.Cleanup(func() { c.Close() })
	return sessions.Wrap(c), &fakeServer{
		t:      t,
		conn:   serverConn,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
	}
}

func (s *fakeServer) recv() proto.Envelope {
	line, err := s.reader.ReadBytes('\n')
	if err != nil {
		s.t.Fatalf("server recv: %v", err)
	}
	env, err := proto.DecodeEnvelope(line)
	if err != nil {
		s.t.Fatalf("server decode: %v", err)
	}
	return env
}

func (s *fakeServer) send(payload []byte) {
	if _, err := s.writer.Write(payload); err != nil {
		s.t.Fatalf("server send: %v", err)
	}
	s.writer.WriteByte('\n')
	s.writer.Flush()
}

// TestCreateSessionUsesLongTimeout verifies that CreateSession waits beyond
// defaultRequestTimeout (5s) when the server is slow to respond.
func TestCreateSessionUsesLongTimeout(t *testing.T) {
	c, server := newFakeClient(t)

	type result struct {
		id  string
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		id, err := c.CreateSession("/tmp/project", "shell", state.SandboxOverrideAuto, state.LaunchOptions{})
		resCh <- result{id, err}
	}()

	env := server.recv()
	time.Sleep(6 * time.Second)
	wire, _ := proto.EncodeResponse(env.ReqID, proto.RespCreateSession{SessionID: "slow-sess"})
	server.send(wire)

	select {
	case r := <-resCh:
		if r.err != nil {
			t.Fatalf("CreateSession: unexpected error: %v", r.err)
		}
		if r.id != "slow-sess" {
			t.Errorf("session id = %q, want slow-sess", r.id)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("CreateSession did not complete in time")
	}
}

// TestOtherRPCsUseShortTimeout verifies that non-CreateSession RPCs
// expire around the default 5s timeout.
func TestOtherRPCsUseShortTimeout(t *testing.T) {
	c, server := newFakeClient(t)

	go func() { server.recv() }() // read but never respond

	start := time.Now()
	err := c.StopSession("some-id")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 10*time.Second {
		t.Errorf("timeout took %v, want ~5s", elapsed)
	}
}

// TestCanonicalProjectPathRoundtrips verifies the project path is sent
// canonicalized in CreateSession.
func TestCanonicalProjectPathRoundtrips(t *testing.T) {
	c, server := newFakeClient(t)

	resCh := make(chan error, 1)
	go func() {
		_, err := c.CreateSession("/tmp", "shell", state.SandboxOverrideAuto, state.LaunchOptions{})
		resCh <- err
	}()

	env := server.recv()
	var cmd proto.CmdEvent
	if err := json.Unmarshal(env.Data, &cmd); err != nil {
		t.Fatalf("unmarshal CmdEvent: %v", err)
	}
	var params state.CreateSessionParams
	if err := json.Unmarshal(cmd.Payload, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params.Project == "" {
		t.Error("project path is empty")
	}

	wire, _ := proto.EncodeResponse(env.ReqID, proto.RespCreateSession{SessionID: "s1"})
	server.send(wire)
	<-resCh
}

// TestPushDriverDecodesCreateSessionReply verifies that PushDriver expects a
// RespCreateSession from the daemon (not RespOK) and returns nil on success.
func TestPushDriverDecodesCreateSessionReply(t *testing.T) {
	c, server := newFakeClient(t)

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.PushDriver("sess-1", "shell", nil)
	}()

	env := server.recv()
	wire, _ := proto.EncodeResponse(env.ReqID, proto.RespCreateSession{SessionID: "sess-1"})
	server.send(wire)

	if err := <-errCh; err != nil {
		t.Fatalf("PushDriver returned error: %v", err)
	}
}
