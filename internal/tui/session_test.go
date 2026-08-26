package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// sessionFixture is the T24 render fixture: one user message plus one
// assistant message with reasoning, three tool parts (completed, running,
// error) and a text part, in that order.
func sessionFixture() store.State {
	s := store.State{Current: &protocol.Session{ID: "ses_0", Title: "T"}}
	s.Messages = []protocol.MessageWithParts{
		{
			Info:  protocol.Message{ID: "m1", Role: "user"},
			Parts: []protocol.Part{{ID: "p1", Type: "text", Text: "hello"}},
		},
		{
			Info: protocol.Message{ID: "m2", Role: "assistant"},
			Parts: []protocol.Part{
				{ID: "r1", Type: "reasoning", Text: "because x\nand y"},
				{ID: "t1", Type: "tool", Tool: "read", CallID: "call_1", State: &protocol.ToolState{
					Status: "completed", Title: "src/main.go", Output: "line1\nline2\nline3",
				}},
				{ID: "t2", Type: "tool", Tool: "bash", CallID: "call_2", State: &protocol.ToolState{
					Status: "running", Input: map[string]any{"command": "ls -la"}, Title: "ls -la",
				}},
				{ID: "t3", Type: "tool", Tool: "grep", CallID: "call_3", State: &protocol.ToolState{
					Status: "error", Title: "grep", Error: "pattern: no match\ndetail line",
				}},
				{ID: "z1", Type: "text", Text: "ok-text"},
			},
		},
	}
	return s
}

func TestRenderMessages(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(s *store.State)
		expanded map[string]bool
		empty    bool // use the empty store instead of sessionFixture()
		want     string
	}{
		{
			name: "collapsed",
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"\u25B6 bash ls -la\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text",
		},
		{
			name:     "completed tool expanded shows output block",
			expanded: map[string]bool{"t1": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"  line1\n  line2\n  line3\n" +
				"\u25B6 bash ls -la\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text",
		},
		{
			name:     "error tool expanded shows error block",
			expanded: map[string]bool{"t3": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"\u25B6 bash ls -la\n" +
				"\u2717 grep pattern: no match\n" +
				"  pattern: no match\n  detail line\n" +
				"ok-text",
		},
		{
			name:     "reasoning expanded shows indented text",
			expanded: map[string]bool{"r1": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25BE think\n" +
				"  because x\n  and y\n" +
				"\u2713 read src/main.go\n" +
				"\u25B6 bash ls -la\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text",
		},
		{
			name: "message error renders red line after parts",
			mutate: func(s *store.State) {
				s.Messages[1].Info.Error = &protocol.MessageError{Type: "unknown", Message: "something broke"}
			},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"\u25B6 bash ls -la\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text\n" +
				"! something broke",
		},
		{
			name:  "empty store renders nothing",
			empty: true,
			want:  "",
		},
		{
			// Upstream parity: a completed bash part shows its output
			// inline (10-line head, "…" overflow hint) without alt+e.
			name: "completed bash collapsed shows 10-line head preview",
			mutate: func(s *store.State) {
				s.Messages[1].Parts[2].State = &protocol.ToolState{
					Status: "completed", Title: "ls -la",
					Output: "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12",
				}
			},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"\u2713 bash ls -la\n" +
				"  l1\n  l2\n  l3\n  l4\n  l5\n  l6\n  l7\n  l8\n  l9\n  l10\n  \u2026\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text",
		},
		{
			name: "completed bash with short output shows it fully, no hint",
			mutate: func(s *store.State) {
				s.Messages[1].Parts[2].State = &protocol.ToolState{
					Status: "completed", Title: "ls -la", Output: "a\nb",
				}
			},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"\u2713 bash ls -la\n" +
				"  a\n  b\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text",
		},
		{
			// Expanded: the existing tail block alone (no duplicated preview).
			name: "completed bash expanded shows tail block without preview",
			mutate: func(s *store.State) {
				s.Messages[1].Parts[2].State = &protocol.ToolState{
					Status: "completed", Title: "ls -la",
					Output: "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12",
				}
			},
			expanded: map[string]bool{"t2": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"\u2713 bash ls -la\n" +
				"  l1\n  l2\n  l3\n  l4\n  l5\n  l6\n  l7\n  l8\n  l9\n  l10\n  l11\n  l12\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text",
		},
		{
			// Regression: an expanded tool with empty output must not drop the
			// parts rendered after it (the old `break` escaped the parts loop).
			name: "expanded empty-output tool keeps later parts",
			mutate: func(s *store.State) {
				s.Messages[1].Parts[1].State.Output = ""
			},
			expanded: map[string]bool{"t1": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"\u25B8 think\n" +
				"\u2713 read src/main.go\n" +
				"\u25B6 bash ls -la\n" +
				"\u2717 grep pattern: no match\n" +
				"ok-text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := sessionFixture()
			if tt.empty {
				s = store.State{Current: &protocol.Session{ID: "ses_0"}}
			}
			if tt.mutate != nil {
				tt.mutate(&s)
			}
			got := stripANSI(renderMessages(&s, tt.expanded, 80, theme.Theme{}))
			if got != tt.want {
				t.Errorf("renderMessages mismatch:\ngot:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestRenderMessagesTitleFallbacks(t *testing.T) {
	one := func(parts ...protocol.Part) store.State {
		s := store.State{Current: &protocol.Session{ID: "ses_0"}}
		s.Messages = []protocol.MessageWithParts{
			{Info: protocol.Message{ID: "m1", Role: "assistant"}, Parts: parts},
		}
		return s
	}
	tests := []struct {
		name string
		s    store.State
		want string
	}{
		{
			name: "running tool without title falls back to command input",
			s: one(protocol.Part{
				ID: "t2", Type: "tool", Tool: "bash", CallID: "call_2",
				State: &protocol.ToolState{Status: "running", Input: map[string]any{"command": "ls -la"}},
			}),
			want: "\u25B6 bash ls -la",
		},
		{
			name: "nil state falls back to callID prefix 8",
			s: one(protocol.Part{
				ID: "t8", Type: "tool", Tool: "read", CallID: "call_abcdef1234",
			}),
			want: "\u25B6 read call_abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(renderMessages(&tt.s, nil, 80, theme.Theme{}))
			if got != tt.want {
				t.Errorf("renderMessages = %q, want %q", got, tt.want)
			}
		})
	}
}

func testSessionApp(s store.State) *recApp {
	return newRecApp(client.New("http://127.0.0.1:9", ""), s, "ses_0")
}

func TestSessionKeys(t *testing.T) {
	// T25 (deviation 51): with the always-focused prompt, plain e/t must type
	// into the input, so the session toggles moved to alt+e / alt+t.
	t.Run("alt+e toggles the most recent tool part", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.handleKey(pressAlt('e'))
		if len(a.sess.expanded) != 1 || !a.sess.expanded["t3"] {
			t.Fatalf("expanded = %v, want {t3:true}", a.sess.expanded)
		}
		a.handleKey(pressAlt('e'))
		if len(a.sess.expanded) != 0 {
			t.Fatalf("expanded = %v, want {}", a.sess.expanded)
		}
	})

	t.Run("alt+e is a no-op without tool parts", func(t *testing.T) {
		s := store.State{
			Current: &protocol.Session{ID: "ses_0"},
			Messages: []protocol.MessageWithParts{{
				Info:  protocol.Message{ID: "m1", Role: "assistant"},
				Parts: []protocol.Part{{ID: "p1", Type: "text", Text: "hi"}},
			}},
		}
		a := testSessionApp(s)
		a.handleKey(pressAlt('e'))
		if len(a.sess.expanded) != 0 {
			t.Fatalf("expanded = %v, want {}", a.sess.expanded)
		}
	})

	t.Run("alt+t toggles all reasoning parts", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.handleKey(pressAlt('t'))
		if len(a.sess.expanded) != 1 || !a.sess.expanded["r1"] {
			t.Fatalf("expanded = %v, want {r1:true}", a.sess.expanded)
		}
		a.handleKey(pressAlt('t'))
		if len(a.sess.expanded) != 0 {
			t.Fatalf("expanded = %v, want {}", a.sess.expanded)
		}
	})

	t.Run("esc while busy aborts and stays on session", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.store.Status = protocol.SessionStatus{Type: protocol.SessionStatusBusy}
		a.handleKey(press(tea.KeyEscape))
		if a.route != routeSession || a.curSessionID != "ses_0" {
			t.Fatalf("route=%v cur=%s, want routeSession/ses_0", a.route, a.curSessionID)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 abort cmd", len(a.Cmds))
		}
	})

	t.Run("esc while idle returns to home", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		a.handleKey(press(tea.KeyEscape))
		if a.route != routeHome || a.curSessionID != "" {
			t.Fatalf("route=%v cur=%s, want routeHome/empty", a.route, a.curSessionID)
		}
		if len(a.Cmds) != 1 {
			t.Fatalf("recorded %d cmds, want 1 hydrate cmd", len(a.Cmds))
		}
	})

	t.Run("pgup pauses follow, pgdn to bottom resumes it", func(t *testing.T) {
		a := testSessionApp(sessionFixture())
		if !a.sess.following {
			t.Fatal("follow must start true")
		}
		a.view()
		a.handleKey(tea.KeyPressMsg{Code: tea.KeyPgUp})
		if a.sess.following {
			t.Fatal("follow must be false after pgup")
		}
		a.handleKey(tea.KeyPressMsg{Code: tea.KeyPgDown})
		if !a.sess.following {
			t.Fatal("follow must resume at bottom after pgdn")
		}
	})
}

// TestRenderMessagesWrapsLongLines: a single line longer than the viewport
// width must be word-wrapped — every rendered line fits the width and no
// word is lost (the pre-wrap build clipped the over-width line at the edge,
// leaving the tail unreachable: no horizontal scroll is bound).
func TestRenderMessagesWrapsLongLines(t *testing.T) {
	s := store.State{Current: &protocol.Session{ID: "ses_0"}}
	s.Messages = []protocol.MessageWithParts{
		{
			Info:  protocol.Message{ID: "m1", Role: "user"},
			Parts: []protocol.Part{{ID: "p1", Type: "text", Text: "print me 1000 words about anime"}},
		},
		{
			Info: protocol.Message{ID: "m2", Role: "assistant"},
			Parts: []protocol.Part{{
				ID: "p2", Type: "text",
				Text: "one two three four five six seven eight nine ten",
			}},
		},
	}
	// w must be >= the locked 28-rune divider.
	const w = 30
	got := stripANSI(renderMessages(&s, nil, w, theme.Theme{}))
	want := "User: print me 1000 words\n" +
		"about anime\n" +
		dividerLine() + "\n" +
		"one two three four five six\n" +
		"seven eight nine ten"
	if got != want {
		t.Fatalf("renderMessages mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
	for _, l := range strings.Split(got, "\n") {
		if len([]rune(l)) > w {
			t.Errorf("line wider than %d: %q", w, l)
		}
	}
}
