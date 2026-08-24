// Package store holds the TUI's in-memory view of the server state for the
// scope directory: REST hydration plus SSE events folded in via Apply.
package store

import (
	"encoding/json"
	"strings"

	"github.com/kido5217/yolo/internal/protocol"
)

// State is the single shared TUI state. The app loop hydrates it over REST
// and calls Apply per SSE event; only the current session's messages are
// tracked.
type State struct {
	Sessions    []protocol.Session
	Current     *protocol.Session
	Messages    []protocol.MessageWithParts
	Providers   []protocol.Provider
	Agents      []protocol.Agent
	Commands    []protocol.Command
	Config      map[string]any
	Status      protocol.SessionStatus // zero value = idle
	Pending     []protocol.PermissionAskedProps
	Live        bool // SSE live (set by the app loop, not Apply)
	LastHydrate int64

	// parts holds the streamed-part state: the part's location in Messages
	// (delta fast path) plus one builder per part so a per-token delta
	// appends instead of re-copying the whole accumulated text.
	parts map[string]*partState
}

// partPos locates one part inside Messages.
type partPos struct{ msg, part int }

// partState is one part's delta accumulation: where the part sits, the
// field last appended to and its shadow text.
type partState struct {
	pos   partPos
	field string // "text" | "reasoning" | "input"
	buf   strings.Builder
}

// Apply folds one server event into the store.
func (s *State) Apply(ev protocol.Event) {
	switch ev.Type {
	case protocol.EventTypeMessageUpdated:
		s.applyMessageUpdated(ev)
	case protocol.EventTypeMessagePartUpdated:
		s.applyMessagePartUpdated(ev)
	case protocol.EventTypeMessagePartDelta:
		s.applyMessagePartDelta(ev)
	case protocol.EventTypeMessageRemoved:
		s.applyMessageRemoved(ev)
	case protocol.EventTypeMessagePartRemoved:
		s.applyMessagePartRemoved(ev)
	case protocol.EventTypeSessionUpdated:
		s.applySessionUpdated(ev)
	case protocol.EventTypeSessionDeleted:
		s.applySessionDeleted(ev)
	case protocol.EventTypeSessionStatus:
		s.applySessionStatus(ev)
	case protocol.EventTypePermissionAsked:
		s.applyPermissionAsked(ev)
	case protocol.EventTypePermissionReplied:
		s.applyPermissionReplied(ev)
	}
}

func (s *State) applyMessageUpdated(ev protocol.Event) {
	var p protocol.MessageUpdatedProps
	if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
		return
	}
	s.upsertMessage(p.Info)
}

func (s *State) applyMessagePartUpdated(ev protocol.Event) {
	var p protocol.MessagePartUpdatedProps
	if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.Part.SessionID) {
		return
	}
	s.upsertPart(p.Part)
}

func (s *State) applyMessagePartDelta(ev protocol.Event) {
	var p protocol.MessagePartDeltaProps
	if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
		return
	}
	s.applyDelta(p)
}

func (s *State) applyMessageRemoved(ev protocol.Event) {
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
	// The removal shifts message positions: re-derive the index.
	s.rebuildPartIndex()
}

func (s *State) applyMessagePartRemoved(ev protocol.Event) {
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
	// The removal shifts part positions within the message.
	s.rebuildPartIndex()
}

func (s *State) applySessionUpdated(ev protocol.Event) {
	var p protocol.SessionUpdatedProps
	if json.Unmarshal(ev.Properties, &p) != nil {
		return
	}
	s.upsertSession(p.Info)
	if s.isCurrent(p.Info.ID) {
		cp := p.Info
		s.Current = &cp
	}
}

func (s *State) applySessionDeleted(ev protocol.Event) {
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
}

func (s *State) applySessionStatus(ev protocol.Event) {
	var p protocol.SessionStatusProps
	if json.Unmarshal(ev.Properties, &p) != nil || !s.isCurrent(p.SessionID) {
		return
	}
	s.Status = p.Status
}

func (s *State) applyPermissionAsked(ev protocol.Event) {
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
}

func (s *State) applyPermissionReplied(ev protocol.Event) {
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

func (s *State) isCurrent(sessionID string) bool {
	return s.Current != nil && s.Current.ID == sessionID
}

func (s *State) upsertSession(se protocol.Session) {
	for i := range s.Sessions {
		if s.Sessions[i].ID == se.ID {
			s.Sessions[i] = se
			return
		}
	}
	s.Sessions = append(s.Sessions, se)
}

func (s *State) upsertMessage(m protocol.Message) {
	for i := range s.Messages {
		if s.Messages[i].Info.ID == m.ID {
			s.Messages[i].Info = m
			return
		}
	}
	s.Messages = append(s.Messages, protocol.MessageWithParts{Info: m, Parts: []protocol.Part{}})
}

func (s *State) upsertPart(part protocol.Part) {
	// A full part update is authoritative in every branch: drop any
	// accumulation shadow so the next delta re-seeds from the new text.
	s.deletePartState(part.ID)
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
		s.placePart(part.ID, i, len(s.Messages[i].Parts)-1)
		return
	}
	s.Messages = append(s.Messages, protocol.MessageWithParts{
		Info:  protocol.Message{ID: part.MessageID, SessionID: part.SessionID},
		Parts: []protocol.Part{part},
	})
	s.placePart(part.ID, len(s.Messages)-1, 0)
}

func (s *State) applyDelta(p protocol.MessagePartDeltaProps) {
	// Fast path: the streaming part's location is known. The re-check
	// keeps a stale index entry from writing through the wrong slot.
	if st := s.parts[p.PartID]; st != nil {
		pp := st.pos
		if pp.msg < len(s.Messages) && pp.part < len(s.Messages[pp.msg].Parts) &&
			s.Messages[pp.msg].Info.ID == p.MessageID && s.Messages[pp.msg].Parts[pp.part].ID == p.PartID {
			s.appendDelta(&s.Messages[pp.msg].Parts[pp.part], p.Field, p.Delta)
			return
		}
		delete(s.parts, p.PartID)
	}
	for i := range s.Messages {
		if s.Messages[i].Info.ID != p.MessageID {
			continue
		}
		for j := range s.Messages[i].Parts {
			if s.Messages[i].Parts[j].ID != p.PartID {
				continue
			}
			s.appendDelta(&s.Messages[i].Parts[j], p.Field, p.Delta)
			s.placePart(p.PartID, i, j)
			return
		}
		// delta before the part announce: synthesize the part
		s.Messages[i].Parts = append(s.Messages[i].Parts, protocol.Part{
			ID: p.PartID, SessionID: p.SessionID, MessageID: p.MessageID,
		})
		s.placePart(p.PartID, i, len(s.Messages[i].Parts)-1)
		s.appendDelta(&s.Messages[i].Parts[len(s.Messages[i].Parts)-1], p.Field, p.Delta)
		return
	}
	// delta for an unknown message: synthesize message and part
	s.Messages = append(s.Messages, protocol.MessageWithParts{
		Info:  protocol.Message{ID: p.MessageID, SessionID: p.SessionID},
		Parts: []protocol.Part{{ID: p.PartID, SessionID: p.SessionID, MessageID: p.MessageID}},
	})
	s.placePart(p.PartID, len(s.Messages)-1, 0)
	s.appendDelta(&s.Messages[len(s.Messages)-1].Parts[0], p.Field, p.Delta)
}

// appendDelta appends a streamed delta to the part per field ("text" |
// "reasoning" -> Text, "input" -> ToolState.Input["input"]). The
// accumulation runs through the part's builder shadow (State.parts), so a
// per-token delta is one Write instead of re-copying the whole accumulated
// text; the slot stores Builder.String() (an alias: the previous snapshot
// stays intact, Write appends after it or regrows to a new array).
func (s *State) appendDelta(pr *protocol.Part, field, delta string) {
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
	if field == "input" {
		if pr.State == nil {
			pr.State = &protocol.ToolState{Status: "running"}
		}
		if pr.State.Input == nil {
			pr.State.Input = map[string]any{}
		}
	}
	b := s.builderFor(pr, field)
	b.WriteString(delta)
	if field == "input" {
		pr.State.Input["input"] = b.String()
	} else {
		pr.Text = b.String()
	}
}

// builderFor returns the part's accumulation shadow, seeding it from the
// part's current field value on first use (or a field switch) so appending
// preserves any text already present (hydration, full part update).
func (s *State) builderFor(pr *protocol.Part, field string) *strings.Builder {
	var seed string
	if field == "input" {
		if pr.State != nil {
			seed, _ = pr.State.Input["input"].(string)
		}
	} else {
		seed = pr.Text
	}
	if s.parts == nil {
		s.parts = make(map[string]*partState)
	}
	st := s.parts[pr.ID]
	if st == nil || st.field != field {
		if st == nil {
			st = &partState{field: field}
			s.parts[pr.ID] = st
		} else {
			st.field = field
			st.buf.Reset()
		}
		st.buf.WriteString(seed)
	}
	return &st.buf
}

// deletePartState drops one part's state (the full part update that
// replaces it is authoritative; the next delta re-seeds).
func (s *State) deletePartState(partID string) {
	delete(s.parts, partID)
}

// placePart records the part's location, creating the (empty) state when
// absent — the shadow text is seeded lazily by the first delta.
func (s *State) placePart(partID string, i, j int) {
	if s.parts == nil {
		s.parts = make(map[string]*partState)
	}
	st := s.parts[partID]
	if st == nil {
		st = &partState{}
		s.parts[partID] = st
	}
	st.pos = partPos{msg: i, part: j}
}

// rebuildPartIndex derives every part location fresh from Messages (rare:
// a message or part removal shifts the slice). Accumulation shadows are
// dropped with it; the next delta re-seeds from the part's current text.
func (s *State) rebuildPartIndex() {
	next := make(map[string]*partState)
	for i := range s.Messages {
		for j := range s.Messages[i].Parts {
			next[s.Messages[i].Parts[j].ID] = &partState{pos: partPos{msg: i, part: j}}
		}
	}
	s.parts = next
}

// ForgetParts drops every part state and accumulation shadow; call when
// Messages is replaced wholesale (e.g. a session-route hydrate).
func (s *State) ForgetParts() {
	s.parts = nil
}
