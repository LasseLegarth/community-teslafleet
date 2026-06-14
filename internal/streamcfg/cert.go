package streamcfg

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnsureSelfSigned writes a self-signed TLS cert+key to certPath/keyPath if they do
// not already exist, returning whether it generated a new pair.
//
// fleet-telemetry is mTLS-only and panics without a TLS config, so it always needs a
// server cert. In Cloudflare-Tunnel mode the tunnel terminates the car's public TLS
// and forwards to fleet-telemetry over the loopback, so a self-signed cert on the
// internal listener is sufficient. Port-forward setups instead point TLSCert/TLSKey
// at a publicly-trusted cert (e.g. Let's Encrypt) the car validates directly.
func EnsureSelfSigned(certPath, keyPath, hostname string) (bool, error) {
	if fileExists(certPath) && fileExists(keyPath) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		return false, fmt.Errorf("create cert dir: %w", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return false, fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return false, fmt.Errorf("serial: %w", err)
	}
	cn := hostname
	if cn == "" {
		cn = "community-teslafleet.local"
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if hostname != "" {
		tmpl.DNSNames = []string{hostname}
	} else {
		tmpl.DNSNames = []string{"localhost", cn}
		tmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return false, fmt.Errorf("create certificate: %w", err)
	}
	if err := writePEM(certPath, "CERTIFICATE", der, 0o644); err != nil {
		return false, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return false, fmt.Errorf("marshal key: %w", err)
	}
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

func writePEM(path, blockType string, der []byte, mode os.FileMode) error {
	b := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, b, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}
