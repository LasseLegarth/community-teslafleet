package onboard

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateKey_ValidPEM(t *testing.T) {
	kp, err := GenerateKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Private key parses as SEC1 EC.
	pb, _ := pem.Decode(kp.PrivatePEM)
	if pb == nil || pb.Type != "EC PRIVATE KEY" {
		t.Fatalf("private PEM type = %v", pb)
	}
	if _, err := x509.ParseECPrivateKey(pb.Bytes); err != nil {
		t.Errorf("parse EC private: %v", err)
	}
	// Public key parses as PKIX.
	ub, _ := pem.Decode(kp.PublicPEM)
	if ub == nil || ub.Type != "PUBLIC KEY" {
		t.Fatalf("public PEM type = %v", ub)
	}
	if _, err := x509.ParsePKIXPublicKey(ub.Bytes); err != nil {
		t.Errorf("parse PKIX public: %v", err)
	}
	// Two generations differ.
	kp2, _ := GenerateKey()
	if string(kp2.PrivatePEM) == string(kp.PrivatePEM) {
		t.Errorf("two generated keys are identical")
	}
}

func TestProbeWellKnown(t *testing.T) {
	kp, _ := GenerateKey()

	// Server that serves the correct key at the well-known path.
	ok := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == WellKnownPath {
			w.Write(kp.PublicPEM)
			return
		}
		http.NotFound(w, r)
	}))
	defer ok.Close()
	domain := strings.TrimPrefix(ok.URL, "https://")

	// Use the test server's TLS client so the self-signed cert is trusted.
	client := ok.Client()
	got := probeWith(client, domain, kp.PublicPEM)
	if !got.OK {
		t.Errorf("expected OK probe, got %+v", got)
	}

	// Wrong key → mismatch.
	other, _ := GenerateKey()
	if probeWith(client, domain, other.PublicPEM).OK {
		t.Errorf("expected mismatch for wrong key")
	}
}

// probeWith mirrors ProbeWellKnown but accepts a client (so tests can trust the
// httptest TLS cert). Kept test-local to avoid widening the public API.
func probeWith(client *http.Client, domain string, expectedPEM []byte) ProbeResult {
	url := "https://" + domain + WellKnownPath
	r := ProbeResult{URL: url}
	resp, err := client.Get(url)
	if err != nil {
		r.Message = err.Error()
		return r
	}
	defer resp.Body.Close()
	r.Status = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		return r
	}
	body := make([]byte, 64*1024)
	n, _ := resp.Body.Read(body)
	if pemEqual(body[:n], expectedPEM) {
		r.OK = true
	}
	return r
}
