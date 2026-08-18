// Package bus is the in-process pub/sub event bus.
package bus

import (
	"sync"

	"github.com/kido5217/yolo/internal/protocol"
)

const subscriberBuffer = 1024

type subscriber struct {
	ch     chan protocol.Event
	closed bool
}

// Bus fans events out to subscribers; a subscriber whose buffer overflows is
// cancelled (spec §7: overflow closes the client — the TUI reconnects).
type Bus struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

// New returns an empty bus.
func New() *Bus { return &Bus{subs: map[*subscriber]struct{}{}} }

// SubscriberCount returns the number of live subscribers. Test support: lets
// a test wait until an SSE subscriber is registered before publishing, so
// the first frames are not dropped by the subscribe/handshake window.
func (b *Bus) SubscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

// Subscribe registers a subscriber and returns its receive channel plus a
// cancel func that unregisters and closes the channel.
func (b *Bus) Subscribe() (<-chan protocol.Event, func()) {
	s := &subscriber{ch: make(chan protocol.Event, subscriberBuffer)}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	return s.ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[s]; ok {
			delete(b.subs, s)
			close(s.ch)
			s.closed = true
		}
		b.mu.Unlock()
	}
}

// Publish delivers e to every subscriber non-blockingly; a full subscriber is
// dropped (its channel closed) so Publish never blocks.
func (b *Bus) Publish(e protocol.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		select {
		case s.ch <- e:
		default:
			delete(b.subs, s)
			if !s.closed {
				close(s.ch)
				s.closed = true
			}
		}
	}
}
