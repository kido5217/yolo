package storage

import (
	"database/sql"
	"errors"
	"sort"
	"strconv"
)

// migrations maps schema version -> DDL, applied in ascending order.
//
// The spec's tables are verbatim, plus the plan-noted additions: message.cost
// REAL and message.tokens TEXT. The session.cost column is kept for parity and
// ignored by SessionFromRow (which recomputes the aggregate from messages).
// The meta table is created in Open before migrations run, so it is not here.
var migrations = map[int]string{
	1: `
CREATE TABLE session (
  id TEXT PRIMARY KEY,
  project_dir TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,
  agent TEXT NOT NULL DEFAULT 'build',
  cost REAL NOT NULL DEFAULT 0,
  time_created INTEGER NOT NULL,
  time_updated INTEGER NOT NULL
);
CREATE TABLE message (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  agent TEXT NOT NULL DEFAULT 'build',
  cost REAL NOT NULL DEFAULT 0,
  tokens TEXT NOT NULL DEFAULT '{}',
  time_created INTEGER NOT NULL,
  time_completed INTEGER
);
CREATE TABLE part (
  id TEXT PRIMARY KEY,
  message_id TEXT NOT NULL REFERENCES message(id) ON DELETE CASCADE,
  session_id TEXT NOT NULL,
  type TEXT NOT NULL,
  tool TEXT,
  state_json TEXT NOT NULL,
  time_created INTEGER NOT NULL
);
CREATE TABLE permission (
  request_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  action TEXT NOT NULL,
  resource TEXT NOT NULL,
  response TEXT,
  always_json TEXT,
  time_created INTEGER NOT NULL
);
`,
	2: `
CREATE TABLE IF NOT EXISTS todo (
    id INTEGER PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    status TEXT NOT NULL,
    priority TEXT NOT NULL DEFAULT 'medium',
    position INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_todo_session ON todo(session_id);
`,
	3: `
ALTER TABLE message ADD COLUMN error_json TEXT;
`,
}

// migrate applies any unapplied migrations in ascending version order.
func (d *DB) migrate() error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	cur, err := d.currentSchemaVersion()
	if err != nil {
		return err
	}
	versions := make([]int, 0, len(migrations))
	for v := range migrations {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for _, v := range versions {
		if v <= cur {
			continue
		}
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(migrations[v]); err != nil {
			tx.Rollback()
			return err
		}
		_, err = tx.Exec(
			`INSERT OR REPLACE INTO meta (key, value) VALUES ('schema_version', ?)`, strconv.Itoa(v))
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) currentSchemaVersion() (int, error) {
	var s string
	err := d.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return strconv.Atoi(s)
}
