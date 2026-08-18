package session

import (
	"testing"
	"time"
)

// TestDefaultBackoffBounds pins the production retry delay: 1s x 2^(n-1)
// (n = failed attempt, 1-based) scaled by a uniform jitter in [0.8, 1.2].
func TestDefaultBackoffBounds(t *testing.T) {
	for attempt := 1; attempt <= 4; attempt++ {
		base := time.Second << uint(attempt-1)
		lo := time.Duration(0.8 * float64(base))
		hi := time.Duration(1.2 * float64(base))
		for i := 0; i < 500; i++ {
			d := defaultBackoff(attempt)
			if d < lo || d > hi {
				t.Fatalf("attempt %d: delay %v outside [%v, %v]", attempt, d, lo, hi)
			}
		}
	}
}
