// Package protofake provides a minimal in-process IPC pair for unit-testing
// proto-layer consumers (state reducers, runtime relay, etc.).  It stretches a
// single net.Pipe() connection and wraps each end in Send/Recv helpers that
// speak the same ndjson wire format as the real daemon.
//
// Public API is intentionally limited to NewPair, ClientSide, ServerSide, and
// Close on each side (ADR 0013).  Importing state, runtime, or tui is
// prohibited; only encoding/json, fmt, net, and client/proto are used.
package protofake

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/takezoh/agent-reactor/client/proto"
)

// ClientSide is the caller-facing end of the fake IPC connection.
type ClientSide struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

// ServerSide is the daemon-facing end of the fake IPC connection.
type ServerSide struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

// NewPair creates a synchronous in-memory connection via net.Pipe and returns
// both ends ready for use.  The caller owns both sides and must Close each.
func NewPair() (*ClientSide, *ServerSide) {
	c, s := net.Pipe()
	client := &ClientSide{
		conn: c,
		enc:  json.NewEncoder(c),
		dec:  json.NewDecoder(c),
	}
	server := &ServerSide{
		conn: s,
		enc:  json.NewEncoder(s),
		dec:  json.NewDecoder(s),
	}
	return client, server
}

// Send encodes env as a single ndjson frame on the client-to-server direction.
func (c *ClientSide) Send(env proto.Envelope) error {
	if err := c.enc.Encode(env); err != nil {
		return fmt.Errorf("protofake: client send: %w", err)
	}
	return nil
}

// Recv blocks until a complete ndjson frame arrives from the server side.
func (c *ClientSide) Recv() (proto.Envelope, error) {
	var env proto.Envelope
	if err := c.dec.Decode(&env); err != nil {
		return proto.Envelope{}, fmt.Errorf("protofake: client recv: %w", err)
	}
	return env, nil
}

// Close closes the client-side connection, causing any pending Recv on the
// other end to return with an error.
func (c *ClientSide) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("protofake: client close: %w", err)
	}
	return nil
}

// Send encodes env as a single ndjson frame on the server-to-client direction.
func (s *ServerSide) Send(env proto.Envelope) error {
	if err := s.enc.Encode(env); err != nil {
		return fmt.Errorf("protofake: server send: %w", err)
	}
	return nil
}

// Recv blocks until a complete ndjson frame arrives from the client side.
func (s *ServerSide) Recv() (proto.Envelope, error) {
	var env proto.Envelope
	if err := s.dec.Decode(&env); err != nil {
		return proto.Envelope{}, fmt.Errorf("protofake: server recv: %w", err)
	}
	return env, nil
}

// Close closes the server-side connection, causing any pending Recv on the
// other end to return with an error.
func (s *ServerSide) Close() error {
	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("protofake: server close: %w", err)
	}
	return nil
}
