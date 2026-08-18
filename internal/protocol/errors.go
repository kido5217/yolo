package protocol

// Error is the wire error envelope body: {"error": {"message": "...", "data"?}}.
type Error struct {
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
