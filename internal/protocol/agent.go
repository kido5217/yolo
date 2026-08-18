package protocol

type Rule struct {
	Permission string `json:"permission"`
	Pattern    string `json:"pattern"`
	Action     string `json:"action"` // "allow" | "deny" | "ask"
}

const (
	ActionAllow = "allow"
	ActionDeny  = "deny"
	ActionAsk   = "ask"
)

type Agent struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Mode        string         `json:"mode"` // "primary"
	Model       *ModelRef      `json:"model,omitempty"`
	Permission  []Rule         `json:"permission"`
	Options     map[string]any `json:"options"`
	Hidden      bool           `json:"hidden,omitempty"`
}

type Command struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Template    string   `json:"template,omitempty"`
	Hints       []string `json:"hints"`
}
