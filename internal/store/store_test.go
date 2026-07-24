package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

// newTestStore opens a fresh in-memory-via-tempfile store for each test.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestMigrate_Idempotent(t *testing.T) {
	s := newTestStore(t)
	// Opening again applies the same schema; schema_version stays at 1.
	var version int
	err := s.db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("query version: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, currentSchemaVersion)
	}
}

func TestTask_CreateGetUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	task := &models.Task{
		ID:        "01HVTASK",
		Title:     "fix login bug",
		Notes:     "redirect loop",
		Status:    models.StatusInbox,
		Priority:  models.PriorityHigh,
		Tags:      []string{"bug", "auth"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != task.Title || got.Status != task.Status {
		t.Fatalf("roundtrip mismatch: got %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "bug" || got.Tags[1] != "auth" {
		t.Fatalf("tags mismatch: got %v", got.Tags)
	}

	// Update: complete the task.
	got.Status = models.StatusDone
	completed := now.Add(time.Hour)
	got.CompletedAt = &completed
	got.UpdatedAt = completed
	if err := s.UpdateTask(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if got2.Status != models.StatusDone || got2.CompletedAt == nil {
		t.Fatalf("update not persisted: %+v", got2)
	}
}

func TestTask_List_Filtering(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for i, tc := range []struct {
		title    string
		status   models.TaskStatus
		priority models.Priority
		tags     []string
	}{
		{"a", models.StatusInbox, models.PriorityHigh, []string{"x"}},
		{"b", models.StatusTodo, models.PriorityUrgent, []string{"y"}},
		{"c", models.StatusDone, models.PriorityLow, []string{"x", "y"}},
	} {
		tk := &models.Task{
			ID:        "T" + string(rune('0'+i)),
			Title:     tc.title,
			Status:    tc.status,
			Priority:  tc.priority,
			Tags:      tc.tags,
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
			UpdatedAt: now,
		}
		if err := s.CreateTask(ctx, tk); err != nil {
			t.Fatalf("create %s: %v", tc.title, err)
		}
	}

	// Inbox only.
	inbox, err := s.ListTasks(ctx, TaskQuery{Filter: TaskFilter{InboxOnly: true}})
	if err != nil {
		t.Fatalf("list inbox: %v", err)
	}
	if len(inbox) != 1 || inbox[0].Title != "a" {
		t.Fatalf("inbox = %v", inbox)
	}

	// Tag filter.
	tagged, err := s.ListTasks(ctx, TaskQuery{Filter: TaskFilter{Tag: "x"}})
	if err != nil {
		t.Fatalf("list tag: %v", err)
	}
	if len(tagged) != 2 {
		t.Fatalf("tag x count = %d, want 2", len(tagged))
	}

	// Ordering: urgent before high before low (done excluded only by filter,
	// here we list all).
	all, err := s.ListTasks(ctx, TaskQuery{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all count = %d, want 3", len(all))
	}
	if all[0].Priority != models.PriorityUrgent {
		t.Fatalf("first should be urgent, got %s (%s)", all[0].Priority, all[0].Title)
	}
}

func TestTask_NotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.GetTask(context.Background(), "nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestProject_CreateListByName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	p := &models.Project{
		ID: "P1", Name: "api", Description: "the API", Status: "active", Color: "blue",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetProjectByName(ctx, "api")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.ID != "P1" {
		t.Fatalf("id = %s", got.ID)
	}
	// Case-insensitive listing order is by name.
	list, err := s.ListProjects(ctx, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d", len(list))
	}
}

func TestInitiative_CRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	start := now.AddDate(0, 0, -7)
	target := now.AddDate(0, 0, 14)

	i := &models.Initiative{
		ID: "I1", Name: "Q3 launch", Outcome: "ship v2", Status: "active",
		StartAt: &start, TargetAt: &target, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateInitiative(ctx, i); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetInitiative(ctx, "I1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Outcome != "ship v2" || got.StartAt == nil || got.TargetAt == nil {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if err := s.DeleteInitiative(ctx, "I1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetInitiative(ctx, "I1"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestBlock_CRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	b := &models.Block{
		ID: "B1", Title: "deep work", Notes: "auth refactor",
		StartsAt: now.Add(2 * time.Hour), EndsAt: now.Add(4 * time.Hour),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateBlock(ctx, b); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetBlock(ctx, "B1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "deep work" {
		t.Fatalf("title = %s", got.Title)
	}

	// Range query: block is 2h from now; a window starting now should include it.
	from := now.Add(time.Hour)
	to := now.Add(5 * time.Hour)
	inRange, err := s.ListBlocks(ctx, &BlockRange{From: from, To: to})
	if err != nil {
		t.Fatalf("list range: %v", err)
	}
	if len(inRange) != 1 {
		t.Fatalf("range len = %d, want 1", len(inRange))
	}
}

func TestTask_ProjectLink_SetNullOnDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	p := &models.Project{ID: "P1", Name: "api", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	pid := "P1"
	tk := &models.Task{
		ID: "T1", Title: "task", Status: models.StatusTodo, Priority: models.PriorityMedium, ProjectID: &pid,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateTask(ctx, tk); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject(ctx, "P1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTask(ctx, "T1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != nil {
		t.Fatalf("project_id should be NULL after delete, got %q", *got.ProjectID)
	}
}
