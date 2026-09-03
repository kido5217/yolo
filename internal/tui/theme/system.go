package theme

import (
	"math"
)

// TerminalColors is the palette result (port of @opentui/core
// TerminalColors): the 16-color palette + default fg/bg; "" = unknown.
type TerminalColors struct {
	Palette           [16]string
	DefaultForeground string
	DefaultBackground string
}

// Tint is the port of upstream tint (theme/index.ts:346): overlay blended
// onto base with alpha, in the upstream FLOAT 0-1 operation order.
func Tint(base, overlay Rgba, alpha float64) Rgba {
	r := float64(base.R)/255 + (float64(overlay.R)/255-float64(base.R)/255)*alpha
	g := float64(base.G)/255 + (float64(overlay.G)/255-float64(base.G)/255)*alpha
	b := float64(base.B)/255 + (float64(overlay.B)/255-float64(base.B)/255)*alpha
	return Rgba{uint8(math.Round(r * 255)), uint8(math.Round(g * 255)), uint8(math.Round(b * 255)), 255}
}

// generateGrayScale is the port of upstream generateGrayScale
// (theme/index.ts:471-523): 12 steps derived from the background luminance,
// branch on luminance < 10 (dark) / > 245 (light). The non-branch sides
// divide by luminance: a pure-black background in light mode (luminance 0)
// yields NaN, which converts to 0 grays — upstream parity, deliberately
// not guarded (the branch pins are the S0 golden).
func generateGrayScale(bg Rgba, isDark bool) [13]Rgba {
	var grays [13]Rgba
	luminance := 0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)
	for i := 1; i <= 12; i++ {
		factor := float64(i) / 12.0
		var newR, newG, newB float64
		if isDark {
			if luminance < 10 {
				grayValue := math.Floor(factor * 0.4 * 255)
				newR, newG, newB = grayValue, grayValue, grayValue
			} else {
				newLum := luminance + (255-luminance)*factor*0.4
				ratio := newLum / luminance
				newR = math.Min(float64(bg.R)*ratio, 255)
				newG = math.Min(float64(bg.G)*ratio, 255)
				newB = math.Min(float64(bg.B)*ratio, 255)
			}
		} else {
			if luminance > 245 {
				grayValue := math.Floor(255 - factor*0.4*255)
				newR, newG, newB = grayValue, grayValue, grayValue
			} else {
				newLum := luminance * (1 - factor*0.4)
				ratio := newLum / luminance
				newR = math.Max(float64(bg.R)*ratio, 0)
				newG = math.Max(float64(bg.G)*ratio, 0)
				newB = math.Max(float64(bg.B)*ratio, 0)
			}
		}
		grays[i] = Rgba{uint8(math.Floor(newR)), uint8(math.Floor(newG)), uint8(math.Floor(newB)), 255}
	}
	return grays
}

// generateMutedTextColor is the port of upstream generateMutedTextColor
// (theme/index.ts:525-554).
func generateMutedTextColor(bg Rgba, isDark bool) Rgba {
	bgLum := 0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)
	var grayValue float64
	if isDark {
		if bgLum < 10 {
			grayValue = 180
		} else {
			grayValue = math.Min(math.Floor(160+bgLum*0.3), 200)
		}
	} else {
		if bgLum > 245 {
			grayValue = 75
		} else {
			grayValue = math.Max(math.Floor(100-(255-bgLum)*0.2), 60)
		}
	}
	g := uint8(grayValue)
	return Rgba{g, g, g, 255}
}

// GenerateSystem is the port of upstream generateSystem
// (theme/index.ts:360-469): terminal palette + default fg/bg → generated
// ThemeJSON. Theme values are Rgba (ResolveTheme's Rgba branch), mirroring
// upstream's RGBA-instance values; missing palette entries fall back to the
// ANSI table, missing default bg/fg to palette[0]/palette[7].
func GenerateSystem(colors TerminalColors, mode string) ThemeJSON {
	isDark := mode == "dark"
	col := func(i int) Rgba {
		if colors.Palette[i] != "" {
			return FromHex(colors.Palette[i])
		}
		return AnsiToRgba(i)
	}
	bg := col(0)
	if colors.DefaultBackground != "" {
		bg = FromHex(colors.DefaultBackground)
	}
	fg := col(7)
	if colors.DefaultForeground != "" {
		fg = FromHex(colors.DefaultForeground)
	}
	transparent := Rgba{bg.R, bg.G, bg.B, 0}
	grays := generateGrayScale(bg, isDark)
	textMuted := generateMutedTextColor(bg, isDark)
	ansiColors := map[string]Rgba{
		"black": col(0), "red": col(1), "green": col(2), "yellow": col(3),
		"blue": col(4), "magenta": col(5), "cyan": col(6), "white": col(7),
		"redBright": col(9), "greenBright": col(10),
	}
	diffAlpha := 0.14
	if isDark {
		diffAlpha = 0.22
	}
	diffAddedBg := Tint(bg, ansiColors["green"], diffAlpha)
	diffRemovedBg := Tint(bg, ansiColors["red"], diffAlpha)
	diffContextBg := grays[2]
	diffAddedLineNumberBg := Tint(diffContextBg, ansiColors["green"], diffAlpha)
	diffRemovedLineNumberBg := Tint(diffContextBg, ansiColors["red"], diffAlpha)
	return ThemeJSON{Theme: map[string]any{
		"primary": ansiColors["cyan"], "secondary": ansiColors["magenta"], "accent": ansiColors["cyan"],
		"error": ansiColors["red"], "warning": ansiColors["yellow"],
		"success": ansiColors["green"], "info": ansiColors["cyan"],
		"text": fg, "textMuted": textMuted, "selectedListItemText": bg,
		"background": transparent, "backgroundPanel": grays[2], "backgroundElement": grays[3], "backgroundMenu": grays[3],
		"borderSubtle": grays[6], "border": grays[7], "borderActive": grays[8],
		"diffAdded": ansiColors["green"], "diffRemoved": ansiColors["red"],
		"diffContext": grays[7], "diffHunkHeader": grays[7],
		"diffHighlightAdded": ansiColors["greenBright"], "diffHighlightRemoved": ansiColors["redBright"],
		"diffAddedBg": diffAddedBg, "diffRemovedBg": diffRemovedBg,
		"diffContextBg": diffContextBg, "diffLineNumber": textMuted,
		"diffAddedLineNumberBg": diffAddedLineNumberBg, "diffRemovedLineNumberBg": diffRemovedLineNumberBg,
		"markdownText": fg, "markdownHeading": fg, "markdownLink": ansiColors["blue"], "markdownLinkText": ansiColors["cyan"],
		"markdownCode": ansiColors["green"], "markdownBlockQuote": ansiColors["yellow"], "markdownEmph": ansiColors["yellow"],
		"markdownStrong": fg, "markdownHorizontalRule": grays[7], "markdownListItem": ansiColors["blue"],
		"markdownListEnumeration": ansiColors["cyan"],
		"markdownImage":           ansiColors["blue"], "markdownImageText": ansiColors["cyan"],
		"markdownCodeBlock": fg,
		"syntaxComment":     textMuted, "syntaxKeyword": ansiColors["magenta"], "syntaxFunction": ansiColors["blue"],
		"syntaxVariable": fg, "syntaxString": ansiColors["green"], "syntaxNumber": ansiColors["yellow"],
		"syntaxType": ansiColors["cyan"], "syntaxOperator": ansiColors["cyan"], "syntaxPunctuation": fg,
	}}
}

// TerminalMode is the port of upstream terminalMode
// (theme/index.ts:353-358): bg luminance > 0.5 (0-1) → "light", else
// "dark"; no bg → "".
func TerminalMode(bgHex string) string {
	if bgHex == "" {
		return ""
	}
	c := FromHex(bgHex)
	return luminanceMode(float64(c.R), float64(c.G), float64(c.B))
}

// luminanceMode is the shared luminance rule: 0.299r+0.587g+0.114b > 0.5.
// Inputs are 0-255; the comparison runs on the 0-1 scale, upstream-exact.
func luminanceMode(r, g, b float64) string {
	if 0.299*r/255+0.587*g/255+0.114*b/255 > 0.5 {
		return "light"
	}
	return "dark"
}
