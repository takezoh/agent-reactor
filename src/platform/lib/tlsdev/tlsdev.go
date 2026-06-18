// Package tlsdev provides the shared dev-friendly TLS serving used by the
// agent-reactor server binaries (cmd/server, cmd/web): serve plain HTTP
// (-insecure), with a supplied certificate, or with an in-memory self-signed
// certificate for localhost when none is given.
package tlsdev

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"time"
)

// Serve runs srv: plain HTTP when insecure; TLS from certFile/keyFile when both
// are set; otherwise TLS with an in-memory self-signed localhost certificate.
func Serve(srv *http.Server, insecure bool, certFile, keyFile string) error {
	switch {
	case insecure:
		return srv.ListenAndServe()
	case certFile != "" && keyFile != "":
		return srv.ListenAndServeTLS(certFile, keyFile)
	default:
		cert, err := SelfSignedCert()
		if err != nil {
			return err
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		return srv.ListenAndServeTLS("", "")
	}
}

// SelfSignedCert generates an in-memory self-signed certificate for localhost,
// used when no -tls-cert/-tls-key is supplied. Dev convenience: clients must
// skip verification or trust it out of band.
func SelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "agent-reactor-dev"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}
