package theme

import (
	"fmt"
	"strconv"
	"strings"
)

// Rgba is a 0-255 color with alpha. Upstream RGBA is float 0-1 but every
// color is int-derived (hex/int/ANSI) or produced by float ops on 0-255
// values rounded at the end (tint, grays), so uint8 storage is exact and the
// operation ORDER is preserved for bit-identical results (strict-copy bar).
type Rgba struct{ R, G, B, A uint8 }

// Hex is the golden-matrix form "#rrggbbaa".
func (c Rgba) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

func isHex(h string, n int) bool {
	if len(h) != n {
		return false
	}
	for _, r := range h {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// FromHex is the port of @opentui/core hexToRgb (0.4.5): strips "#", expands
// 3→6 and 4→8 digits, accepts 6/8-digit hex; invalid input → magenta
// (upstream additionally console.warns — a non-visual log side effect,
// skipped).
func FromHex(s string) Rgba {
	h := strings.TrimPrefix(s, "#")
	switch len(h) {
	case 3:
		h = h[0:1] + h[0:1] + h[1:2] + h[1:2] + h[2:3] + h[2:3]
	case 4:
		h = h[0:1] + h[0:1] + h[1:2] + h[1:2] + h[2:3] + h[2:3] + h[3:4] + h[3:4]
	}
	if !isHex(h, 6) && !isHex(h, 8) {
		return Rgba{255, 0, 255, 255}
	}
	a := uint8(255)
	if len(h) == 8 {
		a = hexByte(h[6:8])
	}
	return Rgba{hexByte(h[0:2]), hexByte(h[2:4]), hexByte(h[4:6]), a}
}

func hexByte(s string) uint8 {
	v, _ := strconv.ParseUint(s, 16, 8)
	return uint8(v)
}

// ansi16 is the upstream standard ANSI table (theme/index.ts:304-321).
var ansi16 = []string{
	"#000000", "#800000", "#008000", "#808000", "#000080", "#800080",
	"#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00",
	"#0000ff", "#ff00ff", "#00ffff", "#ffffff",
}

// AnsiToRgba is the port of upstream ansiToRgba (theme/index.ts:301):
// 0-15 standard, 16-231 the 6x6x6 cube, 232-255 the grayscale ramp,
// anything else black.
func AnsiToRgba(code int) Rgba {
	if code < 16 {
		hex := "#000000"
		if code >= 0 && code < len(ansi16) {
			hex = ansi16[code]
		}
		return FromHex(hex) // upstream: ansiColors[code] ?? "#000000"
	}
	if code < 232 {
		index := code - 16
		b := index % 6
		g := index / 6 % 6
		r := index / 36
		val := func(x int) int {
			if x == 0 {
				return 0
			}
			return x*40 + 55
		}
		return Rgba{uint8(val(r)), uint8(val(g)), uint8(val(b)), 255}
	}
	if code < 256 {
		gray := (code-232)*10 + 8
		return Rgba{uint8(gray), uint8(gray), uint8(gray), 255}
	}
	return Rgba{0, 0, 0, 255}
}

// Resolved is the output of ResolveTheme: every token (incl. the two
// optional ones) mapped to its resolved color, plus the bookkeeping fields.
type Resolved struct {
	Colors                  map[string]Rgba
	ThinkingOpacity         float64
	HasSelectedListItemText bool
}

// Color returns the resolved token (ok=false when absent).
func (r Resolved) Color(name string) (Rgba, bool) {
	c, ok := r.Colors[name]
	return c, ok
}

// ResolveTheme is the port of upstream resolveTheme
// (theme/index.ts:241-299): defs refs, "#hex", "transparent"/"none",
// ANSI ints, {dark,light} variants; optional selectedListItemText
// (default: background), backgroundMenu (default: backgroundElement),
// thinkingOpacity (default: 0.6). Error messages keep the upstream wording.
func ResolveTheme(j ThemeJson, mode string) (Resolved, error) {
	defs := j.Defs
	var resolve func(c any, chain []string) (Rgba, error)
	resolve = func(c any, chain []string) (Rgba, error) {
		if rgb, ok := c.(Rgba); ok {
			return rgb, nil // generateSystem output values (upstream RGBA instanceof)
		}
		switch v := c.(type) {
		case string:
			if v == "transparent" || v == "none" {
				return Rgba{0, 0, 0, 0}, nil
			}
			if strings.HasPrefix(v, "#") {
				return FromHex(v), nil
			}
			for _, prev := range chain {
				if prev == v {
					return Rgba{}, fmt.Errorf("circular color reference: %s", strings.Join(append(chain, v), " -> "))
				}
			}
			next, ok := defs[v]
			if !ok {
				next, ok = j.Theme[v]
			}
			if !ok {
				return Rgba{}, fmt.Errorf("color reference %q not found in defs or theme", v)
			}
			return resolve(next, append(chain, v))
		case float64: // JSON numbers unmarshal to float64
			return AnsiToRgba(int(v)), nil
		case map[string]any:
			return resolve(v[mode], chain)
		}
		return Rgba{}, fmt.Errorf("unresolvable color value %v (%T)", c, c)
	}
	resolved := make(map[string]Rgba, len(j.Theme))
	for key, value := range j.Theme {
		switch key {
		case "selectedListItemText", "backgroundMenu", "thinkingOpacity":
			continue
		}
		c, err := resolve(value, nil)
		if err != nil {
			return Resolved{}, fmt.Errorf("token %s: %w", key, err)
		}
		resolved[key] = c
	}
	hasSLIT := j.Theme["selectedListItemText"] != nil
	if hasSLIT {
		c, err := resolve(j.Theme["selectedListItemText"], nil)
		if err != nil {
			return Resolved{}, fmt.Errorf("token selectedListItemText: %w", err)
		}
		resolved["selectedListItemText"] = c
	} else {
		resolved["selectedListItemText"] = resolved["background"]
	}
	if j.Theme["backgroundMenu"] != nil {
		c, err := resolve(j.Theme["backgroundMenu"], nil)
		if err != nil {
			return Resolved{}, fmt.Errorf("token backgroundMenu: %w", err)
		}
		resolved["backgroundMenu"] = c
	} else if be, ok := resolved["backgroundElement"]; ok {
		resolved["backgroundMenu"] = be
	} else {
		resolved["backgroundMenu"] = resolved["background"]
	}
	thinkingOpacity := 0.6
	if v, ok := j.Theme["thinkingOpacity"].(float64); ok {
		thinkingOpacity = v
	}
	return Resolved{
		Colors:                  resolved,
		ThinkingOpacity:         thinkingOpacity,
		HasSelectedListItemText: hasSLIT,
	}, nil
}
