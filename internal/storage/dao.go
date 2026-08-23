package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kido5217/yolo/internal/protocol"
)

// SessionRow is one stored session.
type SessionRow struct {
	ID, ProjectDir, Title, Model, Agent string
	Cost                                float64
	TimeCreated, TimeUpdated            int64
}

// MessageRow is one stored message; Tokens is the JSON in message.tokens.
type MessageRow struct {
	ID, SessionID, Role, Agent string
	Cost                       float64
	Tokens                     protocol.Tokens
	TimeCreated                int64
	TimeCompleted              *int64
}

// agentOrDefault normalizes the per-message agent ("build" when unset).
func agentOrDefault(a string) string {
	if a == "" {
		return "build"
	}
	return a
}

// PartRow is one stored part; StateJSON per PartToProtocol/ProtocolToPart.
type PartRow struct {
	ID, MessageID, SessionID, Type, Tool string
	StateJSON                            string
	TimeCreated                          int64
}

// PermissionRow is one stored permission request.
type PermissionRow struct {
	RequestID, SessionID, Action, Resource, Response, AlwaysJSON string
	TimeCreated                                                  int64
}

// nullStr renders "" as SQL NULL for nullable text columns.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullPtr renders nil as SQL NULL for nullable integer columns.
func nullPtr(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// CreateSession inserts a session row; an empty agent takes the column
// default "build" (agentOrDefault), mirroring the message side.
func (d *DB) CreateSession(r SessionRow) error {
	_, err := d.Exec(
		`INSERT INTO session (id, project_dir, title, model, agent, cost, `+
			`time_created, time_updated) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.ProjectDir, r.Title, r.Model, agentOrDefault(r.Agent), r.Cost, r.TimeCreated, r.TimeUpdated)
	return err
}

// GetSession fetches one session; missing id -> ErrNotFound.
func (d *DB) GetSession(id string) (SessionRow, error) {
	var r SessionRow
	err := d.QueryRow(
		`SELECT id, project_dir, title, model, agent, cost, time_created, time_updated FROM session WHERE id=?`, id).
		Scan(&r.ID, &r.ProjectDir, &r.Title, &r.Model, &r.Agent, &r.Cost, &r.TimeCreated, &r.TimeUpdated)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	return r, nil
}

// ListSessions lists a project directory's sessions, newest (time_updated) first.
func (d *DB) ListSessions(projectDir string, limit int) ([]SessionRow, error) {
	q := `SELECT id, project_dir, title, model, agent, cost, time_created, ` +
		`time_updated FROM session WHERE project_dir=? ORDER BY time_updated DESC`
	args := []any{projectDir}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionRow{}
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(
			&r.ID, &r.ProjectDir, &r.Title, &r.Model, &r.Agent, &r.Cost,
			&r.TimeCreated, &r.TimeUpdated,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateSession patches a session; zero-valued fields are left untouched.
func (d *DB) UpdateSession(id string, patch SessionRow) error {
	sets := []string{}
	args := []any{}
	if patch.ProjectDir != "" {
		sets = append(sets, `project_dir=?`)
		args = append(args, patch.ProjectDir)
	}
	if patch.Title != "" {
		sets = append(sets, `title=?`)
		args = append(args, patch.Title)
	}
	if patch.Model != "" {
		sets = append(sets, `model=?`)
		args = append(args, patch.Model)
	}
	if patch.Agent != "" {
		sets = append(sets, `agent=?`)
		args = append(args, patch.Agent)
	}
	if patch.Cost != 0 {
		sets = append(sets, `cost=?`)
		args = append(args, patch.Cost)
	}
	if patch.TimeUpdated != 0 {
		sets = append(sets, `time_updated=?`)
		args = append(args, patch.TimeUpdated)
	}
	if len(sets) == 0 {
		return nil
	}
	query := `UPDATE session SET ` + strings.Join(sets, ",") + ` WHERE id=?`
	args = append(args, id)
	res, err := d.Exec(query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSession removes a session (messages/parts cascade via FK).
func (d *DB) DeleteSession(id string) error {
	_, err := d.Exec(`DELETE FROM session WHERE id=?`, id)
	return err
}

// CreateMessage inserts a message row.
func (d *DB) CreateMessage(r MessageRow) error {
	tok, err := json.Marshal(r.Tokens)
	if err != nil {
		return err
	}
	_, err = d.Exec(
		`INSERT INTO message (id, session_id, role, agent, cost, tokens, `+
			`time_created, time_completed) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.SessionID, r.Role, agentOrDefault(r.Agent), r.Cost, string(tok), r.TimeCreated, nullPtr(r.TimeCompleted))
	return err
}

// UpdateMessage rewrites a message row.
func (d *DB) UpdateMessage(r MessageRow) error {
	tok, err := json.Marshal(r.Tokens)
	if err != nil {
		return err
	}
	res, err := d.Exec(
		`UPDATE message SET session_id=?, role=?, agent=?, cost=?, `+
			`tokens=?, time_created=?, time_completed=? WHERE id=?`,
		r.SessionID, r.Role, agentOrDefault(r.Agent), r.Cost, string(tok), r.TimeCreated, nullPtr(r.TimeCompleted), r.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMessage removes a message (parts cascade via FK).
func (d *DB) DeleteMessage(id string) error {
	_, err := d.Exec(`DELETE FROM message WHERE id=?`, id)
	return err
}

// ListMessages lists a session's messages, earliest first.
func (d *DB) ListMessages(sessionID string) ([]MessageRow, error) {
	rows, err := d.Query(
		`SELECT id, session_id, role, agent, cost, tokens, time_created, `+
			`time_completed FROM message WHERE session_id=? `+
			`ORDER BY time_created ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MessageRow{}
	for rows.Next() {
		var r MessageRow
		var tok string
		var tc sql.NullInt64
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Agent, &r.Cost, &tok, &r.TimeCreated, &tc); err != nil {
			return nil, err
		}
		if tc.Valid {
			v := tc.Int64
			r.TimeCompleted = &v
		}
		if tok == "" {
			tok = "{}"
		}
		if err := json.Unmarshal([]byte(tok), &r.Tokens); err != nil {
			return nil, fmt.Errorf("message %s tokens: %w", r.ID, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertPart inserts or updates a part row by id.
func (d *DB) UpsertPart(r PartRow) error {
	_, err := d.Exec(
		`INSERT INTO part (id, message_id, session_id, type, tool, state_json, time_created) VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   message_id=excluded.message_id, session_id=excluded.session_id,
		   type=excluded.type, tool=excluded.tool, state_json=excluded.state_json,
		   time_created=excluded.time_created`,
		r.ID, r.MessageID, r.SessionID, r.Type, nullStr(r.Tool), r.StateJSON, r.TimeCreated)
	return err
}

// GetPart fetches one part; missing id -> ErrNotFound.
func (d *DB) GetPart(id string) (PartRow, error) {
	var r PartRow
	var tool sql.NullString
	err := d.QueryRow(
		`SELECT id, message_id, session_id, type, tool, state_json, time_created FROM part WHERE id=?`, id).
		Scan(&r.ID, &r.MessageID, &r.SessionID, &r.Type, &tool, &r.StateJSON, &r.TimeCreated)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.Tool = tool.String
	return r, nil
}

// ListParts lists a message's parts, earliest first.
func (d *DB) ListParts(messageID string) ([]PartRow, error) {
	return d.listPartsBy(messageID, "")
}

// ListToolParts lists a message's tool parts, earliest first.
func (d *DB) ListToolParts(messageID string) ([]PartRow, error) {
	return d.listPartsBy(messageID, "tool")
}

func (d *DB) listPartsBy(messageID, typ string) ([]PartRow, error) {
	q := `SELECT id, message_id, session_id, type, tool, state_json, time_created FROM part WHERE message_id=?`
	args := []any{messageID}
	if typ != "" {
		q += ` AND type=?`
		args = append(args, typ)
	}
	q += ` ORDER BY time_created ASC`
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PartRow{}
	for rows.Next() {
		var r PartRow
		var tool sql.NullString
		if err := rows.Scan(&r.ID, &r.MessageID, &r.SessionID, &r.Type, &tool, &r.StateJSON, &r.TimeCreated); err != nil {
			return nil, err
		}
		r.Tool = tool.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// ProtocolToPart encodes a wire part into a row. Text/reasoning parts store
// {"text":..., "end":n, "synthetic":true} (end/synthetic omitted when unset);
// tool parts store the full protocol.ToolState JSON. CallID is transient and
// not persisted. A marshal failure (e.g. NaN in a tool state) is an error —
// persisting "" would 500 every later read.
func ProtocolToPart(p protocol.Part) (PartRow, error) {
	r := PartRow{
		ID:          p.ID,
		MessageID:   p.MessageID,
		SessionID:   p.SessionID,
		Type:        p.Type,
		Tool:        p.Tool,
		TimeCreated: p.Time.Start,
	}
	switch {
	case p.State != nil:
		b, err := json.Marshal(p.State)
		if err != nil {
			return PartRow{}, fmt.Errorf("part %s state: %w", p.ID, err)
		}
		r.StateJSON = string(b)
	default:
		// Hot path (streamed deltas): build the fixed 3-key document
		// directly. Must stay byte-identical to the map marshal: sorted
		// keys (end, synthetic, text), compact separators.
		t, err := json.Marshal(p.Text)
		if err != nil {
			return PartRow{}, fmt.Errorf("part %s text: %w", p.ID, err)
		}
		b := make([]byte, 0, len(t)+16)
		b = append(b, '{')
		if p.Time.End != 0 {
			b = append(b, `"end":`...)
			b = strconv.AppendInt(b, p.Time.End, 10)
			b = append(b, ',')
		}
		if p.Synthetic != nil && *p.Synthetic {
			b = append(b, `"synthetic":true,`...)
		}
		b = append(b, `"text":`...)
		b = append(b, t...)
		b = append(b, '}')
		r.StateJSON = string(b)
	}
	return r, nil
}

// PartToProtocol decodes a row into a wire part (inverse of ProtocolToPart).
func PartToProtocol(r PartRow) (protocol.Part, error) {
	p := protocol.Part{
		ID:        r.ID,
		SessionID: r.SessionID,
		MessageID: r.MessageID,
		Type:      r.Type,
		Tool:      r.Tool,
	}
	switch r.Type {
	case "tool":
		st := &protocol.ToolState{}
		if err := json.Unmarshal([]byte(r.StateJSON), st); err != nil {
			return p, fmt.Errorf("part %s state: %w", r.ID, err)
		}
		p.State = st
	default:
		var st struct {
			Text      string `json:"text"`
			End       int64  `json:"end"`
			Synthetic *bool  `json:"synthetic"`
		}
		if err := json.Unmarshal([]byte(r.StateJSON), &st); err != nil {
			return p, fmt.Errorf("part %s state: %w", r.ID, err)
		}
		p.Text = st.Text
		p.Time = protocol.PartTime{Start: r.TimeCreated, End: st.End}
		p.Synthetic = st.Synthetic
	}
	return p, nil
}

// SessionFromRow assembles the wire session, recomputing cost/tokens as the
// sum over assistant messages (session.cost column is ignored by design).
func SessionFromRow(r SessionRow, msgs []MessageRow) protocol.Session {
	var cost float64
	var tok protocol.Tokens
	for _, m := range msgs {
		if m.Role != "assistant" {
			continue
		}
		cost += m.Cost
		tok.Input += m.Tokens.Input
		tok.Output += m.Tokens.Output
		tok.Reasoning += m.Tokens.Reasoning
		tok.Cache.Read += m.Tokens.Cache.Read
		tok.Cache.Write += m.Tokens.Cache.Write
	}
	return protocol.Session{
		ID:        r.ID,
		ProjectID: projectIDFromDir(r.ProjectDir),
		Directory: r.ProjectDir,
		Title:     r.Title,
		Agent:     r.Agent,
		Model:     modelRefFromString(r.Model),
		Cost:      cost,
		Tokens:    tok,
		Version:   "yolo",
		Time:      protocol.SessionTime{Created: r.TimeCreated, Updated: r.TimeUpdated},
	}
}

// SavePermission inserts or updates a permission request by request_id.
func (d *DB) SavePermission(r PermissionRow) error {
	_, err := d.Exec(
		`INSERT INTO permission (request_id, session_id, action, resource, `+
			`response, always_json, time_created) VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(request_id) DO UPDATE SET response=excluded.response, always_json=excluded.always_json`,
		r.RequestID, r.SessionID, r.Action, r.Resource, nullStr(r.Response), nullStr(r.AlwaysJSON), r.TimeCreated)
	return err
}

// ListPermissions lists a session's permission requests; pendingOnly filters
// to rows with no response yet.
func (d *DB) ListPermissions(sessionID string, pendingOnly bool) ([]PermissionRow, error) {
	q := `SELECT request_id, session_id, action, resource, response, ` +
		`always_json, time_created FROM permission WHERE session_id=?`
	if pendingOnly {
		q += ` AND response IS NULL`
	}
	q += ` ORDER BY time_created ASC`
	rows, err := d.Query(q, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PermissionRow{}
	for rows.Next() {
		var r PermissionRow
		var resp, always sql.NullString
		if err := rows.Scan(&r.RequestID, &r.SessionID, &r.Action, &r.Resource, &resp, &always, &r.TimeCreated); err != nil {
			return nil, err
		}
		r.Response = resp.String
		r.AlwaysJSON = always.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplyPermission records a response for a request; unknown id -> ErrNotFound.
func (d *DB) ReplyPermission(requestID, response string) error {
	res, err := d.Exec(`UPDATE permission SET response=? WHERE request_id=?`, response, requestID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SaveTodos replaces a session's todo list wholesale: delete then insert in
// order, position = index. An empty list clears the session's todos.
func (d *DB) SaveTodos(sessionID string, todos []protocol.Todo) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM todo WHERE session_id=?`, sessionID); err != nil {
		tx.Rollback()
		return err
	}
	for i, t := range todos {
		if _, err := tx.Exec(
			`INSERT INTO todo (session_id, content, status, priority, position) VALUES (?,?,?,?,?)`,
			sessionID, t.Content, t.Status, t.Priority, i); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetTodos lists a session's todos in stable position order.
func (d *DB) GetTodos(sessionID string) ([]protocol.Todo, error) {
	rows, err := d.Query(
		`SELECT content, status, priority FROM todo WHERE session_id=? ORDER BY position ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.Todo{}
	for rows.Next() {
		var t protocol.Todo
		if err := rows.Scan(&t.Content, &t.Status, &t.Priority); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AlwaysRules derives allow rules from response='always' rows: one rule per
// pattern in always_json, permission taken from the row's action.
func (d *DB) AlwaysRules(sessionID string) ([]protocol.Rule, error) {
	rows, err := d.Query(
		`SELECT action, always_json FROM permission WHERE session_id=? `+
			`AND response='always' ORDER BY time_created ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []protocol.Rule{}
	for rows.Next() {
		var action string
		var always sql.NullString
		if err := rows.Scan(&action, &always); err != nil {
			return nil, err
		}
		var patterns []string
		if always.Valid && always.String != "" {
			if err := json.Unmarshal([]byte(always.String), &patterns); err != nil {
				return nil, fmt.Errorf("always rules (session=%s, action=%s): %w", sessionID, action, err)
			}
		}
		for _, pat := range patterns {
			out = append(out, protocol.Rule{Permission: action, Pattern: pat, Action: protocol.ActionAllow})
		}
	}
	return out, rows.Err()
}
