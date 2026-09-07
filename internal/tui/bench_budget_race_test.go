//go:build race

package tui

import "time"

// Wall-clock budget for TestRenderMessages100KBBudget under the race
// detector, which slows the string/allocation-heavy glamour
// markdown-to-ANSI path roughly 11x over the non-race build (measured
// min-of-5 1.120-1.143 s on this host, 2026-09-07, vs the ~100 ms
// non-race floor of deviation 163). The render work is unchanged, so a
// real regression (a slow render path / lost wrap that doubles the
// non-race floor, ~200 ms -> ~2.2 s under the detector) still fails
// loudly against the relaxed bound (deviation 294).
const render100KBBudget = 2 * time.Second
