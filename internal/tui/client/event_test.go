package client_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
)

func TestEventsDecodeAndReconnect(t *testing.T) {
	t.Parallel()
	// Frame payloads hoisted so the handler lines stay short.
	const (
		idleFrame = `data: {"id":"evt_1","type":"session.status","properties":` +
			`{"sessionID":"ses_1","status":{"type":"idle"}}}` + "\n\n"
		busyFrame = `data: {"id":"evt_2","type":"session.status","properties":` +
			`{"sessionID":"ses_1","status":{"type":"busy"}}}` + "\n\n"
	)
	var conns int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&conns, 1)
		fl, _ := w.(http.Flusher)
		if n == 1 {
			// first connection: one frame, then the handler returns and the
			// server closes the connection (the client must reconnect)
			_, _ = fmt.Fprint(w, idleFrame)
			fl.Flush()
			return
		}
		_, _ = fmt.Fprint(w, busyFrame)
		fl.Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := client.New(srv.URL, "")
	c.Backoff = func(int) time.Duration { return 10 * time.Millisecond }
	ch, _ := c.Events(ctx)

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

// TestEventsResyncPingsOnDrop pins the resync contract: when the /event
// connection drops, the resync channel receives a ping — the caller must
// re-hydrate state over REST because events published during the drop are
// lost (the bus has no replay). Events keep flowing after the reconnect.
func TestEventsResyncPingsOnDrop(t *testing.T) {
	t.Parallel()
	const (
		idleFrame = `data: {"id":"evt_1","type":"session.status","properties":` +
			`{"sessionID":"ses_1","status":{"type":"idle"}}}` + "\n\n"
		busyFrame = `data: {"id":"evt_2","type":"session.status","properties":` +
			`{"sessionID":"ses_1","status":{"type":"busy"}}}` + "\n\n"
	)
	var conns int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&conns, 1)
		fl, _ := w.(http.Flusher)
		if n == 1 {
			// First connection: one frame, then the handler returns and the
			// server closes the connection (the client must reconnect).
			_, _ = fmt.Fprint(w, idleFrame)
			fl.Flush()
			return
		}
		_, _ = fmt.Fprint(w, busyFrame)
		fl.Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := client.New(srv.URL, "")
	c.Backoff = func(int) time.Duration { return 10 * time.Millisecond }
	ch, resync := c.Events(ctx)

	// Wait for an event with the given id, skipping earlier frames.
	waitFor := func(id string) protocol.Event {
		deadline := time.After(3 * time.Second)
		for {
			select {
			case ev := <-ch:
				if ev.ID == id {
					return ev
				}
			case <-deadline:
				t.Fatalf("timed out waiting for event %s (conns=%d)", id, atomic.LoadInt32(&conns))
				return protocol.Event{}
			}
		}
	}
	waitFor("evt_1") // first connection's frame

	select {
	case <-resync:
	case <-time.After(3 * time.Second):
		t.Fatalf("no resync ping after connection drop (conns=%d)", atomic.LoadInt32(&conns))
	}

	// The reconnect must deliver the second connection's frame.
	if ev := waitFor("evt_2"); ev.Type != "session.status" {
		t.Fatalf("reconnected event = %+v", ev)
	}
}

// TestEventsLargeDataLineSurvives: a single data: line above the former
// 1 MiB scanner cap — escaped tool output (~700 KB+ raw is ≥2× when
// JSON-escaped) — is delivered instead of dropped (safety-2). The 2 MiB
// payload sits under the new 4 MiB cap.
func TestEventsLargeDataLineSurvives(t *testing.T) {
	t.Parallel()
	frame := `data: {"id":"evt_big","type":"message.part.updated","properties":{"part":{"text":"` +
		strings.Repeat("x", 2*1024*1024) + `"}}}` + "\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, frame)
		fl, _ := w.(http.Flusher)
		fl.Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := client.New(srv.URL, "")
	ch, _ := c.Events(ctx)
	select {
	case ev := <-ch:
		if ev.ID != "evt_big" {
			t.Fatalf("event = %+v, want evt_big (the >1 MiB line was dropped)", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out: the >1 MiB data line was not delivered")
	}
}
