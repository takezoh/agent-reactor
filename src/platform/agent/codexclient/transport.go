// Package codexclient implements the Codex app-server stdio JSON-RPC protocol.
// It provides a transport-agnostic Conn for the framing layer, two concrete
// transports (WebSocket-over-UDS and stdio), and thin client/server role helpers.
//
// All types in this package are safe for concurrent use.
package codexclient

import "context"

// Transport abstracts the byte-level message channel.  Each call to
// ReadMessage/WriteMessage corresponds to exactly one framed JSON-RPC message.
type Transport interface {
	// ReadMessage blocks until a complete message is available or ctx is done.
	ReadMessage(ctx context.Context) ([]byte, error)
	// WriteMessage sends one complete message atomically.
	WriteMessage(ctx context.Context, data []byte) error
	// Close tears down the underlying connection.
	Close() error
}
