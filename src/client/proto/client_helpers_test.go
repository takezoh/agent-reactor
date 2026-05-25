package proto

import (
	"testing"
	"time"
)

// TestOtherRPCsUseShortTimeout verifies that generic event RPCs still use
// the default 5s timeout (confirmed by context deadline when server is silent).
func TestOtherRPCsUseShortTimeout(t *testing.T) {
	c, server := newFakeServer(t)
	defer c.Close()

	// server reads the request but intentionally never responds.
	go func() { server.recv() }()

	start := time.Now()
	_, err := sendJSONEvent[RespOK](c, "stop-session", map[string]string{"session_id": "x"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// Should expire around defaultRequestTimeout (5s), not 5 minutes.
	if elapsed > 10*time.Second {
		t.Errorf("timeout took %v, want ~5s", elapsed)
	}
}
