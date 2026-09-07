//go:build !race

package tui

// TestMarkdownTextPartSGR runs without the race detector: its
// terminal-state assertion reads a single drained frame with the
// literal 3-column indent (the non-race frame coalescing pins it).
const markdownSGRTestEnabled = true
