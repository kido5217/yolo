package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kido5217/yolo/internal/protocol"
)

// MessageRow is one stored message; Tokens is the JSON in message.tokens,
// Error the JSON in message.error_json (nil when the turn did not fail).
type MessageRow struct {
	ID, SessionID, Role, Agent string
	Cost                       float64
	Tokens                     protocol.Tokens
	Error                      *protocol.MessageError
	TimeCreated                int64
	TimeCompleted              *int64
}

// errorJSON marshals the message error for the error_json column (NULL
// when unset, mirroring the tokens column).
func errorJSON(e *protocol.MessageError) (any, error) {
	if e == nil {
		return nil, nil
	}
	b, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// scanMessageError decodes the error_json column (NULL/empty -> nil).
func scanMessageError(id string, er sql.NullString) (*protocol.MessageError, error) {
	if !er.Valid || er.String == "" {
		return nil, nil
	}
	var e protocol.MessageError
	if err := json.Unmarshal([]byte(er.String), &e); err != nil {
		return nil, fmt.Errorf("message %s error: %w", id, err)
	}
	return &e, nil
}

// CreateMessage inserts a message row.
func (d *DB) CreateMessage(ctx context.Context, r MessageRow) error {
	tok, err := json.Marshal(r.Tokens)
	if err != nil {
		return err
	}
	er, err := errorJSON(r.Error)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx,
		`INSERT INTO message (id, session_id, role, agent, cost, tokens, `+
			`error_json, time_created, time_completed) VALUES (?,?,?,?,?,?,?,?,?)`,
		r.ID, r.SessionID, r.Role, agentOrDefault(r.Agent), r.Cost, string(tok), er, r.TimeCreated, nullPtr(r.TimeCompleted))
	return err
}

// UpdateMessage rewrites a message row.
func (d *DB) UpdateMessage(ctx context.Context, r MessageRow) error {
	tok, err := json.Marshal(r.Tokens)
	if err != nil {
		return err
	}
	er, err := errorJSON(r.Error)
	if err != nil {
		return err
	}
	res, err := d.ExecContext(ctx,
		`UPDATE message SET session_id=?, role=?, agent=?, cost=?, `+
			`tokens=?, error_json=?, time_created=?, time_completed=? WHERE id=?`,
		r.SessionID, r.Role, agentOrDefault(r.Agent), r.Cost, string(tok), er, r.TimeCreated, nullPtr(r.TimeCompleted), r.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetMessage fetches one message; missing id -> ErrNotFound.
func (d *DB) GetMessage(ctx context.Context, id string) (MessageRow, error) {
	var r MessageRow
	var tok string
	var er sql.NullString
	var tc sql.NullInt64
	err := d.QueryRowContext(ctx,
		`SELECT id, session_id, role, agent, cost, tokens, error_json, `+
			`time_created, time_completed FROM message WHERE id=?`, id).
		Scan(&r.ID, &r.SessionID, &r.Role, &r.Agent, &r.Cost, &tok, &er, &r.TimeCreated, &tc)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	if tc.Valid {
		v := tc.Int64
		r.TimeCompleted = &v
	}
	if tok == "" {
		tok = "{}"
	}
	if err := json.Unmarshal([]byte(tok), &r.Tokens); err != nil {
		return r, fmt.Errorf("message %s tokens: %w", r.ID, err)
	}
	r.Error, err = scanMessageError(r.ID, er)
	return r, err
}

// SetMessageError writes the message's terminal error (error_json); missing
// id -> ErrNotFound.
func (d *DB) SetMessageError(ctx context.Context, id string, e protocol.MessageError) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	res, err := d.ExecContext(ctx, `UPDATE message SET error_json=? WHERE id=?`, string(b), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMessage removes a message (parts cascade via FK).
func (d *DB) DeleteMessage(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM message WHERE id=?`, id)
	return err
}

// ListMessages lists a session's messages, earliest first.
func (d *DB) ListMessages(ctx context.Context, sessionID string) ([]MessageRow, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, session_id, role, agent, cost, tokens, error_json, `+
			`time_created, time_completed FROM message WHERE session_id=? `+
			`ORDER BY time_created ASC, rowid ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MessageRow{}
	for rows.Next() {
		var r MessageRow
		var tok string
		var er sql.NullString
		var tc sql.NullInt64
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Agent, &r.Cost, &tok, &er, &r.TimeCreated, &tc); err != nil {
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
		if r.Error, err = scanMessageError(r.ID, er); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
