package server

import (
	"errors"
	"net/http"

	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
)

// handlePermissionList returns pending asks for every session in the request
// directory (M5: /permission is scoped).
func (s *Server) handlePermissionList(w http.ResponseWriter, r *http.Request) {
	dir, ok := s.scoped(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSessions(r.Context(), dir, 0)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "list sessions", err)
		return
	}
	out := make([]protocol.PermissionAskedProps, 0)
	for _, row := range rows {
		reqs, err := s.Perm.Pending(r.Context(), row.ID)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, "list permissions", err)
			return
		}
		for _, q := range reqs {
			out = append(out, askedProps(q))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func askedProps(q permission.Request) protocol.PermissionAskedProps {
	meta := make(map[string]any, len(q.Meta)+2)
	for k, v := range q.Meta {
		meta[k] = v
	}
	if q.Tool != "" {
		meta["tool"] = q.Tool
	}
	if q.Agent != "" {
		meta["agent"] = q.Agent
	}
	return protocol.PermissionAskedProps{
		ID:         q.RequestID,
		SessionID:  q.SessionID,
		Permission: q.Permission,
		Patterns:   q.Resources,
		Always:     q.Always,
		Metadata:   meta,
	}
}

// handlePermissionReply answers a parked ask; the body is validated before
// the unknown-id lookup (LOCKED: bad response is 400, not 404).
func (s *Server) handlePermissionReply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Response string `json:"response"`
	}
	if err := decode(r, &body); err != nil {
		envelope(w, http.StatusBadRequest, "invalid reply", nil)
		return
	}
	switch body.Response {
	case "once", "always", "reject":
	default:
		envelope(w, http.StatusBadRequest, "invalid response", nil)
		return
	}
	id := r.PathValue("requestID")
	if err := s.Perm.Reply(r.Context(), id, body.Response); err != nil {
		if errors.Is(err, permission.ErrNoPending) {
			envelope(w, http.StatusNotFound, "no pending permission request", nil)
			return
		}
		s.fail(w, http.StatusInternalServerError, "reply permission", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
