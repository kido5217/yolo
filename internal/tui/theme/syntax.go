// syntax.go — the glamour element styles + TermRenderer factory for the
// transcript. S1.2: the markdown* element styles; S1.4: the chroma token
// map (per-language highlighting) + the global "charm" slot workaround;
// S1.5: the GFM trio (Strikethrough/Task/Table); S1.6: the reasoning
// variant (textMuted base + the pre-blended chroma).

package theme

import (
	"fmt"
	"math"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"github.com/alecthomas/chroma/v2/styles"
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

// Chroma builds the full syntax token map (upstream getSyntaxRules,
// theme/index.ts:586-760 — the scope table in the S1.4 plan notes).
func (t Theme) Chroma() ansi.Chroma {
	c := ansi.Chroma{}
	c.Text = ansi.StylePrimitive{Color: t.md("text")}
	c.Comment = ansi.StylePrimitive{Color: t.md("syntaxComment"), Italic: boolPtr(true)}
	c.CommentPreproc = ansi.StylePrimitive{Color: t.md("syntaxComment"), Italic: boolPtr(true)}
	c.Keyword = ansi.StylePrimitive{Color: t.md("syntaxKeyword"), Italic: boolPtr(true)}
	c.KeywordReserved = ansi.StylePrimitive{Color: t.md("syntaxKeyword"), Italic: boolPtr(true)}
	c.KeywordNamespace = ansi.StylePrimitive{Color: t.md("syntaxKeyword")}
	c.KeywordType = ansi.StylePrimitive{Color: t.md("syntaxType"), Bold: boolPtr(true), Italic: boolPtr(true)}
	c.Operator = ansi.StylePrimitive{Color: t.md("syntaxOperator")}
	c.Punctuation = ansi.StylePrimitive{Color: t.md("syntaxPunctuation")}
	c.Name = ansi.StylePrimitive{Color: t.md("syntaxVariable")}
	c.NameBuiltin = ansi.StylePrimitive{Color: t.md("error")}
	c.NameAttribute = ansi.StylePrimitive{Color: t.md("syntaxVariable")}
	c.NameClass = ansi.StylePrimitive{Color: t.md("syntaxType")}
	c.NameConstant = ansi.StylePrimitive{Color: t.md("syntaxNumber")}
	c.NameFunction = ansi.StylePrimitive{Color: t.md("syntaxFunction")}
	c.LiteralNumber = ansi.StylePrimitive{Color: t.md("syntaxNumber")}
	c.LiteralString = ansi.StylePrimitive{Color: t.md("syntaxString")}
	c.LiteralStringEscape = ansi.StylePrimitive{Color: t.md("syntaxString")}
	return c
}

// SubtleChroma is the reasoning variant (upstream generateSubtleSyntax,
// theme/index.ts:560-584: RGB kept, alpha set to ThinkingOpacity. SGR
// 24-bit carries no alpha and lipgloss v2 parseHex takes 6-digit hex only,
// so each foreground is PRE-BLENDED over the theme background:
// out = round(fg*α + bg*(1-α)), half-up per channel; absent background →
// #000000). It blends the TOKEN colors directly (the token name is the
// source of truth).
func (t Theme) SubtleChroma() ansi.Chroma {
	full := t.Chroma()
	alpha := t.R.ThinkingOpacity
	if alpha <= 0 || alpha >= 1 {
		return full
	}
	bg := Rgba{0, 0, 0, 255}
	if c, ok := t.R.Color("background"); ok {
		bg = c
	}
	// pairs is the (field pointer, token) set — exactly the fields
	// Chroma() sets.
	type pair struct {
		p   *ansi.StylePrimitive
		tok string
	}
	pairs := []pair{
		{&full.Text, "text"}, {&full.Comment, "syntaxComment"},
		{&full.CommentPreproc, "syntaxComment"}, {&full.Keyword, "syntaxKeyword"},
		{&full.KeywordReserved, "syntaxKeyword"}, {&full.KeywordNamespace, "syntaxKeyword"},
		{&full.KeywordType, "syntaxType"}, {&full.Operator, "syntaxOperator"},
		{&full.Punctuation, "syntaxPunctuation"}, {&full.Name, "syntaxVariable"},
		{&full.NameBuiltin, "error"}, {&full.NameAttribute, "syntaxVariable"},
		{&full.NameClass, "syntaxType"}, {&full.NameConstant, "syntaxNumber"},
		{&full.NameFunction, "syntaxFunction"}, {&full.LiteralNumber, "syntaxNumber"},
		{&full.LiteralString, "syntaxString"}, {&full.LiteralStringEscape, "syntaxString"},
	}
	for _, pr := range pairs {
		if pr.p.Color == nil {
			continue
		}
		fg, ok := t.R.Color(pr.tok)
		if !ok {
			continue
		}
		out := Rgba{
			R: uint8(math.Round(float64(fg.R)*alpha + float64(bg.R)*(1-alpha))),
			G: uint8(math.Round(float64(fg.G)*alpha + float64(bg.G)*(1-alpha))),
			B: uint8(math.Round(float64(fg.B)*alpha + float64(bg.B)*(1-alpha))),
			A: 255,
		}
		s := hex6(out)
		pr.p.Color = &s
	}
	return full
}

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

// Renderer is a glamour TermRenderer bound to one theme + width. The
// chroma field is the map this renderer registered under the GLOBAL
// "charm" slot (finding 2): Render deletes the slot first, so this
// renderer's map (re-)registers on the next code block — transcript (full)
// and reasoning (subtle) renderers + SIGUSR2 theme switches never
// cross-color. The TUI renders single-threaded (bubbletea View), so
// sequential re-registration is safe.
type Renderer struct {
	tr     *glamour.TermRenderer
	chroma *ansi.Chroma
}

// newRenderer builds a TermRenderer from a base token + a chroma map,
// word-wrapped at width (<=0 disables wrapping). A zero Theme (S0.7)
// skips BOTH WithStyles and the chroma attach: StyleConfig always sets
// the attribute pointers even with nil colors, so attaching either
// would emit attribute-only SGR and squat the global "charm" slot —
// the chroma pointer stays nil and Render skips the slot delete
// (deviation 149 + 152).
func newRenderer(th Theme, width int, base string, ch ansi.Chroma) (*Renderer, error) {
	var opts []glamour.TermRendererOption
	var chroma *ansi.Chroma
	if !th.Zero() {
		cfg := th.StyleConfig(base, width)
		chroma = &ch
		cfg.CodeBlock.Chroma = chroma
		opts = append(opts, glamour.WithStyles(cfg))
	}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	}
	tr, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return nil, err
	}
	return &Renderer{tr: tr, chroma: chroma}, nil
}

// NewTranscriptRenderer builds the text-part renderer: the markdownText
// base + the FULL chroma map, word-wrap at width (the caller passes w-3 —
// the post-indent width; <=0 disables wrapping).
func NewTranscriptRenderer(th Theme, width int) (*Renderer, error) {
	return newRenderer(th, width, "markdownText", th.Chroma())
}

// NewReasoningRenderer builds the expanded-reasoning renderer: the
// textMuted base + the pre-blended chroma map (upstream
// generateSubtleSyntax, theme/index.ts:560-584).
func NewReasoningRenderer(th Theme, width int) (*Renderer, error) {
	return newRenderer(th, width, "textMuted", th.SubtleChroma())
}

// Render renders md to an ANSI string. It clears the global "charm" chroma
// slot first (finding 2) so THIS renderer's map (re-)registers. SGR
// profile (verified, sgrprobe): in a plain unit context glamour's plain
// text is 24-bit (38;2;R;G;B) while the chroma code-block path is
// 256-color (38;5;N); under teatest (the TUI's program environment)
// glamour emits 256-color for both. Teatest goldens therefore pin
// 38;5;N; direct renderer unit tests pin whichever profile the path uses.
func (r *Renderer) Render(md string) (string, error) {
	if r.chroma != nil {
		delete(styles.Registry, "charm")
	}
	return r.tr.Render(md)
}
