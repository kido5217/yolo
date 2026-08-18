package client_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
)

func TestEventsDecodeAndReconnect(t *testing.T) {
	var conns int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&conns, 1)
		fl, _ := w.(http.Flusher)
		if n == 1 {
			// first connection: one frame, then the handler returns and the
			// server closes the connection (the client must reconnect)
			_, _ = fmt.Fprint(w, `data: {"id":"evt_1","type":"session.status","properties":{"sessionID":"ses_1","status":{"type":"idle"}}}`+"\n\n")
			fl.Flush()
			return
		}
		_, _ = fmt.Fprint(w, `data: {"id":"evt_2","type":"session.status","properties":{"sessionID":"ses_1","status":{"type":"busy"}}}`+"\n\n")
		fl.Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := client.New(srv.URL, "")
	c.Backoff = func(int) time.Duration { return 10 * time.Millisecond }
	ch := c.Events(ctx)

	var evs []protocol.Event
	deadline := time.After(3 * time.Second)
	for len(evs) < 2 {
		select {
		case ev := <-ch:
			evs = append(evs, ev)
		case <-deadline:
			t.Fatalf("timed out after %d events, conns=%d", len(evs), atomic.LoadInt32(&conns))
		}
	}
	cancel()
	if evs[0].ID != "evt_1" || evs[1].ID != "evt_2" || evs[1].Type != "session.status" {
		t.Fatalf("events = %+v", evs)
	}
	if n := atomic.LoadInt32(&conns); n < 2 {
		t.Fatalf("reconnections = %d, want >= 2", n)
	}
}
