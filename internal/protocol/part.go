package protocol

type PartTime struct {
	Start     int64 `json:"start"`
	End       int64 `json:"end,omitempty"`
	Compacted int64 `json:"compacted,omitempty"`
}

type ToolState struct {
	Status   string         `json:"status"` // "running" | "completed" | "error"
	Input    map[string]any `json:"input"`
	Title    string         `json:"title,omitempty"`
	Output   string         `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Time     PartTime       `json:"time"`
}

type Part struct {
	ID          string         `json:"id"`
	SessionID   string         `json:"sessionID"`
	MessageID   string         `json:"messageID"`
	Type        string         `json:"type"` // "text" | "reasoning" | "tool"
	Text        string         `json:"text,omitempty"`
	CallID      string         `json:"callID,omitempty"`
	Tool        string         `json:"tool,omitempty"`
	State       *ToolState     `json:"state,omitempty"`
	IsSynthetic *bool          `json:"isSynthetic,omitempty"`
	IsIgnored   *bool          `json:"isIgnored,omitempty"`
	Time        PartTime       `json:"time"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type MessageWithParts struct {
	Info  Message `json:"info"`
	Parts []Part  `json:"parts"`
}
