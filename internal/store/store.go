// Package store is the repository layer over an embedded SQLite database.
//
// It owns connection management, schema migrations, and CRUD operations for
// tasks, projects, initiatives, and blocks. All time fields are stored as
// RFC3339 strings and converted to/from time.Time at the repository boundary.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// currentSchemaVersion is bumped whenever schema.sql changes; migrations run
// from the stored version up to this number.
const currentSchemaVersion = 1

// Store wraps a SQLite database connection.
type Store struct {
	db *sql.DB
}

// Open connects to (creating if absent) the database at path and brings its
// schema up to the current version. The database is configured for
// single-writer use with WAL for fast reads.
func Open(path string) (*Store, error) {
	// SQLite URI options: enable WAL, a busy timeout so concurrent CLI
	// invocations don't immediately fail, and shared cache off.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite is happiest with a single open connection for writes.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB exposes the underlying connection for tests that need raw access.
func (s *Store) DB() *sql.DB { return s.db }

// migrate applies schema.sql on first run and records the version. v1 only
// has a baseline migration; future schema changes will add numbered steps.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	// Record current version if not already present (idempotent).
	var existing int
	row := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_version WHERE version = ?", currentSchemaVersion)
	if err := row.Scan(&existing); err != nil {
		return fmt.Errorf("check version: %w", err)
	}
	if existing == 0 {
		if _, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO schema_version(version) VALUES (?)", currentSchemaVersion); err != nil {
			return fmt.Errorf("record version: %w", err)
		}
	}
	return nil
}

// tx runs fn inside a transaction, committing on nil error and rolling back
// otherwise. Used by write paths that need atomicity.
func (s *Store) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	t, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = t.Rollback()
		}
	}()
	if err := fn(t); err != nil {
		return err
	}
	return t.Commit()
}

// --- helpers shared across repos ---

// ptrString returns the dereferenced value or "" when nil.
func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// nullIfEmpty returns a NULL-bound driver value for an empty string.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
