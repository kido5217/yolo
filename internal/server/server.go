// Package server exposes the core REST + SSE API as a plain http.Handler
// (M5). cmd/yolo wraps it in a listener.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kido5217/yolo/internal/auth"
	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/config"
	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
)

// Deps wires the server to the core (M5: value in, http.Handler out).
type Deps struct {
	DB     *storage.DB
	Bus    *bus.Bus
	Engine *session.Engine
	Prov   *provider.Registry
	Perm   *permission.Service
	Config config.Loader
	// Log receives handler-panic diagnostics; nil = no-op.
	Log *log.Logger
	// WorkDir is the directory scope when x-yolo-directory is absent
	// (process CWD under `yolo serve`).
	WorkDir string
	// Dirs carries home/data/cache locations; zero value = real XDG.
	Dirs config.Dirs
}

// Server holds Deps plus the optional in-process listener.
type Server struct {
	Deps
	handler http.Handler
	srv     *http.Server
	addr    net.Addr
	mu      sync.Mutex

	// Auth store for the server's lifetime (M5): PUT/DELETE /auth mutate it
	// and persist via auth.SaveTo; GET /provider recomputes status from it.
	authMu    sync.Mutex
	authPath  string
	authStore auth.Store
	// authErr is set when initAuth cannot resolve the auth file at all
	// (zero Dirs plus an unresolvable XDG home); auth routes answer with 500.
	authErr error
}

// New returns the core API as a plain http.Handler.
func New(d Deps) http.Handler { return NewServer(d).Handler() }

// NewServer builds the handler on the returned instance (initAuth plus the
// mux) and exposes the listener lifecycle. There is exactly one *Server per
// Deps: handlers and the auth state always live on the same instance.
func NewServer(d Deps) *Server {
	s := &Server{Deps: d}
	s.handler = s.build()
	return s
}

// Handler returns the core API handler.
func (s *Server) Handler() http.Handler { return s.handler }

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) build() http.Handler {
	if err := s.initAuth(); err != nil {
		s.authErr = err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /global/health", s.handleHealth)
	mux.HandleFunc("GET /path", s.handlePath)
	mux.HandleFunc("GET /project/current", s.handleProjectCurrent)
	mux.HandleFunc("GET /session", s.handleSessionList)
	mux.HandleFunc("POST /session", s.handleSessionCreate)
	// literal outranks the {id} wildcard (Go 1.22 specificity)
	mux.HandleFunc("GET /session/status", s.handleSessionStatus)
	mux.HandleFunc("GET /session/{id}", s.handleSessionGet)
	mux.HandleFunc("PATCH /session/{id}", s.handleSessionPatch)
	mux.HandleFunc("DELETE /session/{id}", s.handleSessionDelete)
	mux.HandleFunc("GET /session/{id}/message", s.handleMessages)
	mux.HandleFunc("POST /session/{id}/message", s.handleSend)
	mux.HandleFunc("POST /session/{id}/abort", s.handleAbort)
	mux.HandleFunc("POST /session/{id}/command", s.handleCommand)
	mux.HandleFunc("GET /event", s.handleEvent)
	mux.HandleFunc("GET /provider", s.handleProvider)
	mux.HandleFunc("GET /provider/auth", s.handleProviderAuth)
	mux.HandleFunc("GET /config", s.handleConfigGet)
	mux.HandleFunc("PATCH /config", s.handleConfigPatch)
	mux.HandleFunc("GET /global/config", s.handleGlobalConfigGet)
	mux.HandleFunc("PATCH /global/config", s.handleGlobalConfigPatch)
	mux.HandleFunc("PUT /auth/{providerID}", s.handleAuthPut)
	mux.HandleFunc("DELETE /auth/{providerID}", s.handleAuthDelete)
	mux.HandleFunc("GET /agent", s.handleAgent)
	mux.HandleFunc("GET /command", s.handleCommandList)
	mux.HandleFunc("GET /permission", s.handlePermissionList)
	mux.HandleFunc("POST /permission/{requestID}/reply", s.handlePermissionReply)
	// unknown route -> 404 envelope, per method
	mux.HandleFunc("GET /{tail...}", s.handleNotFound)
	mux.HandleFunc("POST /{tail...}", s.handleNotFound)
	mux.HandleFunc("PATCH /{tail...}", s.handleNotFound)
	mux.HandleFunc("DELETE /{tail...}", s.handleNotFound)
	mux.HandleFunc("PUT /{tail...}", s.handleNotFound)
	return recoverMiddleware(s.Log, mux)
}

// initAuth loads the boot-lifetime auth store from <Dirs.Data>/auth.json
// (missing file = empty store). A corrupt or unreadable file also starts
// empty but is logged — never silently; an unresolvable path at all (zero
// Dirs, broken home) is the returned error.
func (s *Server) initAuth() error {
	p, err := authPath(s.Dirs)
	if err != nil {
		return err
	}
	s.authPath = p
	st, err := auth.LoadFrom(s.authPath)
	switch {
	case err == nil:
		s.authStore = st
	case os.IsNotExist(err):
		s.authStore = auth.Store{}
	default:
		s.authStore = auth.Store{}
		s.Log.Errorf("auth load (%s): %v", s.authPath, err)
	}
	return nil
}

func authPath(d config.Dirs) (string, error) {
	if d.Data == "" {
		return auth.Path()
	}
	return filepath.Join(d.Data, "auth.json"), nil
}

// globalDir is <Dirs.Home>/yolo; zero Home falls back to the real XDG home.
func (s *Server) globalDir() (string, error) {
	if s.Dirs.Home == "" {
		return config.GlobalYoloDir()
	}
	return filepath.Join(s.Dirs.Home, "yolo"), nil
}

// Start listens on addr (":0" = ephemeral) and serves in a goroutine.
func (s *Server) Start(addr string) (net.Addr, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	// ReadHeaderTimeout bounds slowloris-style header sends on
	// user-overridable listen addresses; Read/Write/Idle stay off for the
	// long-lived SSE endpoint.
	srv := &http.Server{Handler: s.handler, ReadHeaderTimeout: 10 * time.Second}
	s.mu.Lock()
	s.srv = srv
	s.addr = ln.Addr()
	bound := s.addr
	s.mu.Unlock()
	go func() {
		_ = srv.Serve(ln)
	}()
	return bound, nil
}

// Addr returns the bound listener address (nil before Start).
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Shutdown gracefully stops the listener within ctx's budget (in-flight
// handlers get to finish); a no-op if Start was never called.
func (s *Server) Shutdown(ctx context.Context) {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return
	}
	_ = srv.Shutdown(ctx)
}

// Close shuts the listener down (2s grace).
func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

// scope resolves the request's project directory: x-yolo-directory
// (URL-encoded absolute path) or the server work dir. Bad escapes or
// non-directories are errors (400).
func (s *Server) scope(r *http.Request) (string, error) {
	v := r.Header.Get("x-yolo-directory")
	if v == "" {
		return s.WorkDir, nil
	}
	d, err := url.PathUnescape(v)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(d)
	if err != nil || !st.IsDir() {
		return "", errors.New("not a directory: " + d)
	}
	return d, nil
}

// decode JSON-decodes the request body into v; an empty body leaves v
// untouched.
func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
