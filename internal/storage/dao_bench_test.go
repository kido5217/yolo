package storage_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

// benchText repeats a representative prose page (quotes, unicode, newlines)
// until it reaches at least size bytes, then trims to a rune boundary: the
// streamed-content shape without pathological escaping.
func benchText(size int) string {
	page := "stream line: build finished, \"tests\" passed, 42 items — ok\n"
	var sb strings.Builder
	for sb.Len() < size {
		sb.WriteString(page)
	}
	s := sb.String()
	if len(s) > size {
		cut := size
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	return s
}

// benchRow encodes p via ProtocolToPart; the bench inputs are always
// marshalable, so a failure is a bench bug.
func benchRow(b testing.TB, p protocol.Part) storage.PartRow {
	row, err := storage.ProtocolToPart(p)
	if err != nil {
		b.Fatal(err)
	}
	return row
}

// BenchmarkProtocolToPart measures the wire-part -> row encoder on the
// per-delta persist path (dao.go ProtocolToPart): text parts with
// accumulated streamed output, the finalization shape (end + synthetic),
// and a tool part carrying 64KB of streamed input.
func BenchmarkProtocolToPart(b *testing.B) {
	syn := true
	text128 := benchText(128 << 10)
	cases := []struct {
		name string
		p    protocol.Part
	}{
		{"text/1KB", protocol.Part{Type: "text", Text: benchText(1 << 10)}},
		{"text/64KB", protocol.Part{Type: "text", Text: benchText(64 << 10)}},
		{"text/128KB", protocol.Part{Type: "text", Text: text128}},
		{"text/128KB_final", protocol.Part{Type: "text", Text: text128, Time: protocol.PartTime{Start: 1, End: 2}, IsSynthetic: &syn}},
		{"tool/input64KB", protocol.Part{Type: "tool", Tool: "bash", State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": benchText(64 << 10)}, Output: "ok"}}},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			var sink storage.PartRow
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				sink = benchRow(b, c.p)
			}
			_ = sink
		})
	}
}

// BenchmarkUpsertPart measures the per-delta SQLite upsert (dao.go
// UpsertPart) with the state_json sizes the engine persists: the same row
// re-written as the part text grows (the streaming update shape) plus a
// fresh-row pass over a 1024-id ring (rows conflict once the ring wraps).
func BenchmarkUpsertPart(b *testing.B) {
	db, err := storage.Open(filepath.Join(b.TempDir(), "yolo.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })
	if err := db.CreateSession(b.Context(), storage.SessionRow{ID: "ses_bench", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		b.Fatal(err)
	}
	if err := db.CreateMessage(b.Context(), storage.MessageRow{ID: "msg_bench", SessionID: "ses_bench", Role: "assistant", TimeCreated: 2}); err != nil {
		b.Fatal(err)
	}
	for _, size := range []int{1 << 10, 64 << 10, 256 << 10} {
		b.Run(fmt.Sprintf("update/%dKB", size>>10), func(b *testing.B) {
			row := storage.PartRow{
				ID: "prt_bench", MessageID: "msg_bench", SessionID: "ses_bench",
				Type: "text", StateJSON: benchRow(b, protocol.Part{Type: "text", Text: benchText(size)}).StateJSON,
				TimeCreated: 3,
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := db.UpsertPart(b.Context(), row); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("insert/1KB", func(b *testing.B) {
		row := storage.PartRow{
			ID: "prt_bench", MessageID: "msg_bench", SessionID: "ses_bench",
			Type: "text", StateJSON: `{"text":"bench"}`, TimeCreated: 3,
		}
		b.ReportAllocs()
		b.ResetTimer()
		i := 0
		for b.Loop() {
			r := row
			r.ID = fmt.Sprintf("prt_bench_%04d", i&1023)
			i++
			if err := db.UpsertPart(b.Context(), r); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkPartToProtocol measures the row -> wire-part decoder on the
// history-replay path (dao.go PartToProtocol). Rows are produced by the
// real encoder so the decoder sees exactly what ProtocolToPart persists.
func BenchmarkPartToProtocol(b *testing.B) {
	cases := []struct {
		name string
		row  storage.PartRow
	}{
		{"text/64KB", benchRow(b, protocol.Part{Type: "text", Text: benchText(64 << 10)})},
		{"text/128KB", benchRow(b, protocol.Part{Type: "text", Text: benchText(128 << 10)})},
		{"tool/input64KB", benchRow(b, protocol.Part{Type: "tool", Tool: "bash", State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": benchText(64 << 10)}, Output: "ok"}})},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			var sink protocol.Part
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				p, err := storage.PartToProtocol(c.row)
				if err != nil {
					b.Fatal(err)
				}
				sink = p
			}
			_ = sink
		})
	}
}
