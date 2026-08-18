package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
)

// Events streams server events (GET /event, SSE) until ctx is done. On a
// dropped connection it backs off (c.backoff(attempt)) and reconnects; the
// returned channel is closed by the reader when ctx is done and nothing is
// emitted after that.
func (c *Client) Events(ctx context.Context) chan protocol.Event {
	ch := make(chan protocol.Event, 4)
	go func() {
		defer close(ch)
		n := 0
		for {
			if err := c.stream(ctx, ch); err == nil {
				return // ctx done
			}
			n++
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.backoff(n)):
			}
		}
	}()
	return ch
}

// stream reads one /event connection to exhaustion. It returns nil when ctx
// is done (the caller stops; the channel closes) and an error otherwise (the
// caller backs off and reconnects).
func (c *Client) stream(ctx context.Context, ch chan<- protocol.Event) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/event", nil)
	if err != nil {
		return err
	}
	c.dirHeader(req)
	resp, err := c.HC.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event stream: status %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev protocol.Event
		if err := json.Unmarshal([]byte(line[len("data: "):]), &ev); err != nil {
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
	return errors.New("event stream closed")
}
