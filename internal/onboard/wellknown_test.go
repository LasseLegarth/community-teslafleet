package onboard

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWellKnownHandler(t *testing.T) {
	dir := t.TempDir()
	srv, err := NewServer(Options{DataDir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	h := srv.WellKnownHandler()

	// Before key generation: 404.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, WellKnownPath, nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("before keys: got %d, want 404", rr.Code)
	}

	// After generation: 200 + PEM body.
	srv.store.mu.Lock()
	err = srv.store.generateKeys()
	srv.store.mu.Unlock()
	if err != nil {
		t.Fatalf("generateKeys: %v", err)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, WellKnownPath, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("after keys: got %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/x-pem-file" {
		t.Errorf("content-type = %q", ct)
	}
	if body := rr.Body.String(); len(body) == 0 || body[:27] != "-----BEGIN PUBLIC KEY-----\n" {
		t.Errorf("body not a public-key PEM: %.40q", body)
	}
}
