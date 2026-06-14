package onboard

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewServer(Options{DataDir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return s, dir
}

func TestServer_PageRenders(t *testing.T) {
	s, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "onboarding") {
		t.Errorf("page missing expected content")
	}
}

func TestServer_GenerateThenDownload(t *testing.T) {
	s, dir := newTestServer(t)
	h := s.Handler()

	// Generate keys → 303 redirect (PRG).
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/generate", nil))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /generate = %d, want 303", rr.Code)
	}
	for _, f := range []string{"private-key.pem", "public-key.pem", "state.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s after generate: %v", f, err)
		}
	}
	// Private key must be 0600.
	if fi, _ := os.Stat(filepath.Join(dir, "private-key.pem")); fi != nil && fi.Mode().Perm() != 0o600 {
		t.Errorf("private key perms = %v, want 0600", fi.Mode().Perm())
	}

	// Download public key.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/public-key.pem", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "PUBLIC KEY") {
		t.Errorf("download public key: code=%d body=%.40q", rr.Code, rr.Body.String())
	}
}

func TestServer_BasicAuth(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewServer(Options{DataDir: dir, Password: "secret"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := s.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no-auth GET = %d, want 401", rr.Code)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("x", "secret")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("auth GET = %d, want 200", rr.Code)
	}
}
