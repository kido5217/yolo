package store_test

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
)

func ev(t *testing.T, typ string, props any) protocol.Event {
	t.Helper()
	e, err := protocol.MakeEvent(typ, props)
	if err != nil {
		t.Fatalf("MakeEvent(%s): %v", typ, err)
	}
	return e
}

// seed is the pinned store state: one current session, one message, one part.
func seed(t *testing.T) *store.Store {
	t.Helper()
	cur := protocol.Session{ID: "ses_1", Title: "T", ProjectID: "prj_1", Directory: "/d"}
	return &store.Store{
		Sessions: []protocol.Session{cur},
		Current:  &cur,
		Messages: []protocol.MessageWithParts{{
			Info:  protocol.Message{ID: "msg_1", SessionID: "ses_1", Role: "user"},
			Parts: []protocol.Part{{ID: "prt_1", SessionID: "ses_1", MessageID: "msg_1", Type: "text", Text: "hi"}},
		}},
		Providers: []protocol.Provider{{ID: "kido"}},
		Agents:    []protocol.Agent{{Name: "build"}},
		Commands:  []protocol.Command{{Name: "/help"}},
		Config:    map[string]any{"model": "kido/q"},
	}
}

func TestApply(t *testing.T) {
	t.Run("message.updated upserts current session messages", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
			SessionID: "ses_1",
			Info:      protocol.Message{ID: "msg_2", SessionID: "ses_1", Role: "assistant"},
		}))
		if len(s.Messages) != 2 || s.Messages[1].Info.ID != "msg_2" || s.Messages[1].Info.Role != "assistant" {
			t.Fatalf("messages = %+v", s.Messages)
		}
		s.Apply(ev(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
			SessionID: "ses_1",
			Info:      protocol.Message{ID: "msg_1", SessionID: "ses_1", Role: "user", Finish: "stop"},
		}))
		if len(s.Messages) != 2 || s.Messages[0].Info.Finish != "stop" {
			t.Fatalf("messages = %+v, want msg_1 updated in place", s.Messages)
		}
	})

	t.Run("message.part.updated upserts parts", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
			SessionID: "ses_1",
			Time:      123,
			Part:      protocol.Part{ID: "prt_2", SessionID: "ses_1", MessageID: "msg_1", Type: "text", Text: "more"},
		}))
		if len(s.Messages[0].Parts) != 2 || s.Messages[0].Parts[1].ID != "prt_2" || s.Messages[0].Parts[1].Text != "more" {
			t.Fatalf("parts = %+v", s.Messages[0].Parts)
		}
		s.Apply(ev(t, protocol.EventTypeMessagePartUpdated, protocol.MessagePartUpdatedProps{
			SessionID: "ses_1",
			Time:      124,
			Part:      protocol.Part{ID: "prt_1", SessionID: "ses_1", MessageID: "msg_1", Type: "text", Text: "edited"},
		}))
		if len(s.Messages[0].Parts) != 2 || s.Messages[0].Parts[0].Text != "edited" {
			t.Fatalf("parts = %+v, want prt_1 edited in place", s.Messages[0].Parts)
		}
	})

	t.Run("message.part.delta appends", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeMessagePartDelta, protocol.MessagePartDeltaProps{
			SessionID: "ses_1", MessageID: "msg_1", PartID: "prt_1",
			Field: "text", Delta: "!!",
		}))
		if s.Messages[0].Parts[0].Text != "hi!!" {
			t.Fatalf("text = %q, want hi!!", s.Messages[0].Parts[0].Text)
		}
	})

	t.Run("message.removed drops the message", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeMessageRemoved, protocol.MessageRemovedProps{
			SessionID: "ses_1", MessageID: "msg_1",
		}))
		if len(s.Messages) != 0 {
			t.Fatalf("messages = %+v", s.Messages)
		}
	})

	t.Run("message.part.removed drops the part", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeMessagePartRemoved, protocol.MessagePartRemovedProps{
			SessionID: "ses_1", MessageID: "msg_1", PartID: "prt_1",
		}))
		if len(s.Messages) != 1 || len(s.Messages[0].Parts) != 0 {
			t.Fatalf("messages = %+v", s.Messages)
		}
	})

	t.Run("session.updated updates list and current", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeSessionUpdated, protocol.SessionUpdatedProps{
			SessionID: "ses_1",
			Info:      protocol.Session{ID: "ses_1", Title: "Renamed", ProjectID: "prj_1", Directory: "/d"},
		}))
		if s.Sessions[0].Title != "Renamed" || s.Current.Title != "Renamed" {
			t.Fatalf("sessions = %+v current = %+v", s.Sessions, s.Current)
		}
	})

	t.Run("session.deleted clears list and current", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeSessionDeleted, protocol.SessionDeletedProps{
			SessionID: "ses_1",
			Info:      protocol.Session{ID: "ses_1", Title: "T"},
		}))
		if len(s.Sessions) != 0 || s.Current != nil {
			t.Fatalf("sessions = %+v current = %+v", s.Sessions, s.Current)
		}
	})

	t.Run("session.status updates current session status", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
			SessionID: "ses_1",
			Status:    protocol.SessionStatus{Type: "busy"},
		}))
		if s.Status.Type != "busy" {
			t.Fatalf("status = %+v", s.Status)
		}
	})

	t.Run("permission asked then replied", func(t *testing.T) {
		s := seed(t)
		s.Apply(ev(t, protocol.EventTypePermissionAsked, protocol.PermissionAskedProps{
			ID: "perm_1", SessionID: "ses_1", Permission: "edit",
		}))
		if len(s.Pending) != 1 || s.Pending[0].ID != "perm_1" || s.Pending[0].Permission != "edit" {
			t.Fatalf("pending = %+v", s.Pending)
		}
		s.Apply(ev(t, protocol.EventTypePermissionReplied, protocol.PermissionRepliedProps{
			SessionID: "ses_1", RequestID: "perm_1", Reply: "once",
		}))
		if len(s.Pending) != 0 {
			t.Fatalf("pending = %+v, want empty after reply", s.Pending)
		}
	})
}

func TestApplyIgnoresOtherSessions(t *testing.T) {
	s := seed(t)
	s.Apply(ev(t, protocol.EventTypeMessageUpdated, protocol.MessageUpdatedProps{
		SessionID: "ses_9",
		Info:      protocol.Message{ID: "msg_9", SessionID: "ses_9", Role: "user"},
	}))
	if len(s.Messages) != 1 || s.Messages[0].Info.ID != "msg_1" {
		t.Fatalf("foreign message leaked: %+v", s.Messages)
	}
	s.Apply(ev(t, protocol.EventTypeSessionStatus, protocol.SessionStatusProps{
		SessionID: "ses_9",
		Status:    protocol.SessionStatus{Type: "busy"},
	}))
	if s.Status.Type != "" {
		t.Fatalf("foreign status leaked: %+v", s.Status)
	}
	// a session.updated for the current session updates the list title
	s.Apply(ev(t, protocol.EventTypeSessionUpdated, protocol.SessionUpdatedProps{
		SessionID: "ses_1",
		Info:      protocol.Session{ID: "ses_1", Title: "New Title", ProjectID: "prj_1", Directory: "/d"},
	}))
	if s.Sessions[0].Title != "New Title" || s.Current.Title != "New Title" {
		t.Fatalf("list = %+v current = %+v", s.Sessions, s.Current)
	}
}
