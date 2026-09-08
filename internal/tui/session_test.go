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
				"Thinking\n" +
				"→ src/main.go\n" +
				"~ Writing command...\n" +
				"✱ grep\n" +
				"ok-text",
		},
		{
			name:     "completed tool expanded shows output block",
			expanded: map[string]bool{"t1": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"Thinking\n" +
				"→ src/main.go\n" +
				"  line1\n  line2\n  line3\n" +
				"~ Writing command...\n" +
				"✱ grep\n" +
				"ok-text",
		},
		{
			name:     "error tool expanded shows error block",
			expanded: map[string]bool{"t3": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"Thinking\n" +
				"→ src/main.go\n" +
				"~ Writing command...\n" +
				"✱ grep\n" +
				"  pattern: no match\n  detail line\n" +
				"ok-text",
		},
		{
			// Zero theme: no reasoning renderer is built, so the expanded
			// part shows only the header row (the body needs a real theme).
			name:     "reasoning expanded under zero theme shows header only",
			expanded: map[string]bool{"r1": true},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"Thinking\n" +
				"→ src/main.go\n" +
				"~ Writing command...\n" +
				"✱ grep\n" +
				"ok-text",
		},
		{
			name: "message error renders bare line after parts",
			mutate: func(s *store.State) {
				s.Messages[1].Info.Error = &protocol.MessageError{Type: "unknown", Message: "something broke"}
			},
			want: "User: hello\n" +
				dividerLine() + "\n" +
				"Thinking\n" +
				"→ src/main.go\n" +
				"~ Writing command...\n" +
				"✱ grep\n" +
				"ok-text\n" +
				"something broke",
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
				"Thinking\n" +
				"→ src/main.go\n" +
				"$ ls -la\n" +
				"  l1\n  l2\n  l3\n  l4\n  l5\n  l6\n  l7\n  l8\n  l9\n  l10\n  \u2026\n" +
				"✱ grep\n" +
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
				"Thinking\n" +
				"→ src/main.go\n" +
				"$ ls -la\n" +
				"  a\n  b\n" +
				"✱ grep\n" +
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
				"Thinking\n" +
				"→ src/main.go\n" +
				"$ ls -la\n" +
				"  l1\n  l2\n  l3\n  l4\n  l5\n  l6\n  l7\n  l8\n  l9\n  l10\n  l11\n  l12\n" +
				"✱ grep\n" +
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
				"Thinking\n" +
				"→ src/main.go\n" +
				"~ Writing command...\n" +
				"✱ grep\n" +
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
			got := stripANSI(renderMessages(&s, tt.expanded, 80, theme.Theme{}, ""))
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
	// S1.7: the running row no longer renders the title (only the pending
	// text), so these cases re-target to completed/error states where the
	// title — and thus the fallback — is visible. Fallback coverage is
	// preserved.
	tests := []struct {
		name string
		s    store.State
		want string
	}{
		{
			name: "completed tool without title falls back to command input",
			s: one(protocol.Part{
				ID: "t2", Type: "tool", Tool: "bash", CallID: "call_2",
				State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": "ls -la"}},
			}),
			want: "$ ls -la",
		},
		{
			name: "error tool without title falls back to callID prefix 8",
			s: one(protocol.Part{
				ID: "t8", Type: "tool", Tool: "read", CallID: "call_abcdef1234",
				State: &protocol.ToolState{Status: "error"},
			}),
			want: "→ call_abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(renderMessages(&tt.s, nil, 80, theme.Theme{}, ""))
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
	got := stripANSI(renderMessages(&s, nil, w, theme.Theme{}, ""))
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

// TestRenderUserFileChips pins the spec §8 chip contract: one chip per
// file part, in part order, after the text lines; a file-only message
// renders the "User:" line plus chips.
func TestRenderUserFileChips(t *testing.T) {
	tests := []struct {
		name  string
		parts []protocol.Part
		want  string
	}{
		{"text plus file chip", []protocol.Part{
			{ID: "p1", Type: "text", Text: "hello"},
			{ID: "f1", Type: protocol.PartTypeFile, MIME: "text/plain", Filename: "notes.txt", URL: "data:text/plain;base64,aGVsbG8="},
		}, "User: hello\nfile: notes.txt (text/plain)"},
		{"two files in part order", []protocol.Part{
			{ID: "t1", Type: "text", Text: "hi"},
			{ID: "f1", Type: protocol.PartTypeFile, MIME: "text/plain", Filename: "a.txt", URL: "u1"},
			{ID: "f2", Type: protocol.PartTypeFile, MIME: "application/octet-stream", Filename: "b.dat", URL: "u2"},
		}, "User: hi\nfile: a.txt (text/plain)\nfile: b.dat (application/octet-stream)"},
		{"file-only message", []protocol.Part{
			{ID: "f1", Type: protocol.PartTypeFile, MIME: "text/plain", Filename: "notes.txt", URL: "u"},
		}, "User:\nfile: notes.txt (text/plain)"},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got := renderUser(
				protocol.MessageWithParts{Info: protocol.Message{ID: "m", Role: "user"}, Parts: c.parts},
				120,
			)
			if got != c.want {
				t.Fatalf("renderUser =\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}

// TestSessionViewSlashMenuAnchors pins the S3 session anchor (spec §6 S3):
// with the slash menu open (input "/"), the dropdown overlays the bottom
// viewport rows at the terminal's full width (the left border at col 0), the
// viewport keeps its full height (no slash-menu term in the viewport height —
// the divider sits right below the full-height viewport), and the prompt line
// renders below the dropdown (the divider + help rows in between).
func TestSessionViewSlashMenuAnchors(t *testing.T) {
	t.Parallel()
	a := testSessionApp(store.State{
		Current: &protocol.Session{ID: "ses_1", Title: "my session"},
		Messages: []protocol.MessageWithParts{
			{Info: protocol.Message{ID: "m1", Role: "user"}, Parts: []protocol.Part{{Type: "text", Text: "plan it"}}},
			{Info: protocol.Message{ID: "m2", Role: "assistant"}, Parts: []protocol.Part{{Type: "text", Text: "ok-text"}}},
		},
		Commands: testCommands(),
	})
	a.size = tea.WindowSizeMsg{Width: 80, Height: 24}
	a.prompt.input.SetValue("/")
	a.prompt.sel = 0
	items := a.menuItems()
	if items == nil {
		t.Fatal("slash menu should be open for input \"/\"")
	}
	n := len(items)
	if n > maxDropdownRows {
		n = maxDropdownRows
	}
	// the viewport height carries no slash-menu term (S3 — the overlay
	// replaces the tail rows, it adds no frame rows): vh = h - 1 (title) -
	// 1 (divider) - the help rows - 1 (prompt) - 1 (footer).
	help := len(strings.Split(wrapLine(sessionHelp, 80), "\n"))
	vh := 24 - 4 - help
	rows := strings.Split(a.view(), "\n")
	if len(rows) != 24 {
		t.Fatalf("frame rows = %d, want 24 (the fixed session frame)", len(rows))
	}
	if got := stripANSI(rows[0]); got != "my session" {
		t.Fatalf("title row = %q, want %q", got, "my session")
	}
	// the viewport keeps full height: every viewport row exactly 80 wide
	// (the padded transcript rows) with the transcript content at the top
	// (the user line, the message divider, the assistant line).
	for i := 0; i < vh; i++ {
		if w := runeWidth(stripANSI(rows[1+i])); w != 80 {
			t.Fatalf("viewport row %d width = %d, want 80", i, w)
		}
	}
	if !strings.Contains(stripANSI(rows[1]), "plan it") {
		t.Fatalf("viewport row 0 = %q, want the user line", rows[1])
	}
	if !strings.HasPrefix(stripANSI(rows[2]), dividerLine()) {
		t.Fatalf("viewport row 1 = %q, want the message divider", rows[2])
	}
	if !strings.Contains(stripANSI(rows[3]), "ok-text") {
		t.Fatalf("viewport row 2 = %q, want the assistant line", rows[3])
	}
	// the dropdown overlays the bottom viewport rows (n rows, bottom-aligned
	// above the prompt block): the full-width box (the left border at col 0,
	// the right border at col 79), the selected row (sel=0) on top.
	top := 1 + vh - n
	bottom := vh
	for i := top; i <= bottom; i++ {
		atFrame(t, rows, i, 0, "|")
		atFrame(t, rows, i, 79, "|")
	}
	if !strings.Contains(stripANSI(rows[top]), "/sessions") {
		t.Fatalf("selected dropdown row = %q, want the first merged command (/sessions)", rows[top])
	}
	// the divider + the help pin the full-height viewport (no menuLines
	// reduction — the divider is right below the last dropdown row).
	if got := stripANSI(rows[bottom+1]); got != dividerLine() {
		t.Fatalf("divider row = %q, want the divider", got)
	}
	if !strings.Contains(stripANSI(rows[bottom+2]), "pgup/pgdn scroll") {
		t.Fatalf("help row = %q, want the session help line", rows[bottom+2])
	}
	// the prompt line renders below the dropdown; the footer on the last row.
	if got := stripANSI(rows[bottom+3]); got != stripANSI(a.prompt.view()) {
		t.Fatalf("prompt row = %q, want the prompt line", got)
	}
	if !strings.Contains(stripANSI(rows[23]), "no model") {
		t.Fatalf("footer row = %q, want the session footer", rows[23])
	}
}
