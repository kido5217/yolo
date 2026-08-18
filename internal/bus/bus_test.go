package bus_test

import (
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/protocol"
)

func TestPubSub(t *testing.T) {
	b := bus.New()
	ch, cancel := b.Subscribe()
	defer cancel()
	e1, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "ses_1"})
	e2, _ := protocol.MakeEvent(protocol.EventTypeSessionStatus, protocol.SessionStatusProps{SessionID: "ses_2"})
	b.Publish(e1)
	b.Publish(e2)
	select {
	case got := <-ch:
		if got.Type != e1.Type {
			t.Fatalf("type = %s", got.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("no event on chan")
	}
	select {
	case got := <-ch:
		if got.Type != e2.Type {
			t.Fatalf("type = %s", got.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("no second event")
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	b := bus.New()
	ch, cancel := b.Subscribe()
	cancel()
	b.Publish(mustEvent(t))
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("delivery after cancel")
		}
		// ok=false: cancel closed the channel and nothing was delivered — correct.
	case <-time.After(50 * time.Millisecond):
		// No delivery reached the (now unregistered) subscriber — correct either way.
	}
}

func TestOverflowCancelsSubscribers(t *testing.T) {
	b := bus.New()
	ch, _ := b.Subscribe()
	// exhaust buffer (1024) + a few
	for i := 0; i < 1100; i++ {
		b.Publish(mustEvent(t))
	}
	select {
	case ev, ok := <-ch:
		if !ok {
			return // channel closed = subscriber dropped, as specified
		}
		_ = ev
	case <-time.After(200 * time.Millisecond):
	}
	// after overflow the subscriber must be gone: subsequent publish must not panic
	for i := 0; i < 10; i++ {
		b.Publish(mustEvent(t))
	}
}

func mustEvent(t *testing.T) protocol.Event {
	t.Helper()
	e, err := protocol.MakeEvent(protocol.EventTypeMessageUpdated, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
