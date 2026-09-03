package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

// partQueryBatch bounds the message ids per IN (...) query in
// ListPartsByMessageIDs. SQLite caps a statement's bound variables (32766 in
// modernc.org/sqlite v1.57.0); batching keeps an arbitrarily large message
// count within the cap instead of failing it.
const partQueryBatch = 500

// ListPartsByMessageIDs fetches the parts of every given message (batched
// N+1 replacement for per-message ListParts calls), running one
// parameterized IN (...) query per partQueryBatch chunk so an unbounded
// message count never trips SQLite's bound-variable cap. Each message's
// parts come back earliest first — time_created ASC, rowid ASC, exactly
// ListParts' per-message order (a message's ids live in a single chunk);
// unknown ids simply contribute no rows. An empty input returns an empty
// slice, not an error.
func (d *DB) ListPartsByMessageIDs(ctx context.Context, messageIDs []string) ([]PartRow, error) {
	if len(messageIDs) == 0 {
		return []PartRow{}, nil
	}
	out := make([]PartRow, 0, len(messageIDs))
	for start := 0; start < len(messageIDs); start += partQueryBatch {
		chunk, err := d.queryPartsByIDs(ctx, messageIDs[start:min(start+partQueryBatch, len(messageIDs))])
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
	}
	return out, nil
}

// queryPartsByIDs runs one parameterized IN (...) query for a batch of
// message ids, returning their parts in message_id, time_created, rowid order.
func (d *DB) queryPartsByIDs(ctx context.Context, messageIDs []string) ([]PartRow, error) {
	q := `SELECT id, message_id, session_id, type, tool, state_json, time_created FROM part WHERE message_id IN (` +
		strings.Repeat(`?,`, len(messageIDs)-1) + `?) ORDER BY message_id ASC, time_created ASC, rowid ASC`
	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		args[i] = id
	}
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
