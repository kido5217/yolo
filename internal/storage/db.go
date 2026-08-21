// Package storage is the SQLite persistence layer (modernc.org/sqlite, pure Go).
package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/kido5217/yolo/internal/protocol"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("storage: not found")

// DB wraps a SQL database. Exec/Query/QueryRow route through a cache of
// prepared statements, so repeated calls reuse the driver's prepared
// statement instead of re-parsing the SQL on every call.
type DB struct {
	*sql.DB
	mu    sync.Mutex
	stmts map[string]*sql.Stmt
}

// Open opens (creates if missing) the database at path and runs pending
// migrations. The PRAGMAs are set per connection via DSN keys, since
// busy_timeout and foreign_keys are not persisted: every connection the
// pool opens carries them, including replacements after idle reaping.
func Open(path string) (*DB, error) {
	dsn := "file:" + path + "?_foreign_keys=1&_busy_timeout=5000&_journal_mode=WAL"
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One shared connection: a single-writer SQLite store needs at most
	// one writer, and it makes the per-connection PRAGMAs total by
	// construction (no pooled connections can bypass them).
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1)
	d := &DB{DB: raw, stmts: make(map[string]*sql.Stmt)}
	if err := d.migrate(); err != nil {
		raw.Close()
		return nil, err
	}
	return d, nil
}

// Close closes the cached prepared statements and the underlying database.
func (d *DB) Close() error {
	d.mu.Lock()
	var errs []error
	for q, st := range d.stmts {
		if err := st.Close(); err != nil {
			errs = append(errs, err)
		}
		delete(d.stmts, q)
	}
	d.mu.Unlock()
	if err := d.DB.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Exec executes query through the cached prepared statement.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	st, err := d.prepare(query)
	if err != nil {
		return nil, err
	}
	return st.Exec(args...)
}

// Query executes query through the cached prepared statement.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	st, err := d.prepare(query)
	if err != nil {
		return nil, err
	}
	return st.Query(args...)
}

// QueryRow executes query through the cached prepared statement. A failed
// prepare (e.g. after Close) falls through to the underlying *sql.DB so
// callers see the same error from Scan as before caching was introduced.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	st, err := d.prepare(query)
	if err != nil {
		return d.DB.QueryRow(query, args...)
	}
	return st.QueryRow(args...)
}

// prepare returns the cached prepared statement for query, preparing and
// caching it on first use. Mutex held only across the map update;
// Exec/Query never run under it (a blocking Exec must not serialize the
// statement lookup).
func (d *DB) prepare(query string) (*sql.Stmt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if st, ok := d.stmts[query]; ok {
		return st, nil
	}
	st, err := d.Prepare(query)
	if err != nil {
		return nil, err
	}
	d.stmts[query] = st
	return st, nil
}

// SchemaVersion returns the applied schema version (0 if none).
func (d *DB) SchemaVersion() (int, error) { return d.currentSchemaVersion() }

// Session returns the wire session with cost/tokens aggregated over its
// assistant messages.
func (d *DB) Session(id string) (protocol.Session, error) {
	row, err := d.GetSession(id)
	if err != nil {
		return protocol.Session{}, err
	}
	msgs, err := d.ListMessages(id)
	if err != nil {
		return protocol.Session{}, err
	}
	return SessionFromRow(row, msgs), nil
}

// projectIDFromDir derives a deterministic project ID from the directory.
func projectIDFromDir(dir string) string {
	sum := sha256.Sum256([]byte(dir))
	return "prj_" + hex.EncodeToString(sum[:])[:24]
}

// ProjectID derives a deterministic project ID from a directory path.
func ProjectID(dir string) string { return projectIDFromDir(dir) }

// modelRefFromString parses "provider/model" into a ModelRef.
func modelRefFromString(s string) *protocol.ModelRef {
	if s == "" {
		return nil
	}
	if i := strings.IndexByte(s, '/'); i > 0 {
		return &protocol.ModelRef{ProviderID: s[:i], ID: s[i+1:]}
	}
	return &protocol.ModelRef{ID: s}
}
