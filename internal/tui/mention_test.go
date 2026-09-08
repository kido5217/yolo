package tui

import (
	"fmt"
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
	mk := func(dir, rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	t.Run("files + directories in preorder (the dir rows trailing-/ marked)", func(t *testing.T) {
		dir := t.TempDir()
		mk(dir, "alpha.go")
		mk(dir, "src/gamma.go")
		mk(dir, "node_modules/dep.js")
		mk(dir, ".git/config")
		got := walkFiles(dir)
		// the directory is recorded at visit time (preorder — before
		// descending), with the trailing-/ display marker.
		want := strings.Join([]string{"alpha.go", "src/", "src/gamma.go"}, "\n")
		if joined := strings.Join(got, "\n"); joined != want {
			t.Fatalf("walk = %q, want files + directories in preorder (the static ignore set pruned):\n%s", joined, want)
		}
	})
	t.Run("the gitignore prune applies to files and directories", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored/\nsecret.txt\n"), 0o644)
		mk(dir, "keep.go")
		mk(dir, "ignored/inner.go")
		mk(dir, "secret.txt")
		got := walkFiles(dir)
		// the .gitignore file itself is walked (not in the ignore set); the
		// pruned dir (ignored/) + file (secret.txt) are absent.
		want := strings.Join([]string{".gitignore", "keep.go"}, "\n")
		if joined := strings.Join(got, "\n"); joined != want {
			t.Fatalf("walk = %q, want the gitignore prune applied to files + directories:\n%s", joined, want)
		}
	})
	t.Run("the file cap counts file entries only (dir rows do not consume it)", func(t *testing.T) {
		dir := t.TempDir()
		for i := 0; i < maxWalkFiles+1; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%04d.go", i)), []byte("x"), 0o644)
		}
		os.MkdirAll(filepath.Join(dir, "ddir1"), 0o755)
		os.MkdirAll(filepath.Join(dir, "ddir2"), 0o755)
		files, dirs := 0, 0
		for _, p := range walkFiles(dir) {
			if strings.HasSuffix(p, "/") {
				dirs++
			} else {
				files++
			}
		}
		if files != maxWalkFiles {
			t.Fatalf("file entries = %d, want the file cap %d", files, maxWalkFiles)
		}
		if dirs != 2 {
			t.Fatalf("dir entries = %d, want 2 (dir rows do not consume the file cap)", dirs)
		}
	})
}

func TestMentionOptions(t *testing.T) {
	mkTree := func(t *testing.T, dir string, files ...string) {
		for _, f := range files {
			p := filepath.Join(dir, filepath.FromSlash(f))
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte("x"), 0o644)
		}
	}
	t.Run("the prefix match ranks first", func(t *testing.T) {
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
		mo, ok := opts[0].value.(mentionOption)
		if !ok || mo.path != "alpha.go" || mo.isDir {
			t.Fatalf("top option = %v, want alpha.go (the prefix match)", opts[0].value)
		}
	})
	t.Run("the positive-score gate filters the weak scattered subsequence", func(t *testing.T) {
		dir := t.TempDir()
		for _, f := range []string{"abacus.go", "xaby.go"} {
			os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644)
		}
		a := testApp()
		a.Service.Dir = dir
		a.prompt.input.SetValue("@ab")
		opts := a.mentionOptions()
		// abacus.go is the prefix match (the x2 boost, a positive score);
		// xaby.go is only a weak scattered subsequence (the fuzzy score <= 0)
		// — the positive-score gate filters it (the threshold-0.5 port).
		if len(opts) != 1 {
			t.Fatalf("options = %d, want 1 (the weak scattered match filtered): %v", len(opts), opts)
		}
		mo, _ := opts[0].value.(mentionOption)
		if mo.path != "abacus.go" {
			t.Fatalf("top option = %q, want abacus.go (the prefix match)", mo.path)
		}
	})
	t.Run("the empty query lists all files + directories (frecency order; walk order where zero)", func(t *testing.T) {
		dir := t.TempDir()
		mkTree(t, dir, "alpha.go", "beta.go", "tools/inner.go")
		a := testApp()
		a.Service.Dir = dir
		a.freq = []frecencyEntry{{Path: "beta.go", Frequency: 5, LastOpen: testNow}}
		a.prompt.input.SetValue("@")
		opts := a.mentionOptions()
		// beta.go ranks first (the frecency order); the rest keep walk order
		// (the files + the dir row, the fuzzy/insert target WITHOUT the
		// trailing-/ display marker).
		want := []string{"beta.go", "alpha.go", "tools", "tools/inner.go"}
		if len(opts) != len(want) {
			t.Fatalf("options = %d, want %d: %v", len(opts), len(want), opts)
		}
		for i, w := range want {
			mo, ok := opts[i].value.(mentionOption)
			if !ok || mo.path != w {
				t.Fatalf("option %d = %v, want %s", i, opts[i].value, w)
			}
			if i == 2 && !mo.isDir {
				t.Fatalf("option 2 = %v, want the tools dir row (isDir)", opts[i].value)
			}
		}
	})
	t.Run("the empty query caps at maxPickerOptions rows", func(t *testing.T) {
		dir := t.TempDir()
		for i := 0; i < 12; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("a%02d.go", i)), []byte("x"), 0o644)
		}
		a := testApp()
		a.Service.Dir = dir
		a.prompt.input.SetValue("@")
		opts := a.mentionOptions()
		if len(opts) != maxPickerOptions {
			t.Fatalf("options = %d, want the cap %d", len(opts), maxPickerOptions)
		}
		mo, _ := opts[len(opts)-1].value.(mentionOption)
		if mo.path != "a09.go" {
			t.Fatalf("last option = %q, want a09.go (walk order, capped)", mo.path)
		}
	})
	t.Run("the non-empty query runs over the merged file+dir pool", func(t *testing.T) {
		dir := t.TempDir()
		mkTree(t, dir, "alpha.go", "beta.go", "tools/inner.go")
		a := testApp()
		a.Service.Dir = dir
		a.prompt.input.SetValue("@tool")
		opts := a.mentionOptions()
		// the tools dir (the fuzzy target is the slash-relative path WITHOUT
		// the trailing / — the insert value) + its file; alpha/beta carry no
		// "tool" subsequence.
		if len(opts) != 2 {
			t.Fatalf("options = %d, want 2 (the merged file+dir pool): %v", len(opts), opts)
		}
		var sawDir, sawFile bool
		for _, o := range opts {
			mo, ok := o.value.(mentionOption)
			if !ok {
				t.Fatalf("option value = %v, want mentionOption", o.value)
			}
			if strings.HasSuffix(mo.path, "/") {
				t.Fatalf("the insert value = %q, want no trailing /", mo.path)
			}
			if mo.path == "tools" && mo.isDir {
				sawDir = true
			}
			if mo.path == "tools/inner.go" && !mo.isDir {
				sawFile = true
			}
		}
		if !sawDir || !sawFile {
			t.Fatalf("options = %v, want the tools dir row + tools/inner.go", opts)
		}
	})
	t.Run("the selection resets to 0 on a query change", func(t *testing.T) {
		a := testApp()
		dir := t.TempDir()
		mkTree(t, dir, "alpha.go", "alpha_beta.go")
		a.Service.Dir = dir
		// type the @-query through the key path (the input fallback): "@"
		// opens the menu, "a" filters — then move the selection and type a
		// new character: the ported filter-rerun reset.
		a.handleKey(press('@'))
		a.handleKey(press('a'))
		a.prompt.sel = 1
		a.handleKey(press('l'))
		if got := a.prompt.input.Value(); got != "@al" {
			t.Fatalf("input = %q, want @al", got)
		}
		if a.prompt.sel != 0 {
			t.Fatalf("sel = %d, want 0 (the @-query changed a -> al)", a.prompt.sel)
		}
	})
}

func TestAcInsert(t *testing.T) {
	t.Run("the file insert is @-prefixed + a trailing space (the insert touch)", func(t *testing.T) {
		a := testApp()
		a.Service.Dir = t.TempDir()
		a.prompt.input.SetValue("fix @")
		a.acInsert(mentionOption{path: "alpha.go"})
		if got := a.prompt.input.Value(); got != "fix @alpha.go " {
			t.Fatalf("insert = %q, want the @-prefixed path + a trailing space", got)
		}
		if len(a.freq) != 1 || a.freq[0].Path != "alpha.go" || a.freq[0].Frequency != 1 {
			t.Fatalf("frecency not recorded (the insert touch): %v", a.freq)
		}
	})
	t.Run("the directory tab-expand keeps the menu open, no touch", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("x"), 0o644)
		os.MkdirAll(filepath.Join(dir, "cmd"), 0o755)
		os.WriteFile(filepath.Join(dir, "cmd", "main.go"), []byte("x"), 0o644)
		a := testApp()
		a.Service.Dir = dir
		a.prompt.input.SetValue("fix @")
		a.acExpand(mentionOption{path: "cmd", isDir: true})
		// the @<path>/ insert: NO trailing space (the value ends at "/").
		if got := a.prompt.input.Value(); got != "fix @cmd/" {
			t.Fatalf("expand = %q, want @cmd/ (no trailing space)", got)
		}
		if a.prompt.sel != 0 {
			t.Fatalf("sel = %d, want 0 (the expand resets sel)", a.prompt.sel)
		}
		// the value still carries a valid @-trigger: the menu stays open and
		// the query re-filters the candidates to the cmd/ subtree.
		if !a.prompt.mentionActive() {
			t.Fatal("the menu should stay open after the expand (a valid @-trigger remains)")
		}
		if got := a.prompt.acQuery(); got != "cmd/" {
			t.Fatalf("query = %q, want cmd/ (the re-filter target)", got)
		}
		var sawSubtree, sawOutside bool
		for _, o := range a.mentionOptions() {
			mo, _ := o.value.(mentionOption)
			switch mo.path {
			case "cmd/main.go":
				sawSubtree = true
			case "alpha.go":
				sawOutside = true
			}
		}
		if !sawSubtree {
			t.Fatal("the re-filter should keep the cmd/ subtree (cmd/main.go)")
		}
		if sawOutside {
			t.Fatal("the re-filter should drop the non-subtree file (alpha.go)")
		}
		if len(a.freq) != 0 {
			t.Fatalf("the dir-expand branch must not touch the frecency: %v", a.freq)
		}
	})
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
	suiteType(tm, "see @")
	teatest.WaitFor(t, tm.Output(), hasLine("alpha.go"), teatest.WithDuration(5*time.Second))
	tm.Send(press(tea.KeyEnter))
	// the inserted value in the drained output (the cell-diff renderer
	// re-emits only the changed cells — the "> " gutter and the "@" trigger
	// were already drained, so the inserted path + trailing space stand
	// alone) — the S3 @-prefixed insert ("see @alpha.go "); asserting over
	// the output, not the live input state (the -race gate, deviation 293).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(stripANSI(string(b)), "alpha.go ")
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

// TestMentionViewEmptyLine pins the S6 empty-state case (spec §3.7): the
// no-candidate render (an @-query matching nothing) renders the parity text
// "No matching items" (upstream verbatim) in the textMuted fg — the fresh pin
// (this slice introduces the text; the old "no match" pin did not exist, the
// plan's spec §2 referent is stale). Mirrors TestPromptMenuEmptyLine (the
// slash S7 idiom).
func TestMentionViewEmptyLine(t *testing.T) {
	th := yoloDarkTheme(t)
	a := testApp()
	got := a.prompt.acView([]selectOption{}, 60, th)
	lines := strings.Split(got, "\n")
	if len(lines) != 1 {
		t.Fatalf("empty render = %d lines, want 1 (the no-match line):\n%s", len(lines), got)
	}
	if !strings.Contains(stripANSI(lines[0]), "No matching items") {
		t.Fatalf("no-match line lost the parity text: %q", stripANSI(lines[0]))
	}
	if !strings.Contains(lines[0], "38;2;128;128;128") {
		t.Fatalf("no-match line missing the textMuted fg SGR (#808080): %s", lines[0])
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

// TestTUIAtPickerMouseHover drives a mouse motion over a row of the open
// @-picker (the S4 mouse, spec §3.8) and asserts the full acceptance chain:
// the motion moves the selection to the hovered row, and the next enter
// inserts THAT row (not the original sel). The teatest leg sends a
// tea.MouseMsg at the S1 anchor for row 1 (mentionDropdownRows — the
// slash-epic tm.Send tea.Msg idiom, cell coordinates, not pixels). The
// model is asserted after the program has quit (not a race with the running
// program, deviation 293's class — the cell-diff renderer re-emits only the
// changed cells, so the insert's gutter and "@" trigger were already
// drained and the model state is the sharp pin, TestTUIAtPicker's idiom).
func TestTUIAtPickerMouseHover(t *testing.T) {
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
	suiteType(tm, "see @")
	teatest.WaitFor(t, tm.Output(), hasLine("beta.go"), teatest.WithDuration(5*time.Second))

	// the S1 anchor for the open @-picker: the first row's cell + the
	// visible count (two rows for the two walked files, at sel 0).
	firstRow, vis := a.mentionDropdownRows()
	if firstRow < 0 || vis != 2 {
		t.Fatalf("@-picker not open (firstRow=%d vis=%d)", firstRow, vis)
	}

	// a motion over row 1 (a non-top row) moves the selection there; the
	// next enter inserts row 1 (beta.go), not the original sel 0 (alpha.go).
	tm.Send(tea.MouseMotionMsg{X: 10, Y: firstRow + 1, Button: tea.MouseNone})
	tm.Send(press(tea.KeyEnter))
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	if got := a.prompt.input.Value(); got != "see @beta.go " {
		t.Fatalf("insert = %q, want the hovered row's @-prefixed insert (see @beta.go )", got)
	}
	if a.prompt.mentionActive() {
		t.Fatal("the @ menu should be closed after the insert (the trailing space kills the trigger)")
	}
}

// TestTUIAtPickerMouseClick drives a mouse click on the first row of the
// open @-picker (the S4 mouse, spec §3.8) and asserts the click is the
// ENTER action — the selected option inserts (the @-prefixed path + a
// trailing space) and the menu closes. NO expand on click (spec §3.8:
// only tab expands — a directory row clicked inserts and closes). The
// teatest leg sends a tea.MouseMsg at the S1 anchor for row 0
// (mentionDropdownRows); the model is asserted after the program has quit
// (the cell-diff note — see TestTUIAtPickerMouseHover).
func TestTUIAtPickerMouseClick(t *testing.T) {
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
	suiteType(tm, "see @")
	teatest.WaitFor(t, tm.Output(), hasLine("alpha.go"), teatest.WithDuration(5*time.Second))

	// the S1 anchor for the open @-picker: two rows for the two walked
	// files, at sel 0.
	firstRow, vis := a.mentionDropdownRows()
	if firstRow < 0 || vis != 2 {
		t.Fatalf("@-picker not open (firstRow=%d vis=%d)", firstRow, vis)
	}

	// a click on row 0 (alpha.go) is the enter action: the insert lands in
	// the prompt and the menu closes (the click is NOT the tab expand).
	tm.Send(tea.MouseClickMsg{X: 10, Y: firstRow, Button: tea.MouseLeft})
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	if got := a.prompt.input.Value(); got != "see @alpha.go " {
		t.Fatalf("insert = %q, want the clicked row's @-prefixed insert (see @alpha.go )", got)
	}
	if a.prompt.mentionActive() {
		t.Fatal("the @ menu should be closed after the click (the insert's trailing space kills the trigger)")
	}
}

// TestAcTabComplete pins the S5 tab-complete semantics (spec §3.5, the §3.10
// table): the tab key, matched against the agent_cycle binding (keymap.go:135),
// dispatches on the selected @-picker row — a file → the enter action
// (acInsert, insert + close), a directory → the expand branch (acExpand, the
// shared S3 dir tab-expand, the menu stays open re-filtered to the subtree).
// The closed-tab leg pins that, with the @ menu closed, tab still cycles the
// agent (the agent_cycle path — the intercept is scoped to mentionActive, so
// a closed-tab tab and shift+tab both keep the base group's agent_cycle).
func TestAcTabComplete(t *testing.T) {
	t.Run("tab on a file row: the enter action (insert + close)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		a := testApp()
		a.Service.Dir = dir
		a.prompt.input.SetValue("fix @")
		if !a.prompt.mentionActive() {
			t.Fatal("the @ menu should be open for input \"fix @\"")
		}
		a.handleKey(pressTab())
		if got := a.prompt.input.Value(); got != "fix @alpha.go " {
			t.Fatalf("insert = %q, want the @-prefixed path + a trailing space (fix @alpha.go )", got)
		}
		if a.prompt.mentionActive() {
			t.Fatal("the @ menu should be closed after the tab (the trailing space kills the trigger)")
		}
		if len(a.freq) != 1 || a.freq[0].Path != "alpha.go" {
			t.Fatalf("frecency not recorded (the insert touch): %v", a.freq)
		}
	})
	t.Run("tab on a directory row: the expand (shared S3 code)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cmd", "main.go"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		a := testApp()
		a.Service.Dir = dir
		a.prompt.input.SetValue("fix @")
		// sel 0 is the cmd/ dir row (the walk's preorder: the dir is recorded
		// before its files). tab on it expands, not inserts.
		a.handleKey(pressTab())
		if got := a.prompt.input.Value(); got != "fix @cmd/" {
			t.Fatalf("expand = %q, want @cmd/ (no trailing space)", got)
		}
		if a.prompt.sel != 0 {
			t.Fatalf("sel = %d, want 0 (the expand resets sel)", a.prompt.sel)
		}
		if !a.prompt.mentionActive() {
			t.Fatal("the @ menu should stay open after the dir expand (a valid @-trigger remains)")
		}
		var sawSubtree bool
		for _, o := range a.mentionOptions() {
			mo, _ := o.value.(mentionOption)
			if mo.path == "cmd/main.go" {
				sawSubtree = true
			}
		}
		if !sawSubtree {
			t.Fatal("the re-filter should keep the cmd/ subtree (cmd/main.go)")
		}
		if len(a.freq) != 0 {
			t.Fatalf("the dir-expand branch must not touch the frecency: %v", a.freq)
		}
	})
	t.Run("closed-tab: the agent-cycle path (agent_cycle)", func(t *testing.T) {
		a := testApp()
		a.store.Agents = agentFixture()  // build, plan, yolo (the GET /agent wire order)
		a.prompt.input.SetValue("hello") // no @ trigger: the @ menu is closed
		if a.prompt.mentionActive() {
			t.Fatal("the @ menu should be closed for input \"hello\"")
		}
		a.handleKey(pressTab())
		if got := a.pendingAgent; got != "plan" {
			t.Fatalf("pendingAgent = %q, want plan (the closed-tab agent-cycle, the @ handler does not own it)", got)
		}
		if got := a.prompt.input.Value(); got != "hello" {
			t.Fatalf("input = %q, want hello (tab is not inserted)", got)
		}
	})
	t.Run("the hint flip: 'tab complete' while the @ menu is open, 'tab agents' when closed", func(t *testing.T) {
		a := testApp()
		if got := a.homeHintLine(); got != "tab agents  ctrl+p commands" {
			t.Fatalf("closed hint = %q, want \"tab agents  ctrl+p commands\"", got)
		}
		a.prompt.input.SetValue("@")
		if !a.prompt.mentionActive() {
			t.Fatal("the @ menu should be open for input \"@\"")
		}
		if got := a.homeHintLine(); !strings.HasPrefix(got, "tab complete") {
			t.Fatalf("open (@) hint = %q, want the first segment \"tab complete\"", got)
		}
		// the slash menu open: still "tab complete" (the S6 flip, unchanged).
		a.prompt.input.SetValue("/new")
		if got := a.homeHintLine(); !strings.HasPrefix(got, "tab complete") {
			t.Fatalf("open (slash) hint = %q, want the first segment \"tab complete\"", got)
		}
	})
}

// TestTUIAtPickerTabComplete is the teatest leg for S5 (spec §3.5, the §3.10
// table): tab on a file row of the open @-picker is the ENTER action — the
// selected file inserts (the @-prefixed path + a trailing space) and the menu
// closes. It mirrors TestTUIAtPicker (the enter leg) but drives the key through
// pressTab (the real handleKey intercept, the S6 slash idiom). A directory row
// expands instead — TestAcTabComplete pins that branch (whitebox).
func TestTUIAtPickerTabComplete(t *testing.T) {
	ts := testutil.Boot(t)
	if err := os.WriteFile(filepath.Join(ts.Dir, "alpha.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := client.New(ts.URL, ts.Dir) // scope (and the walk) to the server work dir
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), hasLine(homeLogoLine), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "see @")
	teatest.WaitFor(t, tm.Output(), hasLine("alpha.go"), teatest.WithDuration(5*time.Second))
	tm.Send(pressTab())
	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	if got := a.prompt.input.Value(); got != "see @alpha.go " {
		t.Fatalf("insert = %q, want the tab-completed @-prefixed insert (see @alpha.go )", got)
	}
	if a.prompt.mentionActive() {
		t.Fatal("the @ menu should be closed after the tab (the trailing space kills the trigger)")
	}
}
