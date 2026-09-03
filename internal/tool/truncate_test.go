package tool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCleanOutputDir pins the retention sweep: only tool_* files older than
// the 7-day window are removed; fresh outputs and non-tool files survive.
func TestCleanOutputDir(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	oldF := filepath.Join(d, "tool_old")
	recent := filepath.Join(d, "tool_recent")
	other := filepath.Join(d, "notes.txt")
	for _, f := range []string{oldF, recent, other} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(oldF, past, past); err != nil {
		t.Fatal(err)
	}
	if err := CleanOutputDir(d); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldF); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old output not removed (stat err=%v)", err)
	}
	for _, f := range []string{recent, other} {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("%s should survive cleanup: %v", filepath.Base(f), err)
		}
	}
	// A missing dir is a no-op (first run before any truncation).
	if err := CleanOutputDir(filepath.Join(d, "does-not-exist")); err != nil {
		t.Fatalf("missing dir: %v", err)
	}
}

func TestTruncateTail(t *testing.T) {
	t.Parallel()
	lines := make([]string, 3000)
	for i := range lines {
		lines[i] = "line"
	}
	out, cut := Truncate(strings.Join(lines, "\n"), Limits{100, 50 * 1024})
	if !cut {
		t.Fatal("want cut")
	}
	got := strings.Split(out, "\n")
	if len(got) != 100 {
		t.Fatalf("lines = %d", len(got))
	}
}

func TestTruncateSingleLineUTF8Cut(t *testing.T) {
	t.Parallel()
	// upstream tail(): a single over-long line keeps its LAST MaxBytes
	// bytes, advanced to a UTF-8 boundary. One 80000-byte line (40000 x
	// U+00E9, 2 bytes each) cuts at byte 28800 (already a boundary) and
	// keeps 51200 bytes = 25600 runes.
	out, cut := Truncate(strings.Repeat("\u00e9", 40000), Limits{100, 50 * 1024})
	if !cut {
		t.Fatal("want cut")
	}
	if r := len([]rune(out)); r != 25600 {
		t.Fatalf("runes = %d, want 25600", r)
	}
	if b := len(out); b != 51200 {
		t.Fatalf("bytes = %d, want 51200", b)
	}
	// mid-rune cut: 51300-byte line of U+65E5 (3 bytes each) cuts at byte
	// 100, inside a rune; the boundary advance skips to byte 102, keeping
	// 51198 bytes = 17066 runes.
	out, cut = Truncate(strings.Repeat("\u65e5", 17100), Limits{100, 50 * 1024})
	if !cut {
		t.Fatal("want cut")
	}
	if r := len([]rune(out)); r != 17066 {
		t.Fatalf("runes = %d, want 17066", r)
	}
	if b := len(out); b != 51198 {
		t.Fatalf("bytes = %d, want 51198", b)
	}
}

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
			for b.Loop() {
				Truncate(c.text, c.l)
			}
		})
	}
}
