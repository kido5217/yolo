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

// scopedSession resolves {id} within the request's directory scope; unknown
// or out-of-scope ids are 404 (M5).
func (s *Server) scopedSession(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	dir, ok := s.scoped(w, r)
	if !ok {
		return "", false
	}
	row, err := s.DB.GetSession(id)
	if errors.Is(err, storage.ErrNotFound) {
		envelope(w, http.StatusNotFound, "session not found", nil)
		return "", false
	}
	if err != nil {
		envelope(w, http.StatusInternalServerError, "lookup session", nil)
		return "", false
	}
	if row.ProjectDir != dir {
		envelope(w, http.StatusNotFound, "session not found", nil)
		return "", false
	}
	return id, true
}

func (s *Server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSessions(dir, 0)
	if err != nil {
		envelope(w, http.StatusInternalServerError, "list sessions", nil)
		return
	}
	out := make([]protocol.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, storage.SessionFromRow(row, nil))
	}
	writeJSON(w, http.StatusOK, out)
}

// newSession inserts a default session in dir; blanks take the defaults
// ("New session" / "build" / catalog default model).
func (s *Server) newSession(dir, title, agent, model string) (protocol.Session, error) {
	if title == "" {
		title = "New session"
	}
	if agent == "" {
		agent = "build"
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
	if err := s.DB.CreateSession(row); err != nil {
		return protocol.Session{}, err
	}
	return s.DB.Session(row.ID)
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
	if err := decode(r, &in); err != nil {
		envelope(w, http.StatusBadRequest, "invalid body", nil)
		return
	}
	ses, err := s.newSession(dir, in.Title, in.Agent, in.Model)
	if err != nil {
		envelope(w, http.StatusInternalServerError, "create session", nil)
		return
	}
	writeJSON(w, http.StatusCreated, ses)
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	id, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	ses, err := s.DB.Session(id)
	if err != nil {
		envelope(w, http.StatusInternalServerError, "lookup session", nil)
		return
	}
	writeJSON(w, http.StatusOK, ses)
}

func (s *Server) handleSessionPatch(w http.ResponseWriter, r *http.Request) {
	id, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	var in struct {
		Title *string `json:"title"`
		Agent *string `json:"agent"`
		Model *string `json:"model"`
		Time  *int64  `json:"time"`
	}
	if err := decode(r, &in); err != nil {
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
	if err := s.DB.UpdateSession(id, patch); err != nil && !errors.Is(err, storage.ErrNotFound) {
		envelope(w, http.StatusInternalServerError, "update session", nil)
		return
	}
	ses, err := s.DB.Session(id)
	if err != nil {
		envelope(w, http.StatusNotFound, "session not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, ses)
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	ses, err := s.DB.Session(id)
	if err != nil {
		envelope(w, http.StatusInternalServerError, "lookup session", nil)
		return
	}
	if err := s.DB.DeleteSession(id); err != nil {
		envelope(w, http.StatusInternalServerError, "delete session", nil)
		return
	}
	s.Engine.Close(id)
	s.emit(protocol.EventTypeSessionDeleted, protocol.SessionDeletedProps{SessionID: id, Info: ses})
	w.WriteHeader(http.StatusNoContent)
}

// messageWire maps a message row to the wire message (parts go separately).
func messageWire(m storage.MessageRow) protocol.Message {
	out := protocol.Message{
		ID: m.ID, SessionID: m.SessionID, Role: m.Role, Agent: m.Agent,
		Cost: m.Cost, Tokens: &m.Tokens,
		Time: protocol.MessageTime{Created: m.TimeCreated},
	}
	if m.TimeCompleted != nil {
		out.Time.Completed = *m.TimeCompleted
	}
	return out
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	id, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListMessages(id)
	if err != nil {
		envelope(w, http.StatusInternalServerError, "list messages", nil)
		return
	}
	out := make([]protocol.MessageWithParts, 0, len(rows))
	for _, m := range rows {
		partRows, err := s.DB.ListParts(m.ID)
		if err != nil {
			envelope(w, http.StatusInternalServerError, "list parts", nil)
			return
		}
		mp := protocol.MessageWithParts{Info: messageWire(m), Parts: make([]protocol.Part, 0, len(partRows))}
		for _, p := range partRows {
			wire, err := storage.PartToProtocol(p)
			if err != nil {
				envelope(w, http.StatusInternalServerError, "decode part", nil)
				return
			}
			mp.Parts = append(mp.Parts, wire)
		}
		out = append(out, mp)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	id, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	var in struct {
		Text string `json:"text"`
	}
	if err := decode(r, &in); err != nil {
		envelope(w, http.StatusBadRequest, "invalid body", nil)
		return
	}
	if strings.TrimSpace(in.Text) == "" {
		envelope(w, http.StatusBadRequest, "empty message", nil)
		return
	}
	res, err := s.Engine.Send(context.Background(), id, in.Text, func(error) {})
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, map[string]string{"message_id": res.MessageID})
	case errors.Is(err, session.ErrSessionBusy):
		envelope(w, http.StatusConflict, "session busy", nil)
	case errors.Is(err, storage.ErrNotFound):
		envelope(w, http.StatusNotFound, "session not found", nil)
	default:
		envelope(w, http.StatusInternalServerError, "start turn", nil)
	}
}

func (s *Server) handleAbort(w http.ResponseWriter, r *http.Request) {
	id, ok := s.scopedSession(w, r)
	if !ok {
		return
	}
	aborted := s.Engine.Abort(id)
	// settle until the turn reports idle so status reads agree right away
	deadline := time.Now().Add(2 * time.Second)
	for s.Engine.Status(id) != protocol.StatusIdle && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"aborted": aborted})
}

func (s *Server) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSessions(dir, 0)
	if err != nil {
		envelope(w, http.StatusInternalServerError, "list sessions", nil)
		return
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.ID] = s.Engine.Status(row.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.scopedSession(w, r); !ok {
		return
	}
	var in struct {
		Command string `json:"command"`
	}
	if err := decode(r, &in); err != nil {
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
		dir, ok := s.scoped(w, r)
		if !ok {
			return
		}
		ses, err := s.newSession(dir, "", "", "")
		if err != nil {
			envelope(w, http.StatusInternalServerError, "create session", nil)
			return
		}
		s.emit(protocol.EventTypeSessionUpdated, protocol.SessionUpdatedProps{SessionID: ses.ID, Info: ses})
		writeJSON(w, http.StatusOK, protocol.CommandResponse{SessionID: ses.ID})
		return
	}
	writeJSON(w, http.StatusOK, protocol.CommandResponse{Handled: "client"})
}

// emit publishes a bus event, dropping marshal failures (never block the API).
func (s *Server) emit(t string, props any) {
	ev, err := protocol.MakeEvent(t, props)
	if err != nil {
		return
	}
	s.Bus.Publish(ev)
}
