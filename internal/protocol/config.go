package protocol

import (
	"fmt"
	"slices"
	"strings"
)

// config wire = config file schema (spec 6.1)

type ToolOutput struct {
	MaxLines int `json:"max_lines,omitempty"`
	MaxBytes int `json:"max_bytes,omitempty"`
}

type ProviderConfig struct {
	BaseURL string         `json:"baseURL,omitempty"`
	APIKey  string         `json:"apiKey,omitempty"`
	Options map[string]any `json:"options,omitempty"`
	Models  map[string]any `json:"models,omitempty"`
}

type CustomAgent struct {
	Description string         `json:"description,omitempty"`
	Permission  map[string]any `json:"permission,omitempty"`
}

// Profile is the optional "profile" element of a profile's config file:
// display metadata for the profile directory (see config.List).
type Profile struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type Config struct {
	Profile      *Profile                  `json:"profile,omitempty"`
	Model        string                    `json:"model,omitempty"`
	Agent        string                    `json:"agent,omitempty"`
	Provider     map[string]ProviderConfig `json:"provider,omitempty"`
	Permission   map[string]any            `json:"permission,omitempty"`
	Instructions []string                  `json:"instructions,omitempty"`
	Theme        string                    `json:"theme,omitempty"`
	ToolOutput   *ToolOutput               `json:"tool_output,omitempty"`
	Agents       map[string]CustomAgent    `json:"agents,omitempty"`
	// Keybinds is the yolo.jsonc keybinds overrides (S4.3): the binding name
	// → the raw binding value (string | keystroke object | array |
	// false/"none"). omitempty keeps the GET /config goldens byte-identical.
	Keybinds map[string]any `json:"keybinds,omitempty"`
}

// ParsePerms converts the config `permission` object into an ordered rule list:
// string value → {"*"} rule; map value → one rule per pattern (shortest first,
// then lexical). "*" rules precede specific patterns (last-match-wins semantics
// apply downstream). Non-string values (top-level or per-pattern) are a config
// error, not a rule: a silently empty action would evaluate to an accidental
// allow downstream.
func ParsePerms(m map[string]any) ([]Rule, error) {
	if m == nil {
		return nil, nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var wild, specific []Rule
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			wild = append(wild, Rule{Permission: k, Pattern: "*", Action: v})
		case map[string]any:
			pats := make([]string, 0, len(v))
			for p := range v {
				pats = append(pats, p)
			}
			slices.SortFunc(pats, func(a, b string) int {
				if len(a) != len(b) {
					return len(a) - len(b)
				}
				return strings.Compare(a, b)
			})
			for _, p := range pats {
				a, ok := v[p].(string)
				if !ok {
					return nil, fmt.Errorf("permission %s: pattern %q value must be a string", k, p)
				}
				specific = append(specific, Rule{Permission: k, Pattern: p, Action: a})
			}
		default:
			return nil, fmt.Errorf("permission %s: value must be a string or object", k)
		}
	}
	return append(wild, specific...), nil
}
