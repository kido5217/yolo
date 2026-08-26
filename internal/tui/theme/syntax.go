// syntax.go — the glamour element styles + TermRenderer factory for the
// transcript. S1.2: the markdown* element styles; S1.4: the chroma token
// map (per-language highlighting) + the global "charm" slot workaround;
// S1.5: the GFM trio (Strikethrough/Task/Table); S1.6: the reasoning
// variant (textMuted base + the pre-blended chroma).

package theme

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
)

// Zero reports whether t is the zero Theme (nil-engine runs, S0.7): the
// transcript render path degrades to the plain wrap.
func (t Theme) Zero() bool { return t.Name == "" }

// hex6 is the 6-digit RGB hex of c. lipgloss v2 parseHex takes #rrggbb or
// #rgb only — 8-digit alpha is unparseable, so subtle (pre-blended) colors
// always land as 6-digit hex (finding 3).
func hex6(c Rgba) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// md returns the color string for a token (nil when absent: glamour falls
// back to its own defaults for unset styles).
func (t Theme) md(name string) *string {
	c, ok := t.R.Color(name)
	if !ok {
		return nil
	}
	s := hex6(c)
	return &s
}

func boolPtr(b bool) *bool { return &b }

// StyleConfig builds the glamour element styles from the markdown* tokens.
// base is the base text token name ("markdownText" for text parts,
// "textMuted" for reasoning); width is the word-wrap width (also the HR
// line length; <4 → the 8-dash fallback). The chroma map is attached in
// S1.4 (CodeBlock.Chroma stays nil until then).
func (t Theme) StyleConfig(base string, width int) ansi.StyleConfig {
	cfg := ansi.StyleConfig{}
	cfg.Document = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color:       t.md("markdownText"),
		BlockPrefix: "\n",
		BlockSuffix: "\n",
	}}
	cfg.Text = ansi.StylePrimitive{Color: t.md(base)}
	cfg.Heading = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color: t.md("markdownHeading"),
		Bold:  boolPtr(true),
	}}
	cfg.BlockQuote = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color: t.md("markdownBlockQuote"),
	}}
	cfg.Emph = ansi.StylePrimitive{Color: t.md("markdownEmph"), Italic: boolPtr(true)}
	cfg.Strong = ansi.StylePrimitive{Color: t.md("markdownStrong"), Bold: boolPtr(true)}
	cfg.HorizontalRule = ansi.StylePrimitive{
		Color:  t.md("markdownHorizontalRule"),
		Format: "\n" + strings.Repeat("─", hrWidth(width)) + "\n",
	}
	cfg.Item = ansi.StylePrimitive{Color: t.md("markdownListItem"), BlockPrefix: "• "}
	cfg.Enumeration = ansi.StylePrimitive{
		Color:       t.md("markdownListEnumeration"),
		BlockPrefix: ". ",
	}
	cfg.Link = ansi.StylePrimitive{Color: t.md("markdownLink"), Underline: boolPtr(true)}
	cfg.LinkText = ansi.StylePrimitive{Color: t.md("markdownLinkText")}
	cfg.Image = ansi.StylePrimitive{
		Color:  t.md("markdownImage"),
		Format: "Image: {{.text}} \u2192",
	}
	cfg.ImageText = ansi.StylePrimitive{Color: t.md("markdownImageText")}
	cfg.Code = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: t.md("markdownCode")}}
	cfg.CodeBlock = ansi.StyleCodeBlock{StyleBlock: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{Color: t.md("markdownCodeBlock")},
	}}
	return cfg
}

// hrWidth is the HorizontalRule line length (the word-wrap width; a
// sub-4-column viewport gets the 8-dash fallback — the upstream element
// renders a full-width top-border box, finding 6).
func hrWidth(width int) int {
	if width < 4 {
		return 8
	}
	return width
}

// Renderer is a glamour TermRenderer bound to one theme + width. The TUI
// renders single-threaded (bubbletea View), so the app builds one per
// renderMessages call — no cache (the construct cost is ~20–50µs; the S1.9
// budget covers the whole re-render).
type Renderer struct {
	tr *glamour.TermRenderer
}

// NewTranscriptRenderer builds the text-part renderer: the markdownText
// base, word-wrap at width (the caller passes w-3 — the post-indent width;
// <=0 disables wrapping). A zero Theme (S0.7) skips WithStyles — all-nil
// element styles, glamour renders plain (no SGR).
func NewTranscriptRenderer(th Theme, width int) (*Renderer, error) {
	var opts []glamour.TermRendererOption
	if !th.Zero() {
		opts = append(opts, glamour.WithStyles(th.StyleConfig("markdownText", width)))
	}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	}
	tr, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return nil, err
	}
	return &Renderer{tr: tr}, nil
}

// Render renders md to an ANSI string. SGR profile (verified, sgrprobe):
// in a plain unit context glamour's plain text is 24-bit (38;2;R;G;B) while
// the chroma code-block path is 256-color (38;5;N); under teatest (the
// TUI's program environment) glamour emits 256-color for both. Teatest
// goldens therefore pin 38;5;N; direct renderer unit tests pin whichever
// profile the path uses.
func (r *Renderer) Render(md string) (string, error) { return r.tr.Render(md) }
