package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
)

func TestMentionTriggerIndex(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"", -1, false},
		{"hello", -1, false},
		{"@", 0, true},
		{"@f", 0, true},
		{"fix @f", 4, true},
		{"fix@f", -1, false},
		{"fix @f el", -1, false},
	}
	for _, tc := range tests {
		idx, ok := mentionTriggerIndex(tc.in)
		if idx != tc.want || ok != tc.ok {
			t.Fatalf("mentionTriggerIndex(%q) = (%d,%v), want (%d,%v)", tc.in, idx, ok, tc.want, tc.ok)
		}
	}
}

func TestWalkFiles(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	mk("alpha.go")
	mk("src/gamma.go")
	mk("node_modules/dep.js")
	mk(".git/config")
	got := walkFiles(dir)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "alpha.go") || !strings.Contains(joined, "src/gamma.go") {
		t.Fatalf("walk missed fixture files:\n%s", joined)
	}
	if strings.Contains(joined, "node_modules") || strings.Contains(joined, ".git") {
		t.Fatalf("walk must skip the static ignore set:\n%s", joined)
	}
}

func TestMentionOptions(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"alpha.go", "beta.go", "alpha_beta.go"} {
		os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
	}
	a := testApp()
	a.Service.Dir = dir
	a.prompt.input.SetValue("@al")
	opts := a.mentionOptions()
	if len(opts) == 0 {
		t.Fatal("no options for @al")
	}
	// the prefix match (alpha.go) ranks first (the x2 prefix boost)
	if opts[0].value.(string) != "alpha.go" {
		t.Fatalf("top option = %v, want alpha.go (the prefix match)", opts[0].value)
	}
}

func TestAcInsert(t *testing.T) {
	a := testApp()
	a.Service.Dir = t.TempDir()
	a.prompt.input.SetValue("see @fil")
	a.acInsert("alpha.go")
	if got := a.prompt.input.Value(); got != "see alpha.go" {
		t.Fatalf("insert = %q, want the path replacing the @-query", got)
	}
	if len(a.freq) != 1 || a.freq[0].Path != "alpha.go" || a.freq[0].Frequency != 1 {
		t.Fatalf("frecency not recorded: %v", a.freq)
	}
}

// TestTUIAtPicker is the teatest leg: a real stack, the @-picker filters the
// walked files and enter inserts the selected path.
func TestTUIAtPicker(t *testing.T) {
	ts := testutil.Boot(t)
	os.WriteFile(filepath.Join(ts.Dir, "alpha.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(ts.Dir, "beta.go"), []byte("x"), 0o644)
	c := client.New(ts.URL, ts.Dir) // scope (and the walk) to the server work dir
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine(homeLogoLine), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "see @a")
	teatest.WaitFor(t, tm.Output(), hasLine("alpha.go"), teatest.WithDuration(5*time.Second))
	tm.Send(press(tea.KeyEnter))
	// the inserted value in the drained output (the cell-diff renderer
	// re-emits only the changed cells — the "> " gutter was already
	// drained, so the token stands alone) — asserting over the output,
	// not the live input state (the -race gate, deviation 293).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(stripANSI(string(b)), "alpha.go")
	}, teatest.WithDuration(5*time.Second))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestMentionViewOpenBox pins the S1 chrome (spec §3.1): the @ picker renders
// the shared bordered dropdown (the slash S1 idiom) — the box chrome (the split
// border, the backgroundMenu inner fill, the 1-col padding), the selection row
// SGR'd with the primary bg + SelectedForeground across the full row, and the
// unselected rows (the text path column + the textMuted parent-dir description
// — the slash S2 two-column idiom).
func TestMentionViewOpenBox(t *testing.T) {
	th := yoloDarkTheme(t)
	a := testApp()
	a.prompt.sel = 0
	opts := []selectOption{
		{value: "internal/tui/app.go"},
		{value: "internal/cli/main.go"},
		{value: "alpha.go"},
	}
	const w = 60
	got := a.prompt.acView(opts, w, th)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("box = %d rows, want 3:\n%s", len(lines), got)
	}
	for i, l := range lines {
		plain := stripANSI(l)
		r := []rune(plain)
		if n := len(r); n != w {
			t.Fatalf("row %d = %d cols, want %d (the width-exact box): %q", i, n, w, plain)
		}
		if r[0] != '|' || r[w-1] != '|' {
			t.Fatalf("row %d = %q, want the split left/right border (no top/bottom edges)", i, plain)
		}
		if r[1] != ' ' {
			t.Fatalf("row %d = %q, want 1 padding after the left border", i, plain)
		}
	}
	// the two-column layout on the unselected row 1 (path + the parent-dir
	// description right-offset by "  ").
	if !strings.Contains(stripANSI(lines[1]), "internal/cli/main.go  internal/cli") {
		t.Fatalf("row 1 lost the two-column layout: %q", stripANSI(lines[1]))
	}
	// the unselected row chrome (yolo dark tokens, the S1 pins): the border
	// fg #484848, the backgroundMenu fill #1e1e1e, the label #eeeeee, the
	// description #808080.
	if u := lines[1]; !strings.Contains(u, "38;2;72;72;72") ||
		!strings.Contains(u, "48;2;30;30;30") ||
		!strings.Contains(u, "38;2;238;238;238") ||
		!strings.Contains(u, "38;2;128;128;128") {
		t.Fatalf("row 1 lost the border/fill/label/description SGR: %s", u)
	}
	// the selection row (sel = 0): the primary bg (#fab283) + the
	// SelectedForeground (#0a0a0a) across the full row width.
	if sel := lines[0]; !strings.Contains(sel, "48;2;250;178;131") {
		t.Fatalf("selection row missing the primary bg SGR (#fab283): %s", sel)
	}
	if !strings.Contains(lines[0], "38;2;10;10;10") {
		t.Fatalf("selection row missing the SelectedForeground SGR (#0a0a0a): %s", lines[0])
	}
	// the unselected rows carry no primary paint.
	if strings.Contains(lines[1], "48;2;250;178;131") || strings.Contains(lines[2], "48;2;250;178;131") {
		t.Fatalf("unselected row carries the primary bg: %s / %s", lines[1], lines[2])
	}
}

// TestMentionViewTruncatesPath pins the row content (spec §3.1): the path
// column is truncateMiddle-ed (locale.go) to the box's inner width — the head
// and tail survive, the middle collapses to the ellipsis.
func TestMentionViewTruncatesPath(t *testing.T) {
	th := yoloDarkTheme(t)
	a := testApp()
	a.prompt.sel = 0
	long := strings.Repeat("a", 30) + "file.go"
	opts := []selectOption{{value: long}}
	const w = 40
	got := a.prompt.acView(opts, w, th)
	lines := strings.Split(got, "\n")
	if len(lines) != 1 {
		t.Fatalf("rows = %d, want 1: %s", len(lines), got)
	}
	plain := stripANSI(lines[0])
	// the path is middle-truncated to the box's inner width (avail = w-4 = 36):
	// the head + the "…" + the tail (file.go) survive.
	if !strings.Contains(plain, "…") {
		t.Fatalf("long path not middle-truncated (no ellipsis): %q", plain)
	}
	if !strings.Contains(plain, "file.go") {
		t.Fatalf("truncated path lost the tail: %q", plain)
	}
	if n := len([]rune(plain)); n != w {
		t.Fatalf("row = %d cols, want %d (width-exact): %q", n, w, plain)
	}
}

// TestHomeViewMentionMenuAnchors pins the S1 home anchor (spec §3.2): with the
// @ menu open on the home route, the dropdown frame is anchored above the box
// top edge at boxL/boxW — the same geometry as the slash S3 anchor, the @ menu's
// line count the only delta (only one menu is open at a time, the @-precedence
// gate). The box + hint stay intact below.
func TestHomeViewMentionMenuAnchors(t *testing.T) {
	t.Parallel()
	a := testApp()
	a.size = tea.WindowSizeMsg{Width: mockW, Height: mockH}
	dir := t.TempDir()
	for _, f := range []string{"alpha.go", "beta.go", "delta.go", "epsilon.go", "gamma.go", "zeta.go"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.Service.Dir = dir
	a.prompt.input.SetValue("@")
	a.prompt.sel = 0
	opts := a.mentionOptions()
	if len(opts) == 0 {
		t.Fatalf("the @ menu should be open for input \"@\" (opts = %v)", opts)
	}
	n := len(opts)
	if n > maxDropdownRows {
		n = maxDropdownRows
	}
	rows := fitsFrame(t, a.homeView(nil, "", "", "", "", ""), mockW, mockH)
	// the dropdown's bottom row is right above the box top edge (mockBoxTop-1);
	// the top row (n rows above) is the selected row (sel=0).
	top := mockBoxTop - n
	bottom := mockBoxTop - 1
	atFrame(t, rows, top, mockBoxL, "|")
	atFrame(t, rows, bottom, mockBoxL, "|")
	// the box width (the right border at the box's right edge).
	atFrame(t, rows, bottom, mockBoxL+mockBoxW-1, "|")
	// the logo is overlaid while open: the mock logo row (mockLogoTop+1) now
	// carries the dropdown border at the box's left edge.
	atFrame(t, rows, mockLogoTop+1, mockBoxL, "|")
	// the selected row (sel=0) carries the path column (the first walked file).
	if !strings.Contains(stripANSI(rows[top]), "alpha.go") {
		t.Fatalf("selected @ row = %q, want the first walked file (alpha.go)", stripANSI(rows[top]))
	}
	// the box stays intact below the dropdown.
	atFrame(t, rows, mockBoxTop, mockBoxL, "┃")
}

// TestMentionPrecedenceGate pins the @-precedence rule (spec §3.9): a value
// satisfying both triggers (/cmd @q) renders the @ menu ONLY — the slash menu
// is suppressed (slashActive is false while mentionActive; menuItems returns
// nil, so the slash dropdown does not render).
func TestMentionPrecedenceGate(t *testing.T) {
	t.Run("slashActive is false while mentionActive", func(t *testing.T) {
		a := testApp()
		a.prompt.input.SetValue("/cmd @q")
		if !a.prompt.mentionActive() {
			t.Fatal("the @-trigger should be active for /cmd @q")
		}
		if a.prompt.slashActive() {
			t.Fatal("slashActive must be false while mentionActive (the @-precedence gate)")
		}
	})
	t.Run("menuItems returns nil while mentionActive", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		a.prompt.input.SetValue("/m @a")
		if !a.prompt.mentionActive() {
			t.Fatal("the @-trigger should be active for /m @a")
		}
		if items := a.menuItems(); items != nil {
			t.Fatalf("menuItems = %v, want nil (the @ menu is the only one open)", items)
		}
	})
	t.Run("the view renders the @ menu only (the slash box is absent)", func(t *testing.T) {
		a := testApp()
		a.store.Commands = testCommands()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		a.Service.Dir = dir
		a.prompt.input.SetValue("/m @a")
		if !a.prompt.mentionActive() || a.prompt.slashActive() {
			t.Fatalf("mentionActive/slashActive = %v/%v, want true/false", a.prompt.mentionActive(), a.prompt.slashActive())
		}
		if items := a.menuItems(); items != nil {
			t.Fatalf("menuItems = %v, want nil (the slash menu is suppressed)", items)
		}
		if opts := a.mentionOptions(); len(opts) == 0 {
			t.Fatalf("no @ options for @a (the alpha.go row should match)")
		}
		out := a.view()
		if !strings.Contains(out, "alpha.go") {
			t.Fatalf("the @ menu must render (the alpha.go row): %s", out)
		}
	})
}
