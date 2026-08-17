package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sync"
)

type Opt func(*Server)

type Server struct {
	WorkDir string
	mux     *http.ServeMux
	srv     *http.Server
	addr    net.Addr
	mu      sync.Mutex
}

func New(workDir string, opts ...Opt) *Server {
	s := &Server{WorkDir: workDir, mux: http.NewServeMux()}
	for _, o := range opts {
		o(s)
	}
	s.mux.HandleFunc("GET /global/health", s.handleHealth)
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

// Scope returns the x-yolo-directory value (URL-decoded) or the server work dir.
func (s *Server) Scope(r *http.Request) (string, error) {
	v := r.Header.Get("x-yolo-directory")
	if v == "" {
		return s.WorkDir, nil
	}
	d, err := url.PathUnescape(v)
	if err != nil {
		return "", err
	}
	if d == "" {
		return s.WorkDir, nil
	}
	return d, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) Start(addr string) (net.Addr, error) {
	s.srv = &http.Server{Addr: addr, Handler: s.Handler()}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s.addr = ln.Addr()
	go func() {
		_ = s.srv.Serve(ln)
	}()
	return s.addr, nil
}

func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *Server) Close() {
	if s.srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2_000_000_000) // 2s ns
	defer cancel()
	_ = s.srv.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
