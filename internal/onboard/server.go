package onboard

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed templates/*.html
var templatesFS embed.FS

// State is the persisted onboarding progress (no secrets — the private key lives in a
// separate file with 0600 perms).
type State struct {
	Domain    string       `json:"domain"`
	ClientID  string       `json:"client_id"`
	HasKeys   bool         `json:"has_keys"`
	LastProbe *ProbeResult `json:"last_probe,omitempty"`
}

// Store persists keys + state under DataDir.
type Store struct {
	dir string
	mu  sync.Mutex
	st  State
}

func newStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	if b, err := os.ReadFile(s.statePath()); err == nil {
		_ = json.Unmarshal(b, &s.st)
	}
	return s, nil
}

func (s *Store) statePath() string   { return filepath.Join(s.dir, "state.json") }
func (s *Store) privatePath() string { return filepath.Join(s.dir, "private-key.pem") }
func (s *Store) publicPath() string  { return filepath.Join(s.dir, "public-key.pem") }

func (s *Store) saveState() error {
	b, _ := json.MarshalIndent(s.st, "", "  ")
	tmp := s.statePath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath())
}

func (s *Store) generateKeys() error {
	kp, err := GenerateKey()
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.privatePath(), kp.PrivatePEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(s.publicPath(), kp.PublicPEM, 0o644); err != nil {
		return err
	}
	s.st.HasKeys = true
	return s.saveState()
}

func (s *Store) publicPEM() ([]byte, error) { return os.ReadFile(s.publicPath()) }

// Server is the onboarding wizard's HTTP handler set.
type Server struct {
	store *Store
	pass  string
	log   *slog.Logger
	tmpl  *template.Template
}

// NewServer builds the wizard server, loading/initialising state under dataDir.
func NewServer(dataDir, password string, log *slog.Logger) (*Server, error) {
	store, err := newStore(dataDir)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{store: store, pass: password, log: log, tmpl: tmpl}, nil
}

// Handler returns the wizard's HTTP handler (mount on its own listener or HA ingress).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("POST /domain", s.saveDomain)
	mux.HandleFunc("POST /generate", s.generate)
	mux.HandleFunc("GET /public-key.pem", s.downloadPublic)
	mux.HandleFunc("POST /probe", s.probe)
	return s.auth(mux)
}

// auth applies optional basic-auth (standalone). HA ingress is already authenticated.
func (s *Server) auth(next http.Handler) http.Handler {
	if s.pass == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pw, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pw), []byte(s.pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="community-teslafleet"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// view is the template data; base is the ingress base path (X-Ingress-Path) so all
// links/forms work under HA ingress.
type view struct {
	Base  string
	State State
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string) {
	s.store.mu.Lock()
	st := s.store.st
	s.store.mu.Unlock()
	v := view{Base: r.Header.Get("X-Ingress-Path"), State: st}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, v); err != nil {
		s.log.Error("render", "tmpl", name, "err", err)
	}
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) { s.render(w, r, "index.html") }

// home redirects back to the wizard root (PRG: avoids re-POST on refresh). Respects the
// HA ingress base path.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, r.Header.Get("X-Ingress-Path")+"/", http.StatusSeeOther)
}

func (s *Server) saveDomain(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	s.store.st.Domain = strings.TrimSpace(r.FormValue("domain"))
	s.store.st.ClientID = strings.TrimSpace(r.FormValue("client_id"))
	_ = s.store.saveState()
	s.store.mu.Unlock()
	s.home(w, r)
}

func (s *Server) generate(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	err := s.store.generateKeys()
	s.store.mu.Unlock()
	if err != nil {
		http.Error(w, "key generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.home(w, r)
}

func (s *Server) downloadPublic(w http.ResponseWriter, r *http.Request) {
	pub, err := s.store.publicPEM()
	if err != nil {
		http.Error(w, "no key yet — generate first", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="com.tesla.3p.public-key.pem"`)
	w.Write(pub)
}

func (s *Server) probe(w http.ResponseWriter, r *http.Request) {
	s.store.mu.Lock()
	domain := s.store.st.Domain
	pub, _ := s.store.publicPEM()
	s.store.mu.Unlock()
	if domain == "" || pub == nil {
		http.Error(w, "set a domain and generate keys first", http.StatusBadRequest)
		return
	}
	res := ProbeWellKnown(domain, pub)
	s.store.mu.Lock()
	s.store.st.LastProbe = &res
	_ = s.store.saveState()
	s.store.mu.Unlock()
	s.home(w, r)
}
