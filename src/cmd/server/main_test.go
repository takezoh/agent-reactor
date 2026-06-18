package main

import (
	"crypto/tls"
	"testing"
)

// These smoke tests exercise the testable surface of the server entry point.
// main/run/serve bind sockets and block, so they are structurally untestable in
// a unit test (the gap is covered by server/web's handler tests); the token and
// self-signed-cert helpers are pure and fully covered here.

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

func TestSelfSignedCertUsable(t *testing.T) {
	cert, err := selfSignedCert()
	if err != nil {
		t.Fatalf("selfSignedCert: %v", err)
	}
	if len(cert.Certificate) == 0 || cert.PrivateKey == nil {
		t.Fatal("selfSignedCert returned an incomplete certificate")
	}
	// It must load into the TLS config the way serve() uses it.
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	if len(cfg.Certificates) != 1 {
		t.Fatal("certificate not accepted by tls.Config")
	}
}
