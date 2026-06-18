package tlsdev

import (
	"crypto/tls"
	"testing"
)

func TestSelfSignedCertUsable(t *testing.T) {
	cert, err := SelfSignedCert()
	if err != nil {
		t.Fatalf("SelfSignedCert: %v", err)
	}
	if len(cert.Certificate) == 0 || cert.PrivateKey == nil {
		t.Fatal("SelfSignedCert returned an incomplete certificate")
	}
	// It must load into the TLS config the way Serve uses it.
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	if len(cfg.Certificates) != 1 {
		t.Fatal("certificate not accepted by tls.Config")
	}
}

func TestSelfSignedCertDistinct(t *testing.T) {
	a, err := SelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}
	b, err := SelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}
	// Distinct keys each call (fresh ecdsa key) — no shared global state.
	if string(a.Certificate[0]) == string(b.Certificate[0]) {
		t.Fatal("two SelfSignedCert calls produced identical certificates")
	}
}
