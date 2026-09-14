package client

import (
	"testing"
	"time"
)

func TestBackoffNeverZeroOrUncapped(t *testing.T) {
	t.Parallel()
	c := &Service{}
	for _, n := range []int{0, 1, 4, 5, 6, 63, 64, 65, 1024} {
		d := c.backoff(n)
		if d <= 0 {
			t.Fatalf("backoff(%d) = %v, want > 0 (shift overflow)", n, d)
		}
		if d > 30*time.Second {
			t.Fatalf("backoff(%d) = %v, want <= 30s", n, d)
		}
	}
}
