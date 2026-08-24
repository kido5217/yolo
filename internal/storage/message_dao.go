package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kido5217/yolo/internal/protocol"
)

// MessageRow is one stored message; Tokens is the JSON in message.tokens.
type MessageRow struct {
	ID, SessionID, Role, Agent string
	Cost                       float64
	Tokens                     protocol.Tokens
	TimeCreated                int64
	TimeCompleted              *int64
}

// CreateMessage inserts a message row.
func (d *DB) CreateMessage(ctx context.Context, r MessageRow) error {
	tok, err := json.Marshal(r.Tokens)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx,
		`INSERT INTO message (id, session_id, role, agent, cost, tokens, `+
			`time_created, time_completed) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.SessionID, r.Role, agentOrDefault(r.Agent), r.Cost, string(tok), r.TimeCreated, nullPtr(r.TimeCompleted))
	return err
}

// UpdateMessage rewrites a message row.
func (d *DB) UpdateMessage(ctx context.Context, r MessageRow) error {
	tok, err := json.Marshal(r.Tokens)
	if err != nil {
		return err
	}
	res, err := d.ExecContext(ctx,
		`UPDATE message SET session_id=?, role=?, agent=?, cost=?, `+
			`tokens=?, time_created=?, time_completed=? WHERE id=?`,
		r.SessionID, r.Role, agentOrDefault(r.Agent), r.Cost, string(tok), r.TimeCreated, nullPtr(r.TimeCompleted), r.ID)
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
		`SELECT id, session_id, role, agent, cost, tokens, time_created, `+
			`time_completed FROM message WHERE session_id=? `+
			`ORDER BY time_created ASC, rowid ASC`, sessionID)
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
