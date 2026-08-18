package protocol

// CommandResponse is the wire shape of POST /session/:id/command (M5).
type CommandResponse struct {
	SessionID string `json:"session_id,omitempty"`
	Handled   string `json:"handled,omitempty"`
}
