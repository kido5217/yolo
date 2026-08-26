package theme

import (
	"strings"
	"testing"
)

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

func resolveOpencodeDark(t *testing.T) Theme {
	t.Helper()
	all, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	return Theme{R: r, Name: "opencode", Mode: "dark"}
}

// TestAllThemesHaveMarkdownSyntaxTokens pins the token matrix: every
// embedded theme × both modes resolves all 23 markdown*/syntax* tokens
// (finding 5: no ThemeJson model change needed).
func TestAllThemesHaveMarkdownSyntaxTokens(t *testing.T) {
	all, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	for name, tj := range all {
		for _, mode := range []string{"dark", "light"} {
			r, err := ResolveTheme(tj, mode)
			if err != nil {
				t.Fatalf("%s/%s: %v", name, mode, err)
			}
			for _, tok := range append(append([]string{}, markdownTokens...), syntaxTokens...) {
				if _, ok := r.Color(tok); !ok {
					t.Errorf("%s/%s: missing token %s", name, mode, tok)
				}
			}
		}
	}
}

// TestStyleConfigMapping pins the markdown* → ansi.StyleConfig field map
// (the opencode.dark goldens; the SGR quantization is pinned by the S1.3
// teatest golden, the 24-bit hex here).
func TestStyleConfigMapping(t *testing.T) {
	cfg := resolveOpencodeDark(t).StyleConfig("markdownText", 77)
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
	cfg := resolveOpencodeDark(t).StyleConfig("textMuted", 77)
	if cfg.Text.Color == nil || *cfg.Text.Color != "#808080" {
		t.Errorf("reasoning base Text.Color = %v, want #808080", cfg.Text.Color)
	}
}

// TestZeroThemeStyleConfigIsNilColors pins the S0.7 zero-Theme contract on
// the markdown path: absent tokens → nil *string → glamour defaults.
func TestZeroThemeStyleConfigIsNilColors(t *testing.T) {
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
	r, err := NewTranscriptRenderer(resolveOpencodeDark(t), 77)
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
