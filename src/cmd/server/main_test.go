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

// TestIsLoopbackAddr pins the -no-auth guardrail: only loopback binds are
// accepted. A regression that allowed -no-auth on 0.0.0.0 / a public IP /
// the empty wildcard would expose the unauthenticated REST surface to the
// network, which is precisely what this check exists to prevent.
func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8443", true},
		{"127.0.0.5:8443", true},
		{"[::1]:8443", true},
		{"localhost:8443", true},
		{"127.0.0.1", true},
		{"::1", true},
		{":8443", false},        // wildcard — binds all interfaces
		{"0.0.0.0:8443", false}, // explicit all-interfaces
		{"[::]:8443", false},    // IPv6 wildcard
		{"192.168.1.5:8443", false},
		{"10.0.0.1:8443", false},
		{"example.com:8443", false}, // unresolved hostname — refuse
		{"", false},
	}
	for _, c := range cases {
		if got := isLoopbackAddr(c.addr); got != c.want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", c.addr, got, c.want)
		}
	}
}
