//go:build race

package tui

// TestMarkdownTextPartSGR is skipped under the race detector: its
// terminal-state assertion reads ONE drained frame, but the detector's
// timing changes the cell-diff renderer's frame coalescing — the
// upstream 3-column indent lands as cursor positioning (ESC[<line>;4H)
// instead of literal spaces, so the literal-indent pin (and the
// merged multi-token condition, deviations 141/142) is unsatisfiable
// no matter how long the wait. The assertion is fully pinned in the
// non-race build (the CI gate); the -race gate's job is data races,
// not re-verifying the render (deviation 294).
const markdownSGRTestEnabled = false
