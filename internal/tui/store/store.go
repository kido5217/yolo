// Package store holds the TUI's in-memory view of the server state for the
// scope directory: REST hydration plus SSE events folded in via Apply.
package store

import (
	"encoding/json"

	"github.com/kido5217/yolo/internal/protocol"
)

// Store is the single shared TUI state. The app loop hydrates it over REST
// and calls Apply per SSE event; only the current session's messages are
// tracked.
type Store struct {
	Sessions    []protocol.Session
	Current     *protocol.Session
	Messages    []protocol.MessageWithParts
	Providers   []protocol.Provider
	Agents      []protocol.Agent
	Commands    []protocol.Command
	Config      map[string]any
	Status      protocol.SessionStatus // zero value = idle
	Pending     []protocol.PermissionAskedProps
	Conn        bool // SSE live (set by the app loop, not Apply)
	LastHydrate int64
}

// Apply folds one server event into the store.
func (s *Store) Apply(ev protocol.Event) {
	switch ev.Type {
	case protocol.EventTypeMessageUpdated:
		var p protocol.MessageUpdatedProps
		if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
			return
		}
		s.upsertMessage(p.Info)
	case protocol.EventTypeMessagePartUpdated:
		var p protocol.MessagePartUpdatedProps
		if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.Part.SessionID) {
			return
		}
		s.upsertPart(p.Part)
	case protocol.EventTypeMessagePartDelta:
		var p protocol.MessagePartDeltaProps
		if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
			return
		}
		s.applyDelta(p)
	case protocol.EventTypeMessageRemoved:
		var p protocol.MessageRemovedProps
		if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
			return
		}
		kept := make([]protocol.MessageWithParts, 0, len(s.Messages))
		for _, m := range s.Messages {
			if m.Info.ID != p.MessageID {
				kept = append(kept, m)
			}
		}
		s.Messages = kept
	case protocol.EventTypeMessagePartRemoved:
		var p protocol.MessagePartRemovedProps
		if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
			return
		}
		for i := range s.Messages {
			if s.Messages[i].Info.ID != p.MessageID {
				continue
			}
			kept := make([]protocol.Part, 0, len(s.Messages[i].Parts))
			for _, pr := range s.Messages[i].Parts {
				if pr.ID != p.PartID {
					kept = append(kept, pr)
				}
			}
			s.Messages[i].Parts = kept
		}
	case protocol.EventTypeSessionUpdated:
		var p protocol.SessionUpdatedProps
		if json.Unmarshal(ev.Properties, &p) != nil {
			return
		}
		s.upsertSession(p.Info)
		if s.isCurrent(p.Info.ID) {
			cp := p.Info
			s.Current = &cp
		}
	case protocol.EventTypeSessionDeleted:
		var p protocol.SessionDeletedProps
		if json.Unmarshal(ev.Properties, &p) != nil {
			return
		}
		kept := make([]protocol.Session, 0, len(s.Sessions))
		for _, se := range s.Sessions {
			if se.ID != p.SessionID {
				kept = append(kept, se)
			}
		}
		s.Sessions = kept
		if s.isCurrent(p.SessionID) {
			s.Current = nil
		}
	case protocol.EventTypeSessionStatus:
		var p protocol.SessionStatusProps
		if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
			return
		}
		s.Status = p.Status
	case protocol.EventTypePermissionAsked:
		var p protocol.PermissionAskedProps
		if json.Unmarshal(ev.Properties, &p) != nil {
			return
		}
		for _, q := range s.Pending {
			if q.ID == p.ID {
				return
			}
		}
		s.Pending = append(s.Pending, p)
	case protocol.EventTypePermissionReplied:
		var p protocol.PermissionRepliedProps
		if json.Unmarshal(ev.Properties, &p) != nil {
			return
		}
		kept := make([]protocol.PermissionAskedProps, 0, len(s.Pending))
		for _, q := range s.Pending {
			if q.ID != p.RequestID {
				kept = append(kept, q)
			}
		}
		s.Pending = kept
	}
}

func (s *Store) isCurrent(sessionID string) bool {
	return s.Current != nil && s.Current.ID == sessionID
}

func (s *Store) upsertSession(se protocol.Session) {
	for i := range s.Sessions {
		if s.Sessions[i].ID == se.ID {
			s.Sessions[i] = se
			return
		}
	}
	s.Sessions = append(s.Sessions, se)
}

func (s *Store) upsertMessage(m protocol.Message) {
	for i := range s.Messages {
		if s.Messages[i].Info.ID == m.ID {
			s.Messages[i].Info = m
			return
		}
	}
	s.Messages = append(s.Messages, protocol.MessageWithParts{Info: m, Parts: []protocol.Part{}})
}

func (s *Store) upsertPart(part protocol.Part) {
	for i := range s.Messages {
		if s.Messages[i].Info.ID != part.MessageID {
			continue
		}
		for j := range s.Messages[i].Parts {
			if s.Messages[i].Parts[j].ID == part.ID {
				s.Messages[i].Parts[j] = part
				return
			}
		}
		s.Messages[i].Parts = append(s.Messages[i].Parts, part)
		return
	}
	s.Messages = append(s.Messages, protocol.MessageWithParts{
		Info:  protocol.Message{ID: part.MessageID, SessionID: part.SessionID},
		Parts: []protocol.Part{part},
	})
}

func (s *Store) applyDelta(p protocol.MessagePartDeltaProps) {
	for i := range s.Messages {
		if s.Messages[i].Info.ID != p.MessageID {
			continue
		}
		for j := range s.Messages[i].Parts {
			if s.Messages[i].Parts[j].ID == p.PartID {
				appendDelta(&s.Messages[i].Parts[j], p.Field, p.Delta)
				return
			}
		}
		// delta before the part announce: synthesize the part
		pr := protocol.Part{ID: p.PartID, SessionID: p.SessionID, MessageID: p.MessageID}
		appendDelta(&pr, p.Field, p.Delta)
		s.Messages[i].Parts = append(s.Messages[i].Parts, pr)
		return
	}
	// delta for an unknown message: synthesize message and part
	pr := protocol.Part{ID: p.PartID, SessionID: p.SessionID, MessageID: p.MessageID}
	appendDelta(&pr, p.Field, p.Delta)
	s.Messages = append(s.Messages, protocol.MessageWithParts{
		Info:  protocol.Message{ID: p.MessageID, SessionID: p.SessionID},
		Parts: []protocol.Part{pr},
	})
}

// appendDelta appends a streamed delta to the part per field ("text" |
// "reasoning" -> Text, "input" -> ToolState.Input["input"]).
func appendDelta(pr *protocol.Part, field, delta string) {
	if pr.Type == "" {
		switch field {
		case "reasoning":
			pr.Type = "reasoning"
		case "input":
			pr.Type = "tool"
		default:
			pr.Type = "text"
		}
	}
	switch field {
	case "reasoning":
		pr.Text += delta
	case "input":
		if pr.State == nil {
			pr.State = &protocol.ToolState{Status: "running"}
		}
		if pr.State.Input == nil {
			pr.State.Input = map[string]any{}
		}
		in, _ := pr.State.Input["input"].(string)
		pr.State.Input["input"] = in + delta
	default:
		pr.Text += delta
	}
}
