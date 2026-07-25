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
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

//go:embed migrations/002_github_outlook.sql
var migration002 string

// currentSchemaVersion is bumped whenever a new migration is added; the runner
// applies every migration up to this number. Migration 1 is the baseline
// schema.sql (applied via CREATE TABLE IF NOT EXISTS for fresh installs).
const currentSchemaVersion = 2

// migrations maps a version number to its SQL, applied in ascending order.
// Migration 1 is intentionally absent here — it's the baseline schema.sql.
var migrations = map[int]string{
	2: migration002,
}

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

// migrate brings the database up to currentSchemaVersion.
//
// Step 1 applies the baseline schema.sql (CREATE TABLE IF NOT EXISTS) — for
// fresh installs this creates everything; for existing DBs it's a no-op.
//
// Steps 2..N run each numbered migration's SQL that hasn't been recorded yet.
// Each statement is split and applied individually so that idempotent
// "ALTER TABLE ADD COLUMN" statements whose column already exists (e.g. a
// partial earlier run) are tolerated rather than aborting the migration.
func (s *Store) migrate(ctx context.Context) error {
	// Baseline: create tables if missing (fresh install or pre-migration DB).
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	// Record the baseline version so a fresh install doesn't re-run migration 1.
	if err := s.recordVersion(ctx, 1); err != nil {
		return err
	}

	// Apply each numbered migration in ascending order.
	for v := 2; v <= currentSchemaVersion; v++ {
		applied, err := s.isApplied(ctx, v)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		sqlText, ok := migrations[v]
		if !ok {
			// No file for this version; record it as applied to skip future checks.
			if err := s.recordVersion(ctx, v); err != nil {
				return err
			}
			continue
		}
		if err := s.applyMigration(ctx, v, sqlText); err != nil {
			return fmt.Errorf("migration %d: %w", v, err)
		}
		if err := s.recordVersion(ctx, v); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration runs each statement in sqlText individually, tolerating
// "duplicate column" errors so ADD COLUMN is idempotent. Comments (lines
// starting with --) are stripped before splitting so they don't lump together
// with the statement that follows them.
func (s *Store) applyMigration(ctx context.Context, version int, sqlText string) error {
	for _, stmt := range splitStatements(stripComments(sqlText)) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			if isDuplicateColumnErr(err) {
				continue
			}
			return fmt.Errorf("statement %q: %w", firstLine(stmt), err)
		}
	}
	return nil
}

// stripComments removes full-line and trailing SQL comments (lines/segments
// starting with --). Good enough for our migration files.
func stripComments(sqlText string) string {
	var b strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (s *Store) isApplied(ctx context.Context, version int) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_version WHERE version = ?", version).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) recordVersion(ctx context.Context, version int) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO schema_version(version) VALUES (?)", version)
	return err
}

// isDuplicateColumnErr reports whether err is SQLite's "duplicate column name".
func isDuplicateColumnErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column")
}

// splitStatements splits SQL on semicolons at the end of lines, ignoring those
// inside comments. Good enough for our migration files.
func splitStatements(sqlText string) []string {
	var stmts []string
	for _, line := range strings.Split(sqlText, ";\n") {
		stmts = append(stmts, line)
	}
	return stmts
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
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

// nullInt returns a NULL-bound driver value for a nil *int.
func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
