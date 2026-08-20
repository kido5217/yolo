package server

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/kido5217/yolo/internal/log"
	"github.com/kido5217/yolo/internal/protocol"
)

// recoverMiddleware turns handler panics into a 500 envelope (M5). Panics
// are diagnosed to lob with the stack trace (nil = dropped); the value alone
// usually cannot locate the fault.
func recoverMiddleware(lob *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				lob.Errorf("handler panic (path=%s): %v\n%s", r.URL.Path, rec, debug.Stack())
				envelope(w, http.StatusInternalServerError, "internal error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// envelope writes the wire error shape: {"error":{"message":...,"data"?}}.
func envelope(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": protocol.Error{Message: msg, Data: data},
	})
}

// fail is the 5xx path: the underlying error is logged (the client only gets
// the generic msg), then the envelope is written. Wire shape unchanged.
func (s *Server) fail(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		s.Log.Errorf("server: %s: %v", msg, err)
	}
	envelope(w, code, msg, nil)
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	envelope(w, http.StatusNotFound, "unknown route "+r.URL.Path, nil)
}
