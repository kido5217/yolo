package protocol

import "sort"

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

type Config struct {
	Model        string                    `json:"model,omitempty"`
	Agent        string                    `json:"agent,omitempty"`
	Provider     map[string]ProviderConfig `json:"provider,omitempty"`
	Permission   map[string]any            `json:"permission,omitempty"`
	Instructions []string                  `json:"instructions,omitempty"`
	Theme        map[string]any            `json:"theme,omitempty"`
	ToolOutput   *ToolOutput               `json:"tool_output,omitempty"`
	Agents       map[string]CustomAgent    `json:"agents,omitempty"`
}

// ParsePerms converts the config `permission` object into an ordered rule list:
// string value → {"*"} rule; map value → one rule per pattern (shortest first,
// then lexical). "*" rules precede specific patterns (last-match-wins semantics
// apply downstream).
func ParsePerms(m map[string]any) []Rule {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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
			sort.Slice(pats, func(i, j int) bool {
				if len(pats[i]) != len(pats[j]) {
					return len(pats[i]) < len(pats[j])
				}
				return pats[i] < pats[j]
			})
			for _, p := range pats {
				a, _ := v[p].(string)
				specific = append(specific, Rule{Permission: k, Pattern: p, Action: a})
			}
		}
	}
	return append(wild, specific...)
}
