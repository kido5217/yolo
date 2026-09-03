package theme

import (
	"math"

	"charm.land/lipgloss/v2"
)

// Theme is a resolved theme + its name/mode, exposing lipgloss styles —
// components never see hex (spec §3).
type Theme struct {
	R    Resolved
	Name string
	Mode string // "dark" | "light"
}

// Color is the raw-token accessor (test hook + generic consumers).
func (t Theme) Color(name string) (RGBA, bool) { return t.R.Color(name) }

// fg returns a foreground style for the token; an absent token or a
// transparent (alpha 0) token yields an empty style (no foreground).
func (t Theme) fg(token string) lipgloss.Style {
	c, ok := t.R.Color(token)
	if !ok || c.A == 0 {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()[:7]))
}

// bg returns a background style for the token; absent/transparent → no
// background (alpha semantics, see the Task interface note).
func (t Theme) bg(token string) lipgloss.Style {
	c, ok := t.R.Color(token)
	if !ok || c.A == 0 {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(c.Hex()[:7]))
}

// WarningSubtle is warning pre-blended over the background at
// ThinkingOpacity (upstream RGBA.fromValues(warning.r, g, b,
// thinkingOpacity), index.tsx:1660) — the lipgloss hex takes 6 digits,
// so the alpha is pre-applied (finding 3).
func (t Theme) WarningSubtle() lipgloss.Style {
	return t.blended("warning")
}

// blended returns fg(token) with the color pre-blended over the
// background at ThinkingOpacity (identity when the token or background
// is absent or α is out of (0,1)).
func (t Theme) blended(token string) lipgloss.Style {
	c, ok := t.R.Color(token)
	if !ok {
		return lipgloss.NewStyle()
	}
	a := t.R.ThinkingOpacity
	if a <= 0 || a >= 1 {
		return t.fg(token)
	}
	bg := RGBA{0, 0, 0, 255}
	if bc, ok := t.R.Color("background"); ok {
		bg = bc
	}
	out := RGBA{
		R: uint8(math.Round(float64(c.R)*a + float64(bg.R)*(1-a))),
		G: uint8(math.Round(float64(c.G)*a + float64(bg.G)*(1-a))),
		B: uint8(math.Round(float64(c.B)*a + float64(bg.B)*(1-a))),
		A: 255,
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex6(out)))
}

// SelectedForeground is the port of upstream selectedForeground
// (theme/index.ts:95-111): explicit selectedListItemText wins; transparent
// background → contrast against bg (or primary) via the luminance rule
// (0.299r+0.587g+0.114b > 0.5 → black, else white); else background.
func (t Theme) SelectedForeground(bg ...RGBA) RGBA {
	if t.R.HasSelectedListItemText {
		c, _ := t.R.Color("selectedListItemText")
		return c
	}
	background, _ := t.R.Color("background")
	if background.A == 0 {
		target := background
		if len(bg) > 0 {
			target = bg[0]
		} else if c, ok := t.R.Color("primary"); ok {
			target = c
		}
		lum := 0.299*float64(target.R) + 0.587*float64(target.G) + 0.114*float64(target.B)
		if lum > 0.5*255 {
			return RGBA{0, 0, 0, 255}
		}
		return RGBA{255, 255, 255, 255}
	}
	return background
}

func (t Theme) Text() lipgloss.Style               { return t.fg("text") }
func (t Theme) TextMuted() lipgloss.Style          { return t.fg("textMuted") }
func (t Theme) Primary() lipgloss.Style            { return t.fg("primary") }
func (t Theme) Secondary() lipgloss.Style          { return t.fg("secondary") }
func (t Theme) Accent() lipgloss.Style             { return t.fg("accent") }
func (t Theme) Error() lipgloss.Style              { return t.fg("error") }
func (t Theme) Warning() lipgloss.Style            { return t.fg("warning") }
func (t Theme) Success() lipgloss.Style            { return t.fg("success") }
func (t Theme) Info() lipgloss.Style               { return t.fg("info") }
func (t Theme) Border() lipgloss.Style             { return t.fg("border") }
func (t Theme) BorderActive() lipgloss.Style       { return t.fg("borderActive") }
func (t Theme) BorderSubtle() lipgloss.Style       { return t.fg("borderSubtle") }
func (t Theme) Background() lipgloss.Style         { return t.bg("background") }
func (t Theme) BackgroundPanel() lipgloss.Style    { return t.bg("backgroundPanel") }
func (t Theme) BackgroundElement() lipgloss.Style  { return t.bg("backgroundElement") }
func (t Theme) BackgroundMenu() lipgloss.Style     { return t.bg("backgroundMenu") }
func (t Theme) MarkdownText() lipgloss.Style       { return t.fg("markdownText") }
func (t Theme) MarkdownHeading() lipgloss.Style    { return t.fg("markdownHeading") }
func (t Theme) MarkdownLink() lipgloss.Style       { return t.fg("markdownLink") }
func (t Theme) MarkdownLinkText() lipgloss.Style   { return t.fg("markdownLinkText") }
func (t Theme) MarkdownCode() lipgloss.Style       { return t.fg("markdownCode") }
func (t Theme) MarkdownBlockQuote() lipgloss.Style { return t.fg("markdownBlockQuote") }
func (t Theme) MarkdownEmph() lipgloss.Style       { return t.fg("markdownEmph") }
func (t Theme) MarkdownStrong() lipgloss.Style     { return t.fg("markdownStrong") }
func (t Theme) MarkdownHorizontalRule() lipgloss.Style {
	return t.fg("markdownHorizontalRule")
}
func (t Theme) MarkdownListItem() lipgloss.Style        { return t.fg("markdownListItem") }
func (t Theme) MarkdownListEnumeration() lipgloss.Style { return t.fg("markdownListEnumeration") }
func (t Theme) MarkdownImage() lipgloss.Style           { return t.fg("markdownImage") }
func (t Theme) MarkdownImageText() lipgloss.Style       { return t.fg("markdownImageText") }
func (t Theme) MarkdownCodeBlock() lipgloss.Style       { return t.fg("markdownCodeBlock") }
func (t Theme) SyntaxComment() lipgloss.Style           { return t.fg("syntaxComment") }
func (t Theme) SyntaxKeyword() lipgloss.Style           { return t.fg("syntaxKeyword") }
func (t Theme) SyntaxFunction() lipgloss.Style          { return t.fg("syntaxFunction") }
func (t Theme) SyntaxVariable() lipgloss.Style          { return t.fg("syntaxVariable") }
func (t Theme) SyntaxString() lipgloss.Style            { return t.fg("syntaxString") }
func (t Theme) SyntaxNumber() lipgloss.Style            { return t.fg("syntaxNumber") }
func (t Theme) SyntaxType() lipgloss.Style              { return t.fg("syntaxType") }
func (t Theme) SyntaxOperator() lipgloss.Style          { return t.fg("syntaxOperator") }
func (t Theme) SyntaxPunctuation() lipgloss.Style       { return t.fg("syntaxPunctuation") }
func (t Theme) DiffAdded() lipgloss.Style               { return t.fg("diffAdded") }
func (t Theme) DiffRemoved() lipgloss.Style             { return t.fg("diffRemoved") }
func (t Theme) DiffContext() lipgloss.Style             { return t.fg("diffContext") }
func (t Theme) DiffHunkHeader() lipgloss.Style          { return t.fg("diffHunkHeader") }
func (t Theme) DiffHighlightAdded() lipgloss.Style      { return t.fg("diffHighlightAdded") }
func (t Theme) DiffHighlightRemoved() lipgloss.Style    { return t.fg("diffHighlightRemoved") }
func (t Theme) DiffAddedBg() lipgloss.Style             { return t.bg("diffAddedBg") }
func (t Theme) DiffRemovedBg() lipgloss.Style           { return t.bg("diffRemovedBg") }
func (t Theme) DiffContextBg() lipgloss.Style           { return t.bg("diffContextBg") }
func (t Theme) DiffLineNumber() lipgloss.Style          { return t.fg("diffLineNumber") }
func (t Theme) DiffAddedLineNumberBg() lipgloss.Style   { return t.bg("diffAddedLineNumberBg") }
func (t Theme) DiffRemovedLineNumberBg() lipgloss.Style { return t.bg("diffRemovedLineNumberBg") }
