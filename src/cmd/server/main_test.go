package main

import "testing"

// main/run bind sockets and block, so they are structurally untestable in a unit
// test (the handler is covered by server/web; TLS serving by platform/lib/tlsdev).
// randToken is pure and covered here.

func TestRandTokenDistinctNonEmpty(t *testing.T) {
	a, b := randToken(), randToken()
	if a == "" {
		t.Fatal("randToken returned empty")
	}
	if a == b {
		t.Fatal("randToken returned identical tokens — not random")
	}
	if len(a) != 48 { // 24 random bytes, hex-encoded
		t.Fatalf("token length = %d, want 48", len(a))
	}
}
