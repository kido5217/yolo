package tool

import (
	"strings"
	"testing"
)

// BenchmarkTruncate pins the single-pass tail cut (candidate-10): hermetic,
// no baseline claim — a multi-pass or per-line-allocation rewrite would show
// up here.
func BenchmarkTruncate(b *testing.B) {
	for _, c := range []struct {
		name string
		text string
		l    Limits
	}{
		{"fits/10KB", strings.Repeat("line under the limit\n", 200), Limits{100000, 1 << 20}},
		{"cut/100KB->50KB", strings.Repeat("a line of tool output\n", 2000), Limits{2000, 50 * 1024}},
		{"cut/1MB->50KB", strings.Repeat("x", 1024*1024), Limits{2000, 50 * 1024}},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Truncate(c.text, c.l)
			}
		})
	}
}
