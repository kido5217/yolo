package protocol

type MessageTime struct {
	Created   int64 `json:"created"`
	Completed int64 `json:"completed,omitempty"`
}

type MessageModel struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

type MessagePath struct {
	Cwd  string `json:"cwd"`
	Root string `json:"root"`
}

type MessageError struct {
	Type    string `json:"type"` // "unknown" | "aborted" | "overflow"
	Message string `json:"message"`
}

type Message struct {
	ID         string        `json:"id"`
	SessionID  string        `json:"sessionID"`
	Role       string        `json:"role"` // "user" | "assistant"
	Time       MessageTime   `json:"time"`
	Agent      string        `json:"agent,omitempty"`
	Model      *MessageModel `json:"model,omitempty"` // user messages
	ParentID   string        `json:"parentID,omitempty"`
	ModelID    string        `json:"modelID,omitempty"`
	ProviderID string        `json:"providerID,omitempty"`
	Mode       string        `json:"mode,omitempty"`
	Path       *MessagePath  `json:"path,omitempty"`
	Cost       float64       `json:"cost,omitempty"`
	Tokens     *Tokens       `json:"tokens,omitempty"`
	Finish     string        `json:"finish,omitempty"`
	Error      *MessageError `json:"error,omitempty"`
	Variant    string        `json:"variant,omitempty"`
}
