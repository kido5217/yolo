package server

import (
	"encoding/json"
	"fmt"
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
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
	}
}
