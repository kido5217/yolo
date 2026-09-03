package server

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/session"
	"github.com/kido5217/yolo/internal/storage"
)

// knownCommands are the TUI-handled commands the server answers with
// {"handled":"client"} (M5); /new creates a session server-side.
var knownCommands = map[string]bool{"/quit": true, "/exit": true, "/help": true, "/model": true, "/agents": true}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) scoped(w http.ResponseWriter, r *http.Request) (string, bool) {
	dir, err := s.scope(r)
	if err != nil {
		envelope(w, http.StatusBadRequest, err.Error(), nil)
		return "", false
	}
	return dir, true
}

func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"directory": dir})
}

func (s *Server) handleProjectCurrent(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":        storage.ProjectID(dir),
		"name":      filepath.Base(dir),
		"directory": dir,
	})
}

// scopedSession resolves {id} within the request's directory scope (unknown
// or out-of-scope ids are 404, M5) and returns the already-fetched row so
// callers don't re-read it behind a second lookup.
func (s *Server) scopedSession(w http.ResponseWriter, r *http.Request) (storage.SessionRow, bool) {
	id := r.PathValue("id")
	dir, ok := s.scoped(w, r)
	if !ok {
		return storage.SessionRow{}, false
	}
	row, err := s.DB.GetSession(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		envelope(w, http.StatusNotFound, "session not found", nil)
		return storage.SessionRow{}, false
	}
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "lookup session", err)
		return storage.SessionRow{}, false
	}
	if row.ProjectDir != dir {
		envelope(w, http.StatusNotFound, "session not found", nil)
		return storage.SessionRow{}, false
	}
	return row, true
}

func (s *Server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSessions(r.Context(), dir, 0)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list sessions", err)
		return
	}
	out := make([]protocol.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, storage.SessionFromRow(row, nil))
	}
	writeJSON(w, http.StatusOK, out)
}

// newSession inserts a default session in dir; blank title/model take the
// defaults ("New session" / catalog default model) and a blank agent takes
// the storage column default "build" (applied by CreateSession).
func (s *Server) newSession(ctx context.Context, dir, title, agent, model string) (protocol.Session, error) {
	if title == "" {
		title = "New session"
	}
	if model == "" {
		pid, mid := s.Prov.Default()
		model = pid + "/" + mid
	}
	now := time.Now().UnixMilli()
	row := storage.SessionRow{
		ID: protocol.NewID("ses"), ProjectDir: dir,
		Title: title, Agent: agent, Model: model,
		TimeCreated: now, TimeUpdated: now,
	}
	if err := s.DB.CreateSession(ctx, row); err != nil {
		return protocol.Session{}, err
	}
	return s.DB.Session(ctx, row.ID)
}

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	var in struct {
		Title string `json:"title"`
		Agent string `json:"agent"`
		Model string `json:"model"`
	}
	if err := decode(w, r, &in); err != nil {
		envelope(w, http.StatusBadRequest, "invalid body", nil)
		return
	}
	ses, err := s.newSession(r.Context(), dir, in.Title, in.Agent, in.Model)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "create session", err)
		return
	}
	writeJSON(w, http.StatusCreated, ses)
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	row, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	// DB.Session = GetSession + ListMessages; the row is the one
	// scopedSession already fetched, so only the message aggregation remains.
	msgs, err := s.DB.ListMessages(r.Context(), row.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "lookup session", err)
		return
	}
	writeJSON(w, http.StatusOK, storage.SessionFromRow(row, msgs))
}

func (s *Server) handleSessionPatch(w http.ResponseWriter, r *http.Request) {
	row, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	var in struct {
		Title *string `json:"title"`
		Agent *string `json:"agent"`
		Model *string `json:"model"`
		Time  *int64  `json:"time"`
	}
	if err := decode(w, r, &in); err != nil {
		envelope(w, http.StatusBadRequest, "invalid body", nil)
		return
	}
	patch := storage.SessionRow{}
	if in.Title != nil {
		patch.Title = *in.Title
	}
	if in.Agent != nil {
		patch.Agent = *in.Agent
	}
	if in.Model != nil {
		patch.Model = *in.Model
	}
	if in.Time != nil {
		patch.TimeUpdated = *in.Time
	}
	if err := s.DB.UpdateSession(r.Context(), row.ID, patch); err != nil && !errors.Is(err, storage.ErrNotFound) {
		s.fail(w, http.StatusInternalServerError, "update session", err)
		return
	}
	// The response reflects the POST-update state (cost/tokens aggregated
	// over messages), so the one remaining re-read is the updated row.
	ses, err := s.DB.Session(r.Context(), row.ID)
	if err != nil {
		envelope(w, http.StatusNotFound, "session not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, ses)
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	row, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	// The deleted event carries the pre-delete row (cost/tokens over the
	// still-present messages); the row is scopedSession's, no re-fetch.
	msgs, err := s.DB.ListMessages(r.Context(), row.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "lookup session", err)
		return
	}
	if err := s.DB.DeleteSession(r.Context(), row.ID); err != nil {
		s.fail(w, http.StatusInternalServerError, "delete session", err)
		return
	}
	s.Engine.Close(row.ID)
	s.emit(protocol.EventTypeSessionDeleted, protocol.SessionDeletedProps{
		SessionID: row.ID, Info: storage.SessionFromRow(row, msgs),
	})
	w.WriteHeader(http.StatusNoContent)
}

// messageWire maps a message row to the wire message (parts go separately).
func messageWire(m storage.MessageRow) protocol.Message {
	out := protocol.Message{
		ID: m.ID, SessionID: m.SessionID, Role: m.Role, Agent: m.Agent,
		Cost: m.Cost, Tokens: &m.Tokens, Error: m.Error,
		Time: protocol.MessageTime{Created: m.TimeCreated},
	}
	if m.TimeCompleted != nil {
		out.Time.Completed = *m.TimeCompleted
	}
	return out
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	row, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListMessages(r.Context(), row.ID)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list messages", err)
		return
	}
	// One batched query for every message's parts (was: one ListParts call
	// per message row — N+1 against local SQLite). The query orders by
	// message_id, time_created, rowid, so each message's slice is
	// earliest-first, byte-identical to the per-message ListParts order.
	messageIDs := make([]string, len(rows))
	for i, m := range rows {
		messageIDs[i] = m.ID
	}
	partRows, err := s.DB.ListPartsByMessageIDs(r.Context(), messageIDs)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list parts", err)
		return
	}
	partsByMessage := make(map[string][]storage.PartRow, len(rows))
	for _, p := range partRows {
		partsByMessage[p.MessageID] = append(partsByMessage[p.MessageID], p)
	}
	out := make([]protocol.MessageWithParts, 0, len(rows))
	for _, m := range rows {
		msgParts := partsByMessage[m.ID]
		mp := protocol.MessageWithParts{Info: messageWire(m), Parts: make([]protocol.Part, 0, len(msgParts))}
		for _, p := range msgParts {
			wire, err := storage.PartToProtocol(p)
			if err != nil {
				s.fail(w, http.StatusInternalServerError, "decode part", err)
				return
			}
			mp.Parts = append(mp.Parts, wire)
		}
		out = append(out, mp)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	row, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	id := row.ID
	var in struct {
		Text string `json:"text"`
	}
	if err := decode(w, r, &in); err != nil {
		envelope(w, http.StatusBadRequest, "invalid body", nil)
		return
	}
	if strings.TrimSpace(in.Text) == "" {
		envelope(w, http.StatusBadRequest, "empty message", nil)
		return
	}
	// The send boundary is the single log site for a turn's terminal
	// state: an aborted turn is user-initiated (info), a failed model
	// turn must not vanish (the 202 is already on its way and the TUI
	// stays idle-looking — upstream promptAsync logs it too).
	res, err := s.Engine.Send(context.Background(), id, in.Text, func(err error) {
		if err == nil {
			return
		}
		if errors.Is(err, context.Canceled) {
			s.Log.Info("turn aborted", "session_id", id)
			return
		}
		s.Log.Error("turn failed", "session_id", id, "error", err)
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, map[string]string{"message_id": res.MessageID})
	case errors.Is(err, session.ErrSessionBusy):
		envelope(w, http.StatusConflict, "session busy", nil)
	case errors.Is(err, storage.ErrNotFound):
		envelope(w, http.StatusNotFound, "session not found", nil)
	default:
		s.fail(w, http.StatusInternalServerError, "start turn", err)
	}
}

func (s *Server) handleAbort(w http.ResponseWriter, r *http.Request) {
	row, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	aborted := s.Engine.Abort(row.ID)
	// settle until the turn reports idle so status reads agree right away
	// (only when a running turn was aborted; an idle session publishes no
	// idle event and would stall the wait to its deadline)
	if aborted {
		s.settleIdle(r, row.ID)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"aborted": aborted})
}

// settleIdle waits (bounded to 2 s) for the session to report idle. A
// started turn always publishes an idle session.status on its deferred exit,
// so the wait is event-driven on the bus instead of a status poll; the
// per-iteration status re-check covers the idle published before the
// subscription. A client disconnect ends the wait early.
func (s *Server) settleIdle(r *http.Request, sessionID string) {
	events, done := s.Bus.Subscribe()
	defer done()
	deadline := time.After(2 * time.Second)
	for s.Engine.Status(sessionID) != protocol.SessionStatusIdle {
		select {
		case <-r.Context().Done():
			return
		case <-deadline:
			return
		case _, ok := <-events:
			if !ok {
				// the bus dropped this subscriber (overflow): the idle may be
				// unobservable — degrade to a bounded poll until the deadline
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

func (s *Server) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSessions(r.Context(), dir, 0)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list sessions", err)
		return
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.ID] = s.Engine.Status(row.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	row, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	var in struct {
		Command string `json:"command"`
	}
	if err := decode(w, r, &in); err != nil {
		envelope(w, http.StatusBadRequest, "invalid body", nil)
		return
	}
	fields := strings.Fields(in.Command)
	if len(fields) == 0 {
		envelope(w, http.StatusBadRequest, "empty command", nil)
		return
	}
	cmd := fields[0]
	if cmd != "/new" && !knownCommands[cmd] {
		envelope(w, http.StatusBadRequest, "unknown command "+cmd, nil)
		return
	}
	if cmd == "/new" {
		// scopedSession already checked row.ProjectDir == the resolved
		// directory, so no second s.scoped call is needed.
		ses, err := s.newSession(r.Context(), row.ProjectDir, "", "", "")
		if err != nil {
			s.fail(w, http.StatusInternalServerError, "create session", err)
			return
		}
		s.emit(protocol.EventTypeSessionUpdated, protocol.SessionUpdatedProps{SessionID: ses.ID, Info: ses})
		writeJSON(w, http.StatusOK, protocol.CommandResponse{SessionID: ses.ID})
		return
	}
	writeJSON(w, http.StatusOK, protocol.CommandResponse{Handled: "client"})
}

// emit publishes a bus event, never blocking the API. A marshal failure is an
// invariant violation for our own wire DTOs — log it, then drop.
func (s *Server) emit(t string, props any) {
	ev, err := protocol.MakeEvent(t, props)
	if err != nil {
		s.Log.Error("event emit failed", "type", t, "error", err)
		return
	}
	s.Bus.Publish(ev)
}
