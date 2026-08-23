package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

// TestAppResyncRehydrates pins the app side of SSE drop recovery:
// (1) the resync pump is armed and delivers resyncMsg on a ping from the
// client (which pings on every dropped /event connection);
// (2) resyncMsg re-triggers the REST hydrate of the current route, so a
// transcript persisted during the drop (events the bus cannot replay) is
// recovered into the store.
func TestAppResyncRehydrates(t *testing.T) {
	ts := testutil.Boot(t)
	ctx := context.Background()
	c := client.New(ts.URL, ts.Dir)
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	// Seed a completed turn in storage: the re-hydrate has something to
	// recover (user + assistant messages).
	if _, err := c.SendMessage(ctx, ses.ID, "go"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	ts.WaitIdle(t, ts.Dir, ses.ID)

	// Fresh app on the same session with an empty display store: simulate
	// the state where the SSE transcript state was lost in a drop.
	ra := newRecApp(c, store.State{}, ses.ID)
	t.Cleanup(ra.Close)

	// (1) The resync pump is wired and delivers resyncMsg on a ping.
	pump := ra.resyncPump()
	if pump == nil {
		t.Fatal("resyncPump must return an armed cmd (resync channel unwired)")
	}
	delivered := make(chan tea.Msg, 1)
	go func() { delivered <- pump() }()
	ra.resyncCh <- struct{}{}
	m := <-delivered
	if _, ok := m.(resyncMsg); !ok {
		t.Fatalf("resync pump delivered %v (%T), want resyncMsg", m, m)
	}
	// (2) resyncMsg re-triggers the re-hydrate.
	if _, cmd := ra.Update(m); cmd == nil {
		t.Fatal("resyncMsg must return a re-hydrate/re-arm cmd")
	}
	// Run the re-hydrate the same way the program loop would and apply its
	// payload; the store must hold both persisted messages.
	hm := ra.hydrateCmd()()
	if _, ok := hm.(hydratedMsg); !ok {
		t.Fatalf("hydrateCmd delivered %T, want hydratedMsg", hm)
	}
	ra.Update(hm)
	if got := len(ra.store.Messages); got != 2 {
		t.Fatalf("store after resync re-hydrate has %d messages, want 2", got)
	}
}

// TestAppResyncFooter pins the outage-window footer (concurrency-4): a
// transient SSE drop flips the footer to a non-live "reconnecting" state,
// restored to "● live" when the re-hydrate completes; the terminal
// connLostMsg still renders "○ off".
func TestAppResyncFooter(t *testing.T) {
	ts := testutil.Boot(t)
	c := client.New(ts.URL, ts.Dir)
	ra := newRecApp(c, store.State{}, "")
	t.Cleanup(ra.Close)
	ra.store.Live = true
	if got := ra.footerView(); !strings.Contains(got, "● live") {
		t.Fatalf("live baseline footer = %q, want \"● live\"", got)
	}
	// Transient drop: the footer leaves the live state for the outage window.
	ra.Update(resyncMsg{})
	if got := ra.footerView(); strings.Contains(got, "● live") || !strings.Contains(got, "reconnecting") {
		t.Fatalf("outage footer = %q, want a non-live \"reconnecting\" state", got)
	}
	// The re-hydrate completes: back to live.
	ra.Update(hydratedMsg{})
	if got := ra.footerView(); !strings.Contains(got, "● live") {
		t.Fatalf("post-rehydrate footer = %q, want \"● live\" restored", got)
	}
	// Terminal loss stays off (unchanged behavior).
	ra.Update(resyncMsg{})
	ra.Update(hydratedMsg{})
	ra.Update(connLostMsg{})
	if got := ra.footerView(); !strings.Contains(got, "○ off") {
		t.Fatalf("connLost footer = %q, want \"○ off\"", got)
	}
}
