package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kido5217/yolo/internal/protocol"
)

// SaveTodos replaces a session's todo list wholesale: delete then insert in
// order, position = index. An empty list clears the session's todos.
func (d *DB) SaveTodos(ctx context.Context, sessionID string, todos []protocol.Todo) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM todo WHERE session_id=?`, sessionID); err != nil {
		tx.Rollback()
		return err
	}
	for i, t := range todos {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO todo (session_id, content, status, priority, position) VALUES (?,?,?,?,?)`,
			sessionID, t.Content, t.Status, t.Priority, i); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetTodos lists a session's todos in stable position order.
func (d *DB) GetTodos(ctx context.Context, sessionID string) ([]protocol.Todo, error) {
	rows, err := d.QueryContext(ctx,
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
func (d *DB) AlwaysRules(ctx context.Context, sessionID string) ([]protocol.Rule, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT action, always_json FROM permission WHERE session_id=? `+
			`AND response='always' ORDER BY time_created ASC, rowid ASC`, sessionID)
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
