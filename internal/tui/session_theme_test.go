package tui

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// sessionChromeSGRTokens are the S0.10 session-chrome SGR color parameters
// under the pinned TTY_FORCE=1 + TERM=xterm-256color env (ANSI256 profile:
// the 24-bit hex tokens quantize through x/ansi v0.11.8 Convert256 —
// charmbracelet/x/ansi color.go:185: to6Cube (v<48→0, v<115→1, else
// (v-35)/40 → 0x00/0x5f/0x87/0xaf/0xd7/0xff), an exact cube hit returns
// early, else the grey index (avg-3)/10 with avg>238 → 23 (avg =
// (r+g+b)/3) and a DistanceHSLuv cube-vs-grey tie-break, the cube winning
// ties). Derived from the yolo dark-mode tokens (the S0.2 goldens):
//
//	text (running tool row, prompt cursor)  #eeeeee (238,238,238):
//	    grey 23 exact (238 = 8+10*23) -> 255
//	textMuted (completed tool row)         #808080 (128,128,128):
//	    grey 12 exact (128 = 8+10*12) -> 244
//	error (error tool row, `!` lines, `○ off`/`◌ reconnecting`)  #e06c75 (224,108,117):
//	    cube (215,95,135) -> 16+152 = 168 vs grey 14 (avg 149 -> 148) -> 246:
//	    the achromatic grey wins in HSLuv -> 246
//	success (footer `● live`, provider `● loaded`)  #7fd88f (127,216,143):
//	    cube (135,215,135) -> 16+98 = 114 vs grey 15 (avg 162 -> 158) -> 247:
//	    the green cube beats the achromatic grey in HSLuv -> 114
//
// 244/255 also appear in the S0.8 logo + S0.9 home/footer surfaces; the
// marker text below pins each chrome contribution.
//
// Substring assertions (no escape/terminator boundaries): the renderer's
// pen-diff merges the changed params into ONE CSI whose inner param order
// is not pinned (the redSGR / logoBoldRe precedent).
// The tokens split across the two post-boot merged WaitFors (deviation 141):
// the cell-diff renderer emits a styled run only in the frame its cells
// change, and each WaitFor drains the shared stream — the completed read row
// + live conn dot + running rows + cursor settle before the reject, the
// error row settles after it.
var (
	chromeTokensSettled = []string{
		"38;5;244", // completed tool row (textMuted)
		"38;5;114", // footer `● live` (success)
		"38;5;255", // running tool row (text)
	}
	chromeTokensRejected = []string{
		"38;5;246", // error tool row (error)
	}
)

// Position-anchored row/dot regexes: the CSI that OPENS the styled run must
// carry the token (param order within the CSI not pinned, resets merged in).
var (
	// The row/dot markers are literal UTF-8 runes: Go's regexp has no \u
	// escape (the plan snippet's `\u2713` in a raw string panics at init).
	completedRowRe = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*38;5;244(?:;[0-9]+)*m→ hello.txt`)
	errorRowRe     = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*38;5;246(?:;[0-9]+)*m\$ call_2`)
	liveDotRe      = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*38;5;114(?:;[0-9]+)*m● live`)
	// The prompt cursor: the static virtual cursor's render is
	// fg(text)+reverse (bubbles cursor.View: Style.Inline(true).Reverse(true),
	// Style = fg(Cursor.Color) via SetStyles) — the merged CSI carries both
	// (order not pinned, other params may merge in).
	cursorRe = regexp.MustCompile(`\x1b\[(?:[0-9]+;)*(?:7;38;5;255|38;5;255;7)(?:;[0-9]+)*m`)
)

// TestSessionChromeThemeSGR is the teatest SGR golden for the S0.10 shell
// restyle: boot the app with a REAL theme engine (the S0.7 wiring) over a
// real server whose scripted turn completes a read, rejects a bash ask and
// ends with text, and pin the session-chrome SGR color parameters.
func TestSessionChromeThemeSGR(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "tool", Name: "read", CallID: "call_1", Args: json.RawMessage(`{"filePath":"hello.txt"}`), Finish: "tool_calls"},
		}},
		fake.Turn{Parts: []llm.Part{
			{Kind: "tool", Name: "bash", CallID: "call_2", Args: json.RawMessage(`{"command":"echo hi"}`), Finish: "tool_calls"},
		}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "all done"}}},
	)
	cfg := &protocol.Config{Permission: map[string]any{"bash": "ask"}}
	ts := testutil.BootWithDriverConfig(t, drv, cfg)
	if err := os.WriteFile(filepath.Join(ts.Dir, "hello.txt"), []byte("world\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}
	if got := e.Active(); got != "yolo" {
		t.Fatalf("active theme = %s, want yolo (no config, no KV)", got)
	}

	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		// The fake terminal is not a TTY, so lipgloss strips every style.
		// Pin the env that derives ANSI256 from TERM alone (suite
		// convention).
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	// Merged condition for the session-route drain: the help line + the
	// prompt cursor cell. The home->session frame re-emits the whole chrome
	// (the rows move), so the cursor's merged CSI (fg text + reverse) lands
	// in this drain; the cell-diff renderer never re-emits the unchanged
	// prompt line afterwards, so the cursor is pinned here (deviation 142).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return hasLine("esc abort/back")(b) && cursorRe.Match(b)
	}, teatest.WithDuration(5*time.Second))
	suiteType(tm, "read then bash")
	tm.Send(press(tea.KeyEnter))
	// Merged condition for the dialog drain (consecutive WaitFors drain each
	// other): the perm echo + the chrome already settled by this frame —
	// the completed read row (textMuted), the live conn dot (success), the
	// running rows' text pen — each contribution in one merged condition
	// (deviation 141). The prompt cursor is pinned in the session-route
	// drain above (deviation 142).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		if !hasPermDialogEcho(b) {
			return false
		}
		s := stripANSI(string(b))
		if !strings.Contains(s, "→ hello.txt") || !strings.Contains(s, "\u25CF live") {
			return false
		}
		for _, tok := range chromeTokensSettled {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return completedRowRe.Match(b) && liveDotRe.Match(b)
	}, teatest.WithDuration(10*time.Second))
	// The park lands on the engine's goroutine after the render; sync on it
	// before replying (same guard as TestPermissionDialogKeyReply).
	waitPending(t, ts, 1)
	tm.Send(press('3')) // reject the bash ask -> the tool error part

	// Merged condition for the reject drain: the rejected bash row (error
	// token) + the final text (deviation 141).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, "$ call_2") || !strings.Contains(s, "all done") {
			return false
		}
		for _, tok := range chromeTokensRejected {
			if !bytes.Contains(b, []byte(tok)) {
				return false
			}
		}
		return errorRowRe.Match(b)
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestToolRowLineTheme pins the state->token chain at the lipgloss level
// (pure render, no renderer): the resolved yolo dark tokens emit their
// 24-bit hex as the foreground (the 38;5;N quantization is pinned by the
// teatest golden above).
func TestToolRowLineTheme(t *testing.T) {
	themes, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(themes["yolo"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "yolo", Mode: "dark"}
	tests := []struct {
		name   string
		part   protocol.Part
		want   string
		fgWant string
	}{
		{
			name: "completed -> textMuted",
			part: protocol.Part{ID: "t1", Type: "tool", Tool: "read", CallID: "call_1",
				State: &protocol.ToolState{Status: "completed", Title: "f.go"}},
			want:   "→ f.go",
			fgWant: "#808080",
		},
		{
			name: "running -> text",
			part: protocol.Part{ID: "t2", Type: "tool", Tool: "bash", CallID: "call_2",
				State: &protocol.ToolState{Status: "running", Title: "ls -la"}},
			want:   "~ Writing command...",
			fgWant: "#eeeeee",
		},
		{
			name: "error -> error",
			part: protocol.Part{ID: "t3", Type: "tool", Tool: "grep", CallID: "call_3",
				State: &protocol.ToolState{Status: "error", Title: "grep", Error: "no match"}},
			want:   "✱ grep",
			fgWant: "#e06c75",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, row, ok := toolRow(tt.part, th, "")
			if !ok || row != tt.want {
				t.Fatalf("toolRow = (%q, %v), want (%q, true)", row, ok, tt.want)
			}
			if got, want := st.GetForeground(), hexColor(tt.fgWant); got != want {
				t.Errorf("fg = %v, want %v", got, want)
			}
		})
	}
}

// hexColor parses "#rrggbb" into the color.RGBA that lipgloss v2 stores for
// Foreground(lipgloss.Color(hex)) (lipgloss v2.0.6 color.go parseHex: the
// 24-bit hex lands as an opaque RGBA — Style.GetForeground returns it as
// image/color.Color, so the table's fgWant hex is compared by value).
func hexColor(s string) color.RGBA {
	rgb, _ := hex.DecodeString(s[1:])
	return color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 0xff}
}

// TestSessionChromeZeroThemeIsPlain pins the nil-engine degradation (the
// S0.7 zero-Theme rule): tool rows + the `!` error line + the footer conn
// segment render plain — no SGR from a missing token, never a panic. The
// byte-exact want (the static divider run inlined, the only SGR the
// transcript may carry) also pins the absence of every token SGR.
func TestSessionChromeZeroThemeIsPlain(t *testing.T) {
	s := sessionFixture()
	s.Messages[1].Info.Error = &protocol.MessageError{Type: "unknown", Message: "something broke"}
	got := renderMessages(&s, nil, 80, theme.Theme{}, "")
	want := "User: hello\n" +
		divider.Render(dividerLine()) + "\n" +
		"Thinking\n" +
		"→ src/main.go\n" +
		"~ Writing command...\n" +
		"✱ grep\n" +
		"ok-text\n" +
		"something broke"
	if got != want {
		t.Fatalf("transcript = %q, want %q", got, want)
	}

	a := newRecApp(client.New("http://127.0.0.1:9", ""), store.State{Live: true}, "")
	fv := a.footerView()
	if fv != stripANSI(fv) {
		t.Fatalf("zero-theme footer carries SGR:\n%q", fv)
	}
	if got := stripANSI(fv); got != "no model · default · ↑0 ↓0 · ● live" {
		t.Fatalf("footer = %q", got)
	}
}
