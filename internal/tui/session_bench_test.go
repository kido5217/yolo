package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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
				sink = renderMessages(st, exp, 80, theme.Theme{}, "")
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

// bigPartState wraps one part in the benchStore message envelope (one
// assistant message, the S0-era fixture shape) — the existing
// benchStore(n, expanded) builds per-message 5-part fixtures by count and
// cannot carry a sized part, so the 100 KB case builds its state directly.
func bigPartState(p protocol.Part) *store.State {
	return &store.State{Messages: []protocol.MessageWithParts{{
		Info:  protocol.Message{ID: "msg_big", SessionID: "ses_bench", Role: "assistant", Agent: "build"},
		Parts: []protocol.Part{p},
	}}}
}

// hundredKBPart builds the spec §4 fixture: a ~100 KB text part (~20 KB
// fenced code + ~85 KB prose) — the worst-case re-render input for the
// S1.3/S1.4 path. The original brief's 1800-line code loop built 210,681 B
// (a plan bug, deviation 163); the loop is 290 lines and the pad divisor
// matches the 38-char pad string.
func hundredKBPart() protocol.Part {
	var b strings.Builder
	b.WriteString("Here is a long analysis.\n\n")
	b.WriteString("```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n")
	for i := 0; i < 290; i++ {
		fmt.Fprintf(&b, "\tfmt.Printf(\"line %04d: the quick brown fox jumps over the lazy dog\")\n", i)
	}
	b.WriteString("}\n```\n\n")
	for i := 0; i < 600; i++ {
		b.WriteString("A paragraph of supporting prose that wraps across several terminal lines and carries **bold** and `inline` spans for the renderer to style.\n\n")
	}
	text := b.String()
	if len(text) < 100*1024 {
		// pad to the 100 KB spec size (the gate is about the input size).
		text += strings.Repeat("padding prose to reach the spec size.\n", (100*1024-len(text))/38+1)
	}
	return protocol.Part{ID: "big", Type: "text", Text: text}
}

// TestRenderMessages100KBBudget is the spec §4 budget gate: re-rendering a
// 100 KB part (the streamed-delta batch case) must stay under budget —
// measured as the min of 5 renders after 3 warmups (the renderer is built
// once per renderMessages call, S1.3; the gate measures the RENDER, the
// steady streaming cost).
//
// Budget derivation (deviation 163): measured min-of-5 98.7–101.6 ms
// (2026-08-27, AMD Ryzen 7 5800X3D, glamour v2.0.1, opencode dark, 104,981 B
// spec-shape part: ~20 KB fenced code + ~85 KB prose); the cost is intrinsic
// glamour markdown→ANSI rendering (~0.9–1.0 ms/KB ≈ 90–100 ms for a 100 KB
// part — the plain zero-theme path renders the same state in 1.78 ms,
// ruling out a yolo-side leak); spec §4's 50 ms was derived from a
// pre-glamour scratch measurement that does not reproduce on the current
// tree — the gate value re-baselines per the brief's own knob ("the gate
// value (not the code) is the knob"), logged as deviation 163; 150 ms =
// 1.5× headroom, and the gate still fails on a real regression (a slow
// render path / lost wrap ≥1.5× the ~100 ms floor).
func TestRenderMessages100KBBudget(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	st := bigPartState(hundredKBPart())
	const (
		warmups = 3
		samples = 5
		budget  = 150 * time.Millisecond
	)
	var best time.Duration
	for i := 0; i < warmups+samples; i++ {
		start := time.Now()
		_ = renderMessages(st, nil, 80, th, "")
		if i >= warmups {
			if d := time.Since(start); i == warmups || d < best {
				best = d
			}
		}
	}
	if best >= budget {
		t.Fatalf("100 KB re-render = %v, budget %v (deviation 163 — measured min-of-5 98.7–101.6 ms (2026-08-27, AMD Ryzen 7 5800X3D, glamour v2.0.1, opencode dark, 104,981 B spec-shape part: ~20 KB fenced code + ~85 KB prose); the cost is intrinsic glamour markdown→ANSI rendering (~0.9–1.0 ms/KB ≈ 90–100 ms for a 100 KB part — the plain zero-theme path renders the same state in 1.78 ms, ruling out a yolo-side leak); spec §4's 50 ms was derived from a pre-glamour scratch measurement that does not reproduce on the current tree — the gate value re-baselines per the brief's own knob, logged as deviation 163; 150 ms = 1.5× headroom, and the gate still fails on a real regression (a slow render path / lost wrap ≥1.5× the ~100 ms floor))", best, budget)
	}
}

// BenchmarkRenderMessages_100KBPart is the standing measurement (the gate
// above is the CI assertion; this tracks drift in `go test -bench`).
func BenchmarkRenderMessages_100KBPart(b *testing.B) {
	all, _ := theme.AllThemes()
	r, _ := theme.ResolveTheme(all["opencode"], "dark")
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	st := bigPartState(hundredKBPart())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderMessages(st, nil, 80, th, "")
	}
}
