package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

// benchStore builds a session transcript of n assistant messages, each with
// 5 parts (2 text, 1 reasoning, 1 completed tool, 1 error tool) — the
// per-frame transcript shape the session route re-renders. The width is
// 80 (the plan geometry); expanded toggles which parts show their I/O or
// reasoning body.
func benchStore(n int, expanded bool) *store.Store {
	st := &store.Store{}
	for i := 0; i < n; i++ {
		mid := fmt.Sprintf("msg_bench_%03d", i)
		parts := []protocol.Part{
			{
				ID:        mid + "_t0",
				MessageID: mid,
				SessionID: "ses_bench",
				Type:      "text",
				Text:      strings.Repeat("assistant answer line. ", 16),
			},
			{
				ID:        mid + "_t1",
				MessageID: mid,
				SessionID: "ses_bench",
				Type:      "text",
				Text:      strings.Repeat("another answer line. ", 16),
			},
			{
				ID:        mid + "_r",
				MessageID: mid,
				SessionID: "ses_bench",
				Type:      "reasoning",
				Text:      strings.Repeat("reasoning step. ", 16),
			},
			{
				ID:        mid + "_tool",
				MessageID: mid,
				SessionID: "ses_bench",
				Type:      "tool",
				Tool:      "bash",
				State: &protocol.ToolState{
					Status: "completed",
					Title:  "ls -la /",
					Input:  map[string]any{"command": "ls -la /"},
					Output: strings.Repeat("file line output. ", 8),
				},
			},
			{
				ID:        mid + "_err",
				MessageID: mid,
				SessionID: "ses_bench",
				Type:      "tool",
				Tool:      "grep",
				State: &protocol.ToolState{
					Status: "error",
					Title:  "grep needle",
					Input:  map[string]any{"pattern": "needle"},
					Error:  "grep: pattern error",
				},
			},
		}
		st.Messages = append(st.Messages, protocol.MessageWithParts{
			Info:  protocol.Message{ID: mid, SessionID: "ses_bench", Role: "assistant", Agent: "build"},
			Parts: parts,
		})
	}
	return st
}

// benchExpanded returns the part-id set the alt+e/alt+t toggles would show,
// keyed to every tool and reasoning part when expanded is true.
func benchExpanded(st *store.Store, expanded bool) map[string]bool {
	m := map[string]bool{}
	if !expanded {
		return m
	}
	for _, msg := range st.Messages {
		for _, p := range msg.Parts {
			if p.Type == "tool" || p.Type == "reasoning" {
				m[p.ID] = true
			}
		}
	}
	return m
}

// BenchmarkRenderMessages measures the per-frame transcript composition
// (tui/session.go renderMessages): the message loop, per-part lipgloss
// styling, the tool-row title fallback and the divider join, at width 80.
func BenchmarkRenderMessages(b *testing.B) {
	for _, c := range []struct {
		name     string
		msgs     int
		expanded bool
	}{
		{"collapsed/50", 50, false},
		{"collapsed/200", 200, false},
		{"expanded/200", 200, true},
	} {
		st := benchStore(c.msgs, c.expanded)
		exp := benchExpanded(st, c.expanded)
		b.Run(c.name, func(b *testing.B) {
			var sink string
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sink = renderMessages(st, exp, 80)
			}
			if sink == "" {
				b.Fatal("renderMessages returned empty output")
			}
		})
	}
}
