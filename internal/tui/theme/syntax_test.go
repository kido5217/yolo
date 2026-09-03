package theme

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/glamour/v2/ansi"
)

// sgrRe strips SGR (color/attribute) escape sequences. Used only for the
// TaskElement contiguity pin in TestGFMRender (deviation 154).
var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// markdownTokens + syntaxTokens are the flat upstream theme keys
// (theme/index.ts; every one of the 33 embedded assets carries them).
var (
	markdownTokens = []string{
		"markdownText", "markdownHeading", "markdownCode", "markdownCodeBlock",
		"markdownBlockQuote", "markdownEmph", "markdownStrong",
		"markdownHorizontalRule", "markdownListItem", "markdownListEnumeration",
		"markdownLink", "markdownLinkText", "markdownImage", "markdownImageText",
	}
	syntaxTokens = []string{
		"syntaxComment", "syntaxKeyword", "syntaxFunction", "syntaxVariable",
		"syntaxString", "syntaxNumber", "syntaxType", "syntaxOperator",
		"syntaxPunctuation",
	}
)

func resolveYoloDark(t *testing.T) Theme {
	t.Helper()
	all, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := ResolveTheme(all["yolo"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	return Theme{R: r, Name: "yolo", Mode: "dark"}
}

// TestAllThemesHaveMarkdownSyntaxTokens pins the token matrix: every
// embedded theme × both modes resolves all 23 markdown*/syntax* tokens
// (finding 5: no ThemeJSON model change needed).
func TestAllThemesHaveMarkdownSyntaxTokens(t *testing.T) {
	all, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	for name, tj := range all {
		for _, mode := range []string{"dark", "light"} {
			t.Run(name+"-"+mode, func(t *testing.T) {
				t.Parallel()
				r, err := ResolveTheme(tj, mode)
				if err != nil {
					t.Fatalf("%s/%s: %v", name, mode, err)
				}
				for _, tok := range append(append([]string{}, markdownTokens...), syntaxTokens...) {
					if _, ok := r.Color(tok); !ok {
						t.Errorf("%s/%s: missing token %s", name, mode, tok)
					}
				}
			})
		}
	}
}

// TestStyleConfigMapping pins the markdown* → ansi.StyleConfig field map
// (the yolo.dark goldens; the SGR quantization is pinned by the S1.3
// teatest golden, the 24-bit hex here).
func TestStyleConfigMapping(t *testing.T) {
	t.Parallel()
	cfg := resolveYoloDark(t).StyleConfig("markdownText", 77)
	check := func(name string, got *string, want string) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("%s = %v, want %s", name, got, want)
		}
	}
	check("Document.Color", cfg.Document.Color, "#eeeeee")
	if cfg.Document.BlockPrefix != "\n" || cfg.Document.BlockSuffix != "\n" {
		t.Errorf("Document block prefix/suffix = %q/%q, want \\n/\\n",
			cfg.Document.BlockPrefix, cfg.Document.BlockSuffix)
	}
	check("Text.Color", cfg.Text.Color, "#eeeeee")
	check("Heading.Color", cfg.Heading.Color, "#9d7cd8")
	if cfg.Heading.Bold == nil || !*cfg.Heading.Bold {
		t.Error("Heading.Bold = false/nil, want true")
	}
	check("BlockQuote.Color", cfg.BlockQuote.Color, "#e5c07b")
	check("Emph.Color", cfg.Emph.Color, "#e5c07b")
	if cfg.Emph.Italic == nil || !*cfg.Emph.Italic {
		t.Error("Emph.Italic = false/nil, want true")
	}
	check("Strong.Color", cfg.Strong.Color, "#f5a742")
	if cfg.Strong.Bold == nil || !*cfg.Strong.Bold {
		t.Error("Strong.Bold = false/nil, want true")
	}
	check("HorizontalRule.Color", cfg.HorizontalRule.Color, "#808080")
	if want := "\n" + strings.Repeat("─", 77) + "\n"; cfg.HorizontalRule.Format != want {
		t.Errorf("HorizontalRule.Format = %q (len %d), want a 77-dash line",
			cfg.HorizontalRule.Format, len(cfg.HorizontalRule.Format))
	}
	check("Item.Color", cfg.Item.Color, "#fab283")
	if cfg.Item.BlockPrefix != "• " {
		t.Errorf("Item.BlockPrefix = %q, want '• '", cfg.Item.BlockPrefix)
	}
	check("Enumeration.Color", cfg.Enumeration.Color, "#56b6c2")
	if cfg.Enumeration.BlockPrefix != ". " {
		t.Errorf("Enumeration.BlockPrefix = %q, want '. '", cfg.Enumeration.BlockPrefix)
	}
	check("Link.Color", cfg.Link.Color, "#fab283")
	if cfg.Link.Underline == nil || !*cfg.Link.Underline {
		t.Error("Link.Underline = false/nil, want true")
	}
	check("LinkText.Color", cfg.LinkText.Color, "#56b6c2")
	check("Image.Color", cfg.Image.Color, "#fab283")
	check("ImageText.Color", cfg.ImageText.Color, "#56b6c2")
	check("Code.Color", cfg.Code.Color, "#7fd88f")
	check("CodeBlock.Color", cfg.CodeBlock.Color, "#eeeeee")
	if cfg.CodeBlock.Chroma != nil {
		t.Error("CodeBlock.Chroma set before S1.4")
	}
}

// TestStyleConfigReasoningBase pins the reasoning base token (S1.6 consumes
// it): the Text style takes the base TOKEN NAME, not a hard-coded color.
func TestStyleConfigReasoningBase(t *testing.T) {
	t.Parallel()
	cfg := resolveYoloDark(t).StyleConfig("textMuted", 77)
	if cfg.Text.Color == nil || *cfg.Text.Color != "#808080" {
		t.Errorf("reasoning base Text.Color = %v, want #808080", cfg.Text.Color)
	}
}

// TestZeroThemeStyleConfigIsNilColors pins the S0.7 zero-Theme contract on
// the markdown path: absent tokens → nil *string → glamour defaults.
func TestZeroThemeStyleConfigIsNilColors(t *testing.T) {
	t.Parallel()
	var th Theme
	cfg := th.StyleConfig("markdownText", 77)
	if cfg.Text.Color != nil || cfg.Heading.Color != nil {
		t.Error("zero Theme must yield nil *string colors")
	}
}

// TestTranscriptRendererRenders pins the factory: a themed renderer emits
// SGR, a zero-Theme renderer degrades to plain output (no SGR). The exact
// 38;5;N parameters are pinned by the S1.3 teatest golden (xterm-256color).
func TestTranscriptRendererRenders(t *testing.T) {
	r, err := NewTranscriptRenderer(resolveYoloDark(t), 77)
	if err != nil {
		t.Fatalf("NewTranscriptRenderer: %v", err)
	}
	out, err := r.Render("hello **world**")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "world") {
		t.Errorf("output missing text: %q", out)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("themed renderer emitted no SGR escapes")
	}
	zr, err := NewTranscriptRenderer(Theme{}, 77)
	if err != nil {
		t.Fatalf("NewTranscriptRenderer(zero): %v", err)
	}
	zout, err := zr.Render("hello **world**")
	if err != nil {
		t.Fatalf("Render(zero): %v", err)
	}
	if strings.Contains(zout, "\x1b[") {
		t.Errorf("zero-Theme renderer emitted SGR: %q", zout)
	}
}

// TestChromaMapping pins the syntax* → ansi.Chroma field map (finding: the
// upstream getSyntaxRules scope table; yolo.dark hexes).
func TestChromaMapping(t *testing.T) {
	t.Parallel()
	ch := resolveYoloDark(t).Chroma()
	check := func(name string, p ansi.StylePrimitive, want string) {
		t.Helper()
		if p.Color == nil || *p.Color != want {
			t.Errorf("%s = %v, want %s", name, p.Color, want)
		}
	}
	check("Text", ch.Text, "#eeeeee")
	check("Comment", ch.Comment, "#808080")
	if ch.Comment.Italic == nil || !*ch.Comment.Italic {
		t.Error("Comment.Italic = false/nil, want true")
	}
	check("Keyword", ch.Keyword, "#9d7cd8")
	if ch.Keyword.Italic == nil || !*ch.Keyword.Italic {
		t.Error("Keyword.Italic = false/nil, want true")
	}
	check("KeywordNamespace", ch.KeywordNamespace, "#9d7cd8")
	if ch.KeywordNamespace.Italic != nil {
		t.Error("KeywordNamespace.Italic set, want nil (upstream keyword.import has no italic)")
	}
	check("KeywordType", ch.KeywordType, "#e5c07b")
	if ch.KeywordType.Bold == nil || !*ch.KeywordType.Bold || ch.KeywordType.Italic == nil || !*ch.KeywordType.Italic {
		t.Error("KeywordType = bold+italic, got", ch.KeywordType.Bold, ch.KeywordType.Italic)
	}
	check("Operator", ch.Operator, "#56b6c2")
	check("Punctuation", ch.Punctuation, "#eeeeee")
	check("Name", ch.Name, "#e06c75")
	check("NameBuiltin", ch.NameBuiltin, "#e06c75")
	check("NameAttribute", ch.NameAttribute, "#e06c75")
	check("NameClass", ch.NameClass, "#e5c07b")
	check("NameConstant", ch.NameConstant, "#f5a742")
	check("NameFunction", ch.NameFunction, "#fab283")
	check("LiteralNumber", ch.LiteralNumber, "#f5a742")
	check("LiteralString", ch.LiteralString, "#7fd88f")
	check("LiteralStringEscape", ch.LiteralStringEscape, "#7fd88f")
	if ch.NameTag.Color != nil || ch.NameDecorator.Color != nil {
		t.Error("NameTag/NameDecorator must stay zero (no upstream scope)")
	}
}

// TestSubtleChroma pins the pre-blend (finding 3): fg = round(fg*α +
// bg*(1-α)) over the theme background, α = ThinkingOpacity (0.6 for
// yolo dark; bg #0a0a0a).
func TestSubtleChroma(t *testing.T) {
	t.Parallel()
	th := resolveYoloDark(t)
	if th.R.ThinkingOpacity != 0.6 {
		t.Fatalf("ThinkingOpacity = %v, want 0.6", th.R.ThinkingOpacity)
	}
	sub := th.SubtleChroma()
	full := th.Chroma()
	check := func(name string, got ansi.StylePrimitive, want string) {
		t.Helper()
		if got.Color == nil || *got.Color != want {
			t.Errorf("%s = %v, want %s", name, got.Color, want)
		}
	}
	check("Comment", sub.Comment, "#515151")             // #808080 @0.6 over #0a0a0a
	check("Keyword", sub.Keyword, "#624e86")             // #9d7cd8
	check("LiteralString", sub.LiteralString, "#50865a") // #7fd88f
	check("LiteralNumber", sub.LiteralNumber, "#97682c") // #f5a742
	check("Operator", sub.Operator, "#387178")           // #56b6c2
	// attributes survive the blend (only the foreground changes upstream)
	if sub.Keyword.Italic == nil || !*sub.Keyword.Italic {
		t.Error("subtle Keyword lost its Italic")
	}
	if *sub.Comment.Color == *full.Comment.Color {
		t.Error("subtle map identical to full — the blend did nothing")
	}
}

// TestChromaSlotWorkaround pins the finding-2 contract: two renderers with
// different chroma maps, rendered in EITHER order, each get their own
// colors (the global "charm" slot is deleted before every Render).
func TestChromaSlotWorkaround(t *testing.T) {
	th := resolveYoloDark(t)
	full, err := NewTranscriptRenderer(th, 77)
	if err != nil {
		t.Fatalf("full renderer: %v", err)
	}
	sub, err := NewReasoningRenderer(th, 77)
	if err != nil {
		t.Fatalf("subtle renderer: %v", err)
	}
	const md = "\n```go\nvar x = 1\n```\n"
	// The keyword token ("var") is color+italic. CHROMA code blocks render
	// through chroma's own terminal formatter (quick.Highlight), which
	// quantizes to 256-COLOR SGR even in a unit context — glamour's plain
	// text stays 24-bit, but the highlighted code is 38;5;N (verified:
	// full keyword #9d7cd8 -> 140, subtle pre-blended #624e86 -> 60).
	// Pin the 256 index as a substring.
	for _, order := range []struct {
		name string
		r    *Renderer
		want string
	}{
		{"full", full, "38;5;140"},
		{"subtle", sub, "38;5;60"},
	} {
		out, err := order.r.Render(md)
		if err != nil {
			t.Fatalf("Render(%s): %v", order.name, err)
		}
		if !strings.Contains(out, order.want) {
			t.Errorf("Render(%s) missing %q in: %q", order.name, order.want, out)
		}
	}
	// order matters in BOTH directions: render subtle first, then full —
	// the full renderer must still emit its OWN keyword (the slot delete
	// re-registers on the next code block; without it the full renderer
	// leaks the subtle 38;5;60, verified in the detail pass).
	if _, err := sub.Render(md); err != nil {
		t.Fatalf("Render(subtle, first): %v", err)
	}
	out, err := full.Render(md)
	if err != nil {
		t.Fatalf("Render(full, again): %v", err)
	}
	if !strings.Contains(out, "38;5;140") {
		t.Errorf("full renderer cross-colored by the subtle render: %q", out)
	}
}

// TestStyleConfigGFM pins the S1.5 GFM trio (yolo.dark).
func TestStyleConfigGFM(t *testing.T) {
	t.Parallel()
	cfg := resolveYoloDark(t).StyleConfig("markdownText", 77)
	if cfg.Strikethrough.CrossedOut == nil || !*cfg.Strikethrough.CrossedOut {
		t.Error("Strikethrough.CrossedOut = false/nil, want true")
	}
	if cfg.Strikethrough.Color != nil {
		t.Error("Strikethrough.Color set, want nil (no upstream token)")
	}
	if cfg.Task.Ticked != "• " || cfg.Task.Unticked != "• " {
		t.Errorf("Task ticks = %q/%q, want '• '/'• ' (upstream hides the checkbox)",
			cfg.Task.Ticked, cfg.Task.Unticked)
	}
	if cfg.Task.Color == nil || *cfg.Task.Color != "#fab283" {
		t.Errorf("Task.Color = %v, want #fab283 (markdownListItem)", cfg.Task.Color)
	}
}

// TestGFMRender pins the three GFM features end-to-end (theme yolo
// dark; the 24-bit SGR is asserted directly — the 38;5;N quantization is
// the teatest layer's job). Verified against glamour v2.0.1 behavior:
// the Task element's StylePrimitive styles only the checkbox (the item
// TEXT renders in the base Text color — upstream parity: opentui's list
// items carry the markdown base fg); the table grid is the │ / ┼ / ─
// column layout (no corner glyphs); the strikethrough run resets to
// default first, so SGR 9 is standalone. v2.0.1 emits the TaskElement tick
// prefix and the item text as SEPARATE SGR runs with a reset between them
// (inter-run reset, ansi/task.go + baseelement.go:
// \x1b[38;2;238;238;238m• \x1b[m\x1b[38;2;238;238;238mdone), so the
// "bullet immediately precedes its item's text" pin runs on an
// SGR-stripped copy; every other assertion stays on the raw output
// (deviation 154).
func TestGFMRender(t *testing.T) {
	r, err := NewTranscriptRenderer(resolveYoloDark(t), 77)
	if err != nil {
		t.Fatalf("NewTranscriptRenderer: %v", err)
	}
	// 1) table: the glamour grid column borders (│ separator, ┼ join).
	out, err := r.Render("| a | b |\n|---|---|\n| 1 | 2 |\n")
	if err != nil {
		t.Fatalf("Render(table): %v", err)
	}
	for _, want := range []string{"\u2502", "\u253C"} { // │ ┼
		if !strings.Contains(out, want) {
			t.Errorf("table missing border %q in %q", want, out)
		}
	}
	// 2) task list: hidden checkbox, "• " bullets, the item text in the
	// base text color (38;2;238;238;238 = markdownText #eeeeee).
	out, err = r.Render("- [x] done\n- [ ] todo\n")
	if err != nil {
		t.Fatalf("Render(task): %v", err)
	}
	// Contiguity on the SGR-stripped copy only: the tick prefix and the
	// item text are separate SGR runs with an inter-run reset (deviation 154).
	stripped := sgrRe.ReplaceAllString(out, "")
	if !strings.Contains(stripped, "\u2022 done") || !strings.Contains(stripped, "\u2022 todo") {
		t.Errorf("task list = %q, want '• done' / '• todo' (stripped)", stripped)
	}
	if strings.Contains(out, "[x]") || strings.Contains(out, "[ ]") {
		t.Errorf("checkbox visible: %q", out)
	}
	if !strings.Contains(out, "38;2;238;238;238") { // base text color
		t.Errorf("task item missing the base text color: %q", out)
	}
	// 3) strikethrough: SGR 9 (crossed-out), standalone after a reset
	// (\x1b[9m) — or merged, if glamour ever changes the pen handling.
	out, err = r.Render("a ~~gone~~ word\n")
	if err != nil {
		t.Fatalf("Render(strike): %v", err)
	}
	if !strings.Contains(out, "\x1b[9m") && !strings.Contains(out, ";9m") {
		t.Errorf("strikethrough missing SGR 9: %q", out)
	}
	if !strings.Contains(out, "gone") {
		t.Errorf("strikethrough lost its text: %q", out)
	}
}
