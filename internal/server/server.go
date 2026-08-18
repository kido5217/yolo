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
	"sync"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/config"
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
}

// New returns the core API as a plain http.Handler.
func New(d Deps) http.Handler { return build(d) }

// NewServer builds the handler and exposes the listener lifecycle.
func NewServer(d Deps) *Server { return &Server{Deps: d, handler: build(d)} }

// Handler returns the core API handler.
func (s *Server) Handler() http.Handler { return s.handler }

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func build(d Deps) http.Handler {
	s := &Server{Deps: d}
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
	// unknown route -> 404 envelope, per method
	mux.HandleFunc("GET /{tail...}", s.handleNotFound)
	mux.HandleFunc("POST /{tail...}", s.handleNotFound)
	mux.HandleFunc("PATCH /{tail...}", s.handleNotFound)
	mux.HandleFunc("DELETE /{tail...}", s.handleNotFound)
	return recoverMiddleware(mux)
}

// Start listens on addr (":0" = ephemeral) and serves in a goroutine.
func (s *Server) Start(addr string) (net.Addr, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s.srv = &http.Server{Handler: s.handler}
	s.addr = ln.Addr()
	go func() {
		_ = s.srv.Serve(ln)
	}()
	return s.addr, nil
}

// Addr returns the bound listener address (nil before Start).
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Close shuts the listener down (2s grace).
func (s *Server) Close() {
	if s.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
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
