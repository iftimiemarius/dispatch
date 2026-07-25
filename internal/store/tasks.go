package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

// ErrNotFound is returned when a single-entity lookup has no row.
var ErrNotFound = errors.New("not found")

// TaskFilter narrows a task query. Nil/zero fields are ignored.
type TaskFilter struct {
	Status      *models.TaskStatus
	Priority    *models.Priority
	ProjectID   *string
	InitiativeID *string
	Tag         string
	InboxOnly   bool // tasks with status == inbox
}

// TaskQuery is the full options for listing tasks.
type TaskQuery struct {
	Filter TaskFilter
	Limit  int
}

// CreateTask inserts a new task. CreatedAt/UpdatedAt are set by the caller.
func (s *Store) CreateTask(ctx context.Context, t *models.Task) error {
	tags := strings.Join(t.Tags, ",")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tasks (id, title, notes, status, priority, project_id, initiative_id, tags, due_at, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Notes, string(t.Status), string(t.Priority),
		nullIfEmpty(ptrString(t.ProjectID)), nullIfEmpty(ptrString(t.InitiativeID)),
		tags, timeStr(t.DueAt), timeStrNonZero(t.CreatedAt), timeStrNonZero(t.UpdatedAt), timeStr(t.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

// GetTask returns a single task by ID.
func (s *Store) GetTask(ctx context.Context, id string) (*models.Task, error) {
	row := s.db.QueryRowContext(ctx, taskSelect+" WHERE t.id = ?", id)
	t, err := scanTask(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

// ResolveTask accepts either a full task ID or a short suffix (the last N
// characters, as shown in listings) and returns the matching task. If the
// short suffix matches more than one task, ErrAmbiguous is returned so the
// caller can ask for a longer prefix.
func (s *Store) ResolveTask(ctx context.Context, ref string) (*models.Task, error) {
	// Fast path: exact full-ID match.
	if t, err := s.GetTask(ctx, ref); err == nil {
		return t, nil
	}
	// Slow path: match by suffix. ULIDs are 26 chars; a short ref is a suffix.
	list, err := s.ListTasks(ctx, TaskQuery{})
	if err != nil {
		return nil, err
	}
	var matches []*models.Task
	for _, t := range list {
		if strings.HasSuffix(t.ID, ref) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("%w: %q matches %d tasks", ErrAmbiguous, ref, len(matches))
	}
}

// ErrAmbiguous is returned when a short reference matches multiple entities.
var ErrAmbiguous = errors.New("ambiguous reference")
// then due date then creation.
func (s *Store) ListTasks(ctx context.Context, q TaskQuery) ([]*models.Task, error) {
	var (
		clauses []string
		args    []any
	)
	if q.Filter.Status != nil {
		clauses = append(clauses, "t.status = ?")
		args = append(args, string(*q.Filter.Status))
	}
	if q.Filter.Priority != nil {
		clauses = append(clauses, "t.priority = ?")
		args = append(args, string(*q.Filter.Priority))
	}
	if q.Filter.ProjectID != nil {
		clauses = append(clauses, "t.project_id = ?")
		args = append(args, *q.Filter.ProjectID)
	}
	if q.Filter.InitiativeID != nil {
		clauses = append(clauses, "t.initiative_id = ?")
		args = append(args, *q.Filter.InitiativeID)
	}
	if q.Filter.Tag != "" {
		// comma-separated tags: match if tag is surrounded by commas/edges.
		clauses = append(clauses, "((',' || t.tags || ',') LIKE ?)")
		args = append(args, "%,"+q.Filter.Tag+",%")
	}
	if q.Filter.InboxOnly {
		clauses = append(clauses, "t.status = 'inbox'")
	}

	query := taskSelect
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` ORDER BY
		CASE t.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END,
		t.due_at IS NULL, t.due_at, t.created_at`
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t, err := scanTask(rows.Scan)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// UpdateTask overwrites all mutable fields of a task by ID.
func (s *Store) UpdateTask(ctx context.Context, t *models.Task) error {
	tags := strings.Join(t.Tags, ",")
	res, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET
			title = ?, notes = ?, status = ?, priority = ?,
			project_id = ?, initiative_id = ?, tags = ?,
			due_at = ?, updated_at = ?, completed_at = ?
		WHERE id = ?`,
		t.Title, t.Notes, string(t.Status), string(t.Priority),
		nullIfEmpty(ptrString(t.ProjectID)), nullIfEmpty(ptrString(t.InitiativeID)),
		tags, timeStr(t.DueAt), t.UpdatedAt.Format(time.RFC3339), timeStr(t.CompletedAt), t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return requireAffected(res, "task", t.ID)
}

// DeleteTask removes a task by ID.
func (s *Store) DeleteTask(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return requireAffected(res, "task", id)
}

// CountTasks returns the number of tasks matching the filter.
func (s *Store) CountTasks(ctx context.Context, f TaskFilter) (int, error) {
	var (
		clauses []string
		args    []any
	)
	if f.Status != nil {
		clauses = append(clauses, "status = ?")
		args = append(args, string(*f.Status))
	}
	if f.InboxOnly {
		clauses = append(clauses, "status = 'inbox'")
	}
	query := "SELECT COUNT(*) FROM tasks"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	var n int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

const taskSelect = `SELECT t.id, t.title, t.notes, t.status, t.priority,
		t.project_id, t.initiative_id, t.tags, t.due_at,
		t.created_at, t.updated_at, t.completed_at
	FROM tasks t`

type scanFn func(dest ...any) error

func scanTask(scan scanFn) (*models.Task, error) {
	var (
		t           models.Task
		projectID   sql.NullString
		initiativeID sql.NullString
		tags        sql.NullString
		dueAt       sql.NullString
		completedAt sql.NullString
		createdAt   string
		updatedAt   string
		status      string
		priority    string
	)
	err := scan(&t.ID, &t.Title, &t.Notes, &status, &priority,
		&projectID, &initiativeID, &tags, &dueAt,
		&createdAt, &updatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	t.Status = models.TaskStatus(status)
	t.Priority = models.Priority(priority)
	if projectID.Valid {
		v := projectID.String
		t.ProjectID = &v
	}
	if initiativeID.Valid {
		v := initiativeID.String
		t.InitiativeID = &v
	}
	if tags.Valid && tags.String != "" {
		for _, tag := range strings.Split(tags.String, ",") {
			if tag = strings.TrimSpace(tag); tag != "" {
				t.Tags = append(t.Tags, tag)
			}
		}
	}
	if dueAt.Valid {
		t.DueAt = parseTime(dueAt.String)
	}
	t.CreatedAt = parseTimeOrZero(createdAt)
	t.UpdatedAt = parseTimeOrZero(updatedAt)
	if completedAt.Valid {
		t.CompletedAt = parseTime(completedAt.String)
	}
	return &t, nil
}

// requireAffected turns a no-op UPDATE/DELETE into an ErrNotFound.
func requireAffected(res sql.Result, kind, id string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s %s: %w", kind, id, ErrNotFound)
	}
	return nil
}

// timeStr formats a *time.Time as RFC3339, or NULL when nil.
func timeStr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

// timeStrNonZero formats a time.Time as RFC3339, or NULL when the zero value.
func timeStrNonZero(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

// parseTime parses an RFC3339 string into a *time.Time (nil on failure).
func parseTime(s string) *time.Time {
	t, ok := parseTimeOk(s)
	if !ok {
		return nil
	}
	return &t
}

func parseTimeOrZero(s string) time.Time {
	t, _ := parseTimeOk(s)
	return t
}

func parseTimeOk(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}
