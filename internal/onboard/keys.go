// Package onboard implements the guided Tesla partner-onboarding wizard: generate the
// signing keypair, render the .well-known public key, probe that it's reachable, and
// (with live credentials) register the partner account, pair vehicles and enroll
// telemetry. This file is the locally-testable core (keys + probe); the Tesla-API
// steps live in tesla.go and require real credentials to verify.
package onboard

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WellKnownPath is where Tesla fetches a partner's public key during registration.
const WellKnownPath = "/.well-known/appspecific/com.tesla.3p.public-key.pem"

// KeyPair is a freshly generated prime256v1 (P-256) keypair: the private key feeds the
// vehicle-command proxy (command signing); the public key is published at WellKnownPath.
type KeyPair struct {
	PrivatePEM []byte // SEC1 "EC PRIVATE KEY"
	PublicPEM  []byte // PKIX "PUBLIC KEY"
}

// GenerateKey creates a P-256 keypair (the curve Tesla requires for fleet signing).
func GenerateKey() (*KeyPair, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	privDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal private: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public: %w", err)
	}
	return &KeyPair{
		PrivatePEM: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER}),
		PublicPEM:  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
	}, nil
}

// ProbeResult is the outcome of checking the hosted .well-known public key.
type ProbeResult struct {
	URL     string
	OK      bool
	Status  int
	Message string
}

// ProbeWellKnown fetches https://<domain><WellKnownPath> and verifies it serves exactly
// the expected public-key PEM — the single most common onboarding mistake. domain may
// include a custom port (e.g. "example.com:8443"); it must be publicly reachable HTTPS.
func ProbeWellKnown(domain string, expectedPEM []byte) ProbeResult {
	url := "https://" + strings.TrimRight(domain, "/") + WellKnownPath
	r := ProbeResult{URL: url}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(url)
	if err != nil {
		r.Message = "not reachable: " + err.Error()
		return r
	}
	defer resp.Body.Close()
	r.Status = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		r.Message = fmt.Sprintf("HTTP %d (expected 200)", resp.StatusCode)
		return r
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if !pemEqual(body, expectedPEM) {
		r.Message = "reachable, but the served key does not match the generated key"
		return r
	}
	r.OK = true
	r.Message = "public key is reachable and matches ✓"
	return r
}

// pemEqual compares two PEM blobs ignoring surrounding/!line-ending whitespace.
func pemEqual(a, b []byte) bool {
	norm := func(p []byte) []byte {
		return bytes.Join(bytes.Fields(p), []byte("\n"))
	}
	return bytes.Equal(norm(a), norm(b))
}
