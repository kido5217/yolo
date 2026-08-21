// Package storage is the SQLite persistence layer (modernc.org/sqlite, pure Go).
package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/kido5217/yolo/internal/protocol"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("storage: not found")

// DB wraps a SQL database.
type DB struct {
	*sql.DB
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
	d := &DB{raw}
	if err := d.migrate(); err != nil {
		raw.Close()
		return nil, err
	}
	return d, nil
}

// Close closes the underlying database.
func (d *DB) Close() error { return d.DB.Close() }

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
