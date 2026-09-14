package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
)

// sseDataPrefix is the SSE data-line prefix; events carry their JSON after it.
var sseDataPrefix = []byte("data: ")

// Events streams server events (GET /event, SSE) until ctx is done. On a
// dropped connection it backs off (c.backoff(attempt)) and reconnects; both
// channels are closed by the reader when ctx is done and nothing is emitted
// after that. The resync channel receives a ping on every drop: events
// published while the stream was down are lost (the bus has no replay), so
// the caller must re-hydrate its state over REST on each ping.
func (c *Service) Events(ctx context.Context) (chan protocol.Event, chan struct{}) {
	ch := make(chan protocol.Event, 4)
	resync := make(chan struct{}, 4)
	go func() {
		defer close(ch)
		defer close(resync)
		n := 0
		for {
			if err := c.stream(ctx, ch); err == nil {
				return // ctx done
			}
			n++
			// The drop just happened: signal the caller to resync. A full
			// buffer (slow consumer) drops the ping — a later ping or the
			// next event's re-render converges the state.
			select {
			case resync <- struct{}{}:
			default:
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.backoff(n)):
			}
		}
	}()
	return ch, resync
}

// stream reads one /event connection to exhaustion. It returns nil when ctx
// is done (the caller stops; the channel closes) and an error otherwise (the
// caller backs off and reconnects).
func (c *Service) stream(ctx context.Context, ch chan<- protocol.Event) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/event", nil)
	if err != nil {
		return err
	}
	c.dirHeader(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event stream: status %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	// 4 MiB max token: a single data: line carries one whole event JSON;
	// escaped tool output (~700 KB+ raw, ≥2× when escaped) exceeded the
	// former 1 MiB cap (safety-2). Overflow still aborts the stream and
	// fires the resync ping — bounded by the re-hydrate.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		// sc.Bytes() is valid until the next Scan; unmarshalling copies the
		// payload out, so no per-line []byte conversion is needed.
		line := sc.Bytes()
		if !bytes.HasPrefix(line, sseDataPrefix) {
			continue
		}
		var ev protocol.Event
		if err := json.Unmarshal(line[len(sseDataPrefix):], &ev); err != nil {
			continue
		}
		select {
		case ch <- ev:
		case <-ctx.Done():
			return nil
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if sc.Err() != nil {
		return sc.Err()
	}
	return errors.New("client: event stream closed")
}
