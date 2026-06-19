package protofake_test

import (
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/proto/protofake"
)

// buildEnvelope converts the raw bytes returned by EncodeCommand / EncodeEvent
// into a proto.Envelope so callers can pass it to Send(proto.Envelope).
func buildEnvelope(t *testing.T, raw []byte, encErr error) proto.Envelope {
	t.Helper()
	if encErr != nil {
		t.Fatalf("encode: %v", encErr)
	}
	var env proto.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return env
}

// TestNewPairRoundTrip sends a CmdSurfaceSubscribe from client to server and
// verifies the server receives the correct envelope shape.
// net.Pipe is synchronous so Send must run in its own goroutine.
func TestNewPairRoundTrip(t *testing.T) {
	t.Parallel()

	c, s := protofake.NewPair()
	defer c.Close() //nolint:errcheck
	defer s.Close() //nolint:errcheck

	raw, encErr := proto.EncodeCommand("req-1", proto.CmdSurfaceSubscribe{SessionID: "s1"})
	env := buildEnvelope(t, raw, encErr)

	sendErr := make(chan error, 1)
	go func() {
		sendErr <- c.Send(env)
	}()

	got, err := s.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if se := <-sendErr; se != nil {
		t.Fatalf("Send: %v", se)
	}

	if got.Type != proto.TypeCommand {
		t.Errorf("Type = %q, want %q", got.Type, proto.TypeCommand)
	}
	if got.Cmd != proto.CmdNameSurfaceSubscribe {
		t.Errorf("Cmd = %q, want %q", got.Cmd, proto.CmdNameSurfaceSubscribe)
	}
	if got.ReqID != "req-1" {
		t.Errorf("ReqID = %q, want %q", got.ReqID, "req-1")
	}
}

// TestServerToClientEvent sends an EvtSurfaceOutput from server to client and
// verifies the client receives the correct envelope shape.
// net.Pipe is synchronous so Send must run in its own goroutine.
func TestServerToClientEvent(t *testing.T) {
	t.Parallel()

	c, s := protofake.NewPair()
	defer c.Close() //nolint:errcheck
	defer s.Close() //nolint:errcheck

	evt := proto.EvtSurfaceOutput{
		SessionID: "s2",
		TimeSec:   1.5,
		DataB64:   "aGVsbG8=",
		Sequence:  0,
	}
	raw, encErr := proto.EncodeEvent(evt)
	env := buildEnvelope(t, raw, encErr)

	sendErr := make(chan error, 1)
	go func() {
		sendErr <- s.Send(env)
	}()

	got, err := c.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if se := <-sendErr; se != nil {
		t.Fatalf("Send: %v", se)
	}

	if got.Type != proto.TypeEvent {
		t.Errorf("Type = %q, want %q", got.Type, proto.TypeEvent)
	}
	if got.Name != proto.EvtNameSurfaceOutput {
		t.Errorf("Name = %q, want %q", got.Name, proto.EvtNameSurfaceOutput)
	}
}

// TestCloseStopsRecv verifies that closing one side causes Recv on the peer
// to return an error because the pipe connection is broken.
func TestCloseStopsRecv(t *testing.T) {
	t.Parallel()

	c, s := protofake.NewPair()
	defer s.Close() //nolint:errcheck

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err := s.Recv()
	if err == nil {
		t.Fatal("expected error from Recv after peer close, got nil")
	}
}
