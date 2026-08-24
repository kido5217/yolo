package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// SessionRow is one stored session.
type SessionRow struct {
	ID, ProjectDir, Title, Model, Agent string
	Cost                                float64
	TimeCreated, TimeUpdated            int64
}

// CreateSession inserts a session row; an empty agent takes the column
// default "build" (agentOrDefault), mirroring the message side.
func (d *DB) CreateSession(ctx context.Context, r SessionRow) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO session (id, project_dir, title, model, agent, cost, `+
			`time_created, time_updated) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.ProjectDir, r.Title, r.Model, agentOrDefault(r.Agent), r.Cost, r.TimeCreated, r.TimeUpdated)
	return err
}

// GetSession fetches one session; missing id -> ErrNotFound.
func (d *DB) GetSession(ctx context.Context, id string) (SessionRow, error) {
	var r SessionRow
	err := d.QueryRowContext(ctx,
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
func (d *DB) ListSessions(ctx context.Context, projectDir string, limit int) ([]SessionRow, error) {
	q := `SELECT id, project_dir, title, model, agent, cost, time_created, ` +
		`time_updated FROM session WHERE project_dir=? ORDER BY time_updated DESC`
	args := []any{projectDir}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := d.QueryContext(ctx, q, args...)
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
func (d *DB) UpdateSession(ctx context.Context, id string, patch SessionRow) error {
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
	res, err := d.ExecContext(ctx, query, args...)
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

// DeleteSession removes a session (messages/parts cascade via FK).
func (d *DB) DeleteSession(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM session WHERE id=?`, id)
	return err
}
