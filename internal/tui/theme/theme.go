// Package theme is the TUI-local theme engine: 33 embedded upstream themes,
// resolution, system-theme generation, terminal palette detection, custom
// discovery, and the selection chain (config > KV > default). TUI-local by
// design (root principle 4): no internal/* imports outside internal/tui —
// every filesystem path is injected by cmd/yolo.
package theme

import (
	"encoding/json"
	"fmt"
)

// DefaultName is the fallback active theme (upstream default, theme.tsx:96).
const DefaultName = "opencode"

// ThemeJson mirrors the upstream theme JSON shape (theme/index.ts:120):
// defs = named color constants; theme = semantic tokens, each a
// {dark,light} variant, a hex string, a defs/theme ref name, or an ANSI int.
type ThemeJson struct {
	Schema string         `json:"$schema,omitempty"`
	Defs   map[string]any `json:"defs,omitempty"`
	Theme  map[string]any `json:"theme"`
}

// AllThemes parses the 33 embedded upstream theme assets. Names are the asset
// file stems (kebab-case preserved: catppuccin-frappe, one-dark, ...).
func AllThemes() (map[string]ThemeJson, error) {
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		return nil, fmt.Errorf("theme assets: %w", err)
	}
	out := make(map[string]ThemeJson, len(entries))
	for _, e := range entries {
		name := e.Name()[:len(e.Name())-len(".json")]
		data, err := assetsFS.ReadFile("assets/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("theme asset %s: %w", e.Name(), err)
		}
		var tj ThemeJson
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
