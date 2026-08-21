package server

import (
	"encoding/json"
	"io"
	"net/http"
)

// handleEvent streams bus events as SSE frames: `data: {json}\n\n` (M5).
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		envelope(w, http.StatusInternalServerError, "streaming unsupported", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl.Flush()

	events, done := s.Bus.Subscribe()
	defer done()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok { // bus dropped the subscription (subscriber overflow)
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			// Three writes instead of Fprintf: identical wire bytes,
			// no fmt buffer growth for large frames. A failed write
			// means the stream is dead (the loop exits on ctx done).
			if _, err := io.WriteString(w, "data: "); err != nil {
				continue
			}
			if _, err := w.Write(b); err != nil {
				continue
			}
			if _, err := io.WriteString(w, "\n\n"); err != nil {
				continue
			}
			fl.Flush()
		}
	}
}
