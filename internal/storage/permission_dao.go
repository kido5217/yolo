package storage

import (
	"context"
	"database/sql"
)

// PermissionRow is one stored permission request.
type PermissionRow struct {
	RequestID, SessionID, Action, Resource, Response, AlwaysJSON string
	TimeCreated                                                  int64
}

// SavePermission inserts or updates a permission request by request_id.
func (d *DB) SavePermission(ctx context.Context, r PermissionRow) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO permission (request_id, session_id, action, resource, `+
			`response, always_json, time_created) VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(request_id) DO UPDATE SET response=excluded.response, always_json=excluded.always_json`,
		r.RequestID, r.SessionID, r.Action, r.Resource, nullStr(r.Response), nullStr(r.AlwaysJSON), r.TimeCreated)
	return err
}

// ListPermissions lists a session's permission requests; pendingOnly filters
// to rows with no response yet.
func (d *DB) ListPermissions(ctx context.Context, sessionID string, pendingOnly bool) ([]PermissionRow, error) {
	q := `SELECT request_id, session_id, action, resource, response, ` +
		`always_json, time_created FROM permission WHERE session_id=?`
	if pendingOnly {
		q += ` AND response IS NULL`
	}
	q += ` ORDER BY time_created ASC, rowid ASC`
	rows, err := d.QueryContext(ctx, q, sessionID)
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
func (d *DB) ReplyPermission(ctx context.Context, requestID, response string) error {
	res, err := d.ExecContext(ctx, `UPDATE permission SET response=? WHERE request_id=?`, response, requestID)
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
