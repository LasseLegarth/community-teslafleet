package streamcfg

import (
	"crypto/tls"
	"path/filepath"
	"testing"
)

func TestEnsureSelfSigned_GeneratesUsableCert(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "certs", "cert.pem")
	key := filepath.Join(dir, "certs", "key.pem")

	gen, err := EnsureSelfSigned(cert, key, "")
	if err != nil {
		t.Fatalf("EnsureSelfSigned: %v", err)
	}
	if !gen {
		t.Fatal("expected a new cert to be generated")
	}
	// The pair must load as a valid TLS keypair (what fleet-telemetry needs).
	if _, err := tls.LoadX509KeyPair(cert, key); err != nil {
		t.Fatalf("generated pair is not a valid TLS keypair: %v", err)
	}

	// Idempotent: a second call must not regenerate.
	gen, err = EnsureSelfSigned(cert, key, "")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if gen {
		t.Error("expected no regeneration when cert already exists")
	}
}
