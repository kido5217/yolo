//go:build !race

package tui

import "time"

// Wall-clock budget for TestRenderMessages100KBBudget without the race
// detector: 150 ms = 1.5x headroom over the measured ~100 ms non-race
// floor (deviation 163).
const render100KBBudget = 150 * time.Millisecond
