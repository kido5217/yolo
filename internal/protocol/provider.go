package protocol

type ProviderAuth struct {
	Type        string `json:"type"`   // "api" | "none"
	Status      string `json:"status"` // "loaded" | "missing" | "not-required"
	RequiresKey bool   `json:"requiresKey"`
}

type ModelLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

type ModelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

type Model struct {
	ID                 string     `json:"id"`
	ProviderID         string     `json:"providerID"`
	Name               string     `json:"name"`
	Family             string     `json:"family,omitempty"`
	SupportsToolCall   bool       `json:"supportsToolCall"`
	SupportsReasoning  bool       `json:"supportsReasoning"`
	SupportsAttachment bool       `json:"supportsAttachment"`
	Limit              ModelLimit `json:"limit"`
	Cost               ModelCost  `json:"cost"`
	Adapter            string     `json:"adapter"` // "openai" | "anthropic" (driver selection)
}

type Provider struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Source  string           `json:"source"` // "builtin" | "config"
	Env     []string         `json:"env"`
	Options map[string]any   `json:"options"`
	Models  map[string]Model `json:"models"`
	Auth    *ProviderAuth    `json:"auth,omitempty"` // Yolo extension: drives TUI auth status
}
