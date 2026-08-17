package protocol

import "encoding/json"

// Event is the SSE frame payload: {"id","type","properties"}.
type Event struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

const (
	EventTypeMessageUpdated     = "message.updated"
	EventTypeMessagePartUpdated = "message.part.updated"
	EventTypeMessagePartDelta   = "message.part.delta"
	EventTypeMessageRemoved     = "message.removed"
	EventTypeMessagePartRemoved = "message.part.removed"
	EventTypeSessionUpdated     = "session.updated"
	EventTypeSessionDeleted     = "session.deleted"
	EventTypeSessionStatus      = "session.status"
	EventTypePermissionAsked    = "permission.asked"
	EventTypePermissionReplied  = "permission.replied"
)

// MakeEvent wraps typed props in the legacy SSE envelope.
func MakeEvent(t string, props any) (Event, error) {
	raw, err := json.Marshal(props)
	if err != nil {
		return Event{}, err
	}
	return Event{ID: NewEventID(), Type: t, Properties: raw}, nil
}

// typed props (JSON shapes match upstream v1.18.18 openapi legacy schemas):

type MessageUpdatedProps struct {
	SessionID string  `json:"sessionID"`
	Info      Message `json:"info"`
}

type MessageRemovedProps struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
}

type MessagePartUpdatedProps struct {
	SessionID string `json:"sessionID"`
	Part      Part   `json:"part"`
	Time      int64  `json:"time"`
}

type MessagePartDeltaProps struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
	Field     string `json:"field"` // "text" | "reasoning" | "input"
	Delta     string `json:"delta"`
}

type MessagePartRemovedProps struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
}

type SessionUpdatedProps struct {
	SessionID string  `json:"sessionID"`
	Info      Session `json:"info"`
}

type SessionDeletedProps struct {
	SessionID string  `json:"sessionID"`
	Info      Session `json:"info"`
}

type SessionStatusProps struct {
	SessionID string        `json:"sessionID"`
	Status    SessionStatus `json:"status"`
}

type PermissionToolRef struct {
	MessageID string `json:"messageID"`
	CallID    string `json:"callID"`
}

type PermissionAskedProps struct {
	ID         string             `json:"id"`
	SessionID  string             `json:"sessionID"`
	Permission string             `json:"permission"`
	Patterns   []string           `json:"patterns"`
	Metadata   map[string]any     `json:"metadata"`
	Always     []string           `json:"always"`
	Tool       *PermissionToolRef `json:"tool,omitempty"`
}

type PermissionRepliedProps struct {
	SessionID string `json:"sessionID"`
	RequestID string `json:"requestID"`
	Reply     string `json:"reply"` // "once" | "always" | "reject"
	Auto      bool   `json:"auto,omitempty"`
}
