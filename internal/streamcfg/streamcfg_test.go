package streamcfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuild_TunnelModeOmitsTLS(t *testing.T) {
	c := Build("tesla_telemetry", "tcp://0.0.0.0:5284", "", "", 4443)
	if c.TLS != nil {
		t.Errorf("expected no TLS block in tunnel mode, got %+v", c.TLS)
	}
	if c.Port != 4443 || c.Namespace != "tesla_telemetry" {
		t.Errorf("unexpected config: port=%d ns=%s", c.Port, c.Namespace)
	}
	if !c.TransmitDecodedRecords {
		t.Error("transmit_decoded_records must be true (ingest parses protojson)")
	}
	if got := c.Records["V"]; len(got) != 1 || got[0] != "zmq" {
		t.Errorf("V records = %v, want [zmq]", got)
	}
	if c.ZMQ.Addr != "tcp://0.0.0.0:5284" {
		t.Errorf("zmq addr = %q", c.ZMQ.Addr)
	}
}

func TestBuild_CertModeIncludesTLS(t *testing.T) {
	c := Build("ns", "tcp://0.0.0.0:5284", "/c/cert.pem", "/c/key.pem", 443)
	if c.TLS == nil || c.TLS.ServerCert != "/c/cert.pem" || c.TLS.ServerKey != "/c/key.pem" {
		t.Errorf("expected TLS block with cert/key, got %+v", c.TLS)
	}
}

func TestWrite_AtomicAndValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	c := Build("ns", "tcp://0.0.0.0:5284", "", "", 4443)
	if err := Write(path, c); err != nil {
		t.Fatalf("Write: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var round ServerConfig
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("written config is not valid JSON: %v", err)
	}
	if round.Port != 4443 {
		t.Errorf("round-trip port = %d, want 4443", round.Port)
	}
}
