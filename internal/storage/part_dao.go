package storage

import (
	"context"
	"database/sql"
	"errors"
)

// PartRow is one stored part; StateJSON per PartToProtocol/ProtocolToPart.
type PartRow struct {
	ID, MessageID, SessionID, Type, Tool string
	StateJSON                            string
	TimeCreated                          int64
}

// UpsertPart inserts or updates a part row by id.
func (d *DB) UpsertPart(ctx context.Context, r PartRow) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO part (id, message_id, session_id, type, tool, state_json, time_created) VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   message_id=excluded.message_id, session_id=excluded.session_id,
		   type=excluded.type, tool=excluded.tool, state_json=excluded.state_json,
		   time_created=excluded.time_created`,
		r.ID, r.MessageID, r.SessionID, r.Type, nullStr(r.Tool), r.StateJSON, r.TimeCreated)
	return err
}

// GetPart fetches one part; missing id -> ErrNotFound.
func (d *DB) GetPart(ctx context.Context, id string) (PartRow, error) {
	var r PartRow
	var tool sql.NullString
	err := d.QueryRowContext(ctx,
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
func (d *DB) ListParts(ctx context.Context, messageID string) ([]PartRow, error) {
	return d.listPartsBy(ctx, messageID, "")
}

// ListToolParts lists a message's tool parts, earliest first.
func (d *DB) ListToolParts(ctx context.Context, messageID string) ([]PartRow, error) {
	return d.listPartsBy(ctx, messageID, "tool")
}

func (d *DB) listPartsBy(ctx context.Context, messageID, typ string) ([]PartRow, error) {
	q := `SELECT id, message_id, session_id, type, tool, state_json, time_created FROM part WHERE message_id=?`
	args := []any{messageID}
	if typ != "" {
		q += ` AND type=?`
		args = append(args, typ)
	}
	q += ` ORDER BY time_created ASC, rowid ASC`
	rows, err := d.QueryContext(ctx, q, args...)
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
