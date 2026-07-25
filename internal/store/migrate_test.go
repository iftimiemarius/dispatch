package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

func now() time.Time { return time.Now().UTC() }

// TestMigrate_UpgradesV1DB asserts that a database created with the v1 schema
// (without the GitHub/Outlook columns) is upgraded to the current version with
// all new columns present. This guards the real migration runner.
func TestMigrate_UpgradesV1DB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Build a hand-written v1 database: the original schema, version 1, no
	// new columns. This simulates an existing install from before this feature.
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	v1Schema := `
		CREATE TABLE schema_version (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')));
		INSERT INTO schema_version(version) VALUES (1);
		CREATE TABLE initiatives (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, outcome TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active', start_at TEXT, target_at TEXT,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
		CREATE TABLE projects (
			id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active', color TEXT NOT NULL DEFAULT '',
			initiative_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
		CREATE TABLE tasks (
			id TEXT PRIMARY KEY, title TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'inbox', priority TEXT NOT NULL DEFAULT 'medium',
			project_id TEXT, initiative_id TEXT, tags TEXT NOT NULL DEFAULT '',
			due_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT);
		CREATE TABLE blocks (
			id TEXT PRIMARY KEY, task_id TEXT, title TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '',
			starts_at TEXT NOT NULL, ends_at TEXT NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
	`
	if _, err := db.Exec(v1Schema); err != nil {
		t.Fatalf("seed v1 schema: %v", err)
	}
	// Confirm the old tasks table has NO github columns (sanity: we're really v1).
	var cols []string
	rows, _ := db.Query("PRAGMA table_info(tasks)")
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		_ = rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		cols = append(cols, name)
	}
	rows.Close()
	db.Close()

	if hasColumn(cols, "github_repo") {
		t.Fatalf("test setup wrong: tasks already has github_repo")
	}

	// Now open via the real Store, which runs migrations.
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open after v1: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Version must be current.
	var version int
	if err := s.db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version); err != nil {
		t.Fatalf("version query: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("version = %d, want %d", version, currentSchemaVersion)
	}

	// New columns must exist.
	checkCol := func(table, col string) {
		t.Helper()
		var got []string
		r, _ := s.db.Query("PRAGMA table_info(" + table + ")")
		for r.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			_ = r.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			got = append(got, name)
		}
		r.Close()
		if !hasColumn(got, col) {
			t.Errorf("table %s missing column %s after migration (has %v)", table, col, got)
		}
	}
	checkCol("projects", "github_repo")
	checkCol("tasks", "github_repo")
	checkCol("tasks", "github_issue_number")
	checkCol("blocks", "outlook_event_id")

	// And a fresh write/read of the new fields must work end-to-end.
	ctx := context.Background()
	p := &models.Project{ID: "P1", Name: "api", Status: "active", GitHubRepo: strPtr("octocat/Hello-World"),
		CreatedAt: now(), UpdatedAt: now()}
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("create project with github_repo: %v", err)
	}
	got, err := s.GetProject(ctx, "P1")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got.GitHubRepo == nil || *got.GitHubRepo != "octocat/Hello-World" {
		t.Fatalf("github_repo not round-tripped: %+v", got)
	}
}

func hasColumn(cols []string, name string) bool {
	for _, c := range cols {
		if c == name {
			return true
		}
	}
	return false
}

func strPtr(s string) *string { return &s }
