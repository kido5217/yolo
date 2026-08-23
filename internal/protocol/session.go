package protocol

type SessionTime struct {
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
}

type CacheTokens struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

type Tokens struct {
	Input     int64       `json:"input"`
	Output    int64       `json:"output"`
	Reasoning int64       `json:"reasoning"`
	Cache     CacheTokens `json:"cache"`
}

type ModelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
}

type Session struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"projectID"`
	Directory  string         `json:"directory"`
	Title      string         `json:"title"`
	Agent      string         `json:"agent,omitempty"`
	Model      *ModelRef      `json:"model,omitempty"`
	Cost       float64        `json:"cost"`
	Tokens     Tokens         `json:"tokens"`
	Version    string         `json:"version"`
	Time       SessionTime    `json:"time"`
	Permission []Rule         `json:"permission,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type SessionStatus struct {
	Type    string `json:"type"` // "idle" | "busy" | "retry"
	Attempt int    `json:"attempt,omitempty"`
	Message string `json:"message,omitempty"`
	Next    int64  `json:"next,omitempty"`
}

const (
	SessionStatusIdle  = "idle"
	SessionStatusBusy  = "busy"
	SessionStatusRetry = "retry"
)

// Todo is one item in a session's todo list (todowrite tool).
type Todo struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority,omitempty"`
}
