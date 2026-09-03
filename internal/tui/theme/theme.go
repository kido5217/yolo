// Package theme is the TUI-local theme engine: 33 embedded upstream themes,
// resolution, system-theme generation, terminal palette detection, custom
// discovery, and the selection chain (config > KV > default). TUI-local by
// design (root principle 4): no internal/* imports outside internal/tui —
// every filesystem path is injected by cmd/yolo.
package theme

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// DefaultName is the fallback active theme (upstream default,
// theme.tsx:96; rebranded to the yolo default).
const DefaultName = "yolo"

// ThemeJSON mirrors the upstream theme JSON shape (theme/index.ts:120):
// defs = named color constants; theme = semantic tokens, each a
// {dark,light} variant, a hex string, a defs/theme ref name, or an ANSI int.
type ThemeJSON struct {
	Schema string         `json:"$schema,omitempty"`
	Defs   map[string]any `json:"defs,omitempty"`
	Theme  map[string]any `json:"theme"`
}

var (
	allThemesOnce sync.Once
	allThemes     map[string]ThemeJSON
	allThemesErr  error
)

// AllThemes parses the 33 embedded upstream theme assets. Names are the asset
// file stems (kebab-case preserved: catppuccin-frappe, one-dark, ...). The
// assets are embedded (immutable), so the parsed map is cached for the
// process lifetime; callers must treat the returned map as read-only.
func AllThemes() (map[string]ThemeJSON, error) {
	allThemesOnce.Do(func() {
		allThemes, allThemesErr = loadAllThemes()
	})
	return allThemes, allThemesErr
}

func loadAllThemes() (map[string]ThemeJSON, error) {
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		return nil, fmt.Errorf("theme assets: %w", err)
	}
	out := make(map[string]ThemeJSON, len(entries))
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		data, err := assetsFS.ReadFile("assets/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("theme asset %s: %w", e.Name(), err)
		}
		var tj ThemeJSON
		if err := json.Unmarshal(data, &tj); err != nil {
			return nil, fmt.Errorf("theme asset %s: %w", e.Name(), err)
		}
		if tj.Theme == nil {
			return nil, fmt.Errorf("theme asset %s: not a theme", e.Name())
		}
		out[name] = tj
	}
	return out, nil
}

// IsTheme is the upstream isTheme check (theme/index.ts:194): a non-array
// object with a non-array object "theme" member.
func IsTheme(v any) bool {
	obj, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, ok = obj["theme"].(map[string]any)
	return ok
}
