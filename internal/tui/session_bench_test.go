package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// benchStore builds a session transcript of n assistant messages, each with
// 5 parts (2 text, 1 reasoning, 1 completed tool, 1 error tool) — the
// per-frame transcript shape the session route re-renders. The width is
// 80 (the plan geometry); expanded toggles which parts show their I/O or
// reasoning body.
func benchStore(n int, expanded bool) *store.State {
	st := &store.State{}
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
func benchExpanded(st *store.State, expanded bool) map[string]bool {
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
				sink = renderMessages(st, exp, 80, theme.Theme{})
			}
			if sink == "" {
				b.Fatal("renderMessages returned empty output")
			}
		})
	}
}

// BenchmarkStoreApply pins the per-event fold path (candidate-9): the
// message/part upserts plus the delta burst through the per-part builder
// (appendDelta zero-copy). Hermetic — no baseline claim; a future
// quadratic re-scan or builder reset would show up here.
func BenchmarkStoreApply(b *testing.B) {
	for _, n := range []int{50, 200} {
		b.Run(fmt.Sprintf("msgs/%d", n), func(b *testing.B) {
			evs := applyBenchEvents(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				st := &store.State{}
				st.Current = &protocol.Session{ID: "ses_bench"}
				for _, ev := range evs {
					st.Apply(ev)
				}
			}
		})
	}
}

// applyBenchEvents builds the SSE stream for n assistant messages in the
// benchStore shape: one message.updated, one part.updated per part, and a
// 20-delta burst per text part (the streamed hot path).
func applyBenchEvents(n int) []protocol.Event {
	evs := make([]protocol.Event, 0, n*8)
	add := func(typ string, props any) {
		raw, err := json.Marshal(props)
		if err != nil {
			panic(err)
		}
		evs = append(evs, protocol.Event{Type: typ, Properties: raw})
	}
	for i := 0; i < n; i++ {
		mid := fmt.Sprintf("msg_bench_%03d", i)
		msg := protocol.Message{ID: mid, SessionID: "ses_bench", Role: "assistant", Agent: "build"}
		add(protocol.EventTypeMessageUpdated, map[string]any{"sessionID": "ses_bench", "info": msg})
		for _, ptype := range []string{"text", "text", "reasoning", "tool", "tool"} {
			pid := mid + "_" + ptype + fmt.Sprint(i)
			part := protocol.Part{
				ID: pid, MessageID: mid, SessionID: "ses_bench", Type: ptype,
				Text: strings.Repeat("line. ", 8),
			}
			if ptype == "tool" {
				part.Tool = "grep"
				part.State = &protocol.ToolState{Status: "completed", Title: "grep needle"}
			}
			add(protocol.EventTypeMessagePartUpdated, map[string]any{"part": part})
		}
		// delta burst on the first text part
		for d := 0; d < 20; d++ {
			add(protocol.EventTypeMessagePartDelta, map[string]any{
				"sessionID": "ses_bench", "messageID": mid,
				"partID": mid + "_text" + fmt.Sprint(i), "field": "text", "delta": "tok",
			})
		}
	}
	return evs
}
