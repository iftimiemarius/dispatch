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

// CreateBlock inserts a new calendar block.
func (s *Store) CreateBlock(ctx context.Context, b *models.Block) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO blocks (id, task_id, title, notes, starts_at, ends_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, nullIfEmpty(ptrString(b.TaskID)), b.Title, b.Notes,
		b.StartsAt.Format("2006-01-02T15:04:05Z07:00"), b.EndsAt.Format("2006-01-02T15:04:05Z07:00"),
		b.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), b.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	if err != nil {
		return fmt.Errorf("create block: %w", err)
	}
	return nil
}

// GetBlock returns a single block by ID.
func (s *Store) GetBlock(ctx context.Context, id string) (*models.Block, error) {
	row := s.db.QueryRowContext(ctx, blockSelect+" WHERE b.id = ?", id)
	b, err := scanBlock(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return b, nil
}

// ResolveBlock accepts a full block ID or a short suffix and returns the
// matching block, returning ErrAmbiguous when a suffix matches several.
func (s *Store) ResolveBlock(ctx context.Context, ref string) (*models.Block, error) {
	if b, err := s.GetBlock(ctx, ref); err == nil {
		return b, nil
	}
	list, err := s.ListBlocks(ctx, nil)
	if err != nil {
		return nil, err
	}
	var matches []*models.Block
	for _, b := range list {
		if strings.HasSuffix(b.ID, ref) {
			matches = append(matches, b)
		}
	}
	switch len(matches) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("%w: %q matches %d blocks", ErrAmbiguous, ref, len(matches))
	}
}

// BlockRange is an optional half-open window for listing blocks.
type BlockRange struct {
	From time.Time // inclusive
	To   time.Time // inclusive
}

// ListBlocks returns blocks ordered by start time, optionally within a window.
func (s *Store) ListBlocks(ctx context.Context, r *BlockRange) ([]*models.Block, error) {
	query := blockSelect
	var (
		clauses []string
		args    []any
	)
	if r != nil {
		if !r.From.IsZero() {
			clauses = append(clauses, "b.starts_at >= ?")
			args = append(args, r.From.Format("2006-01-02T15:04:05Z07:00"))
		}
		if !r.To.IsZero() {
			clauses = append(clauses, "b.starts_at <= ?")
			args = append(args, r.To.Format("2006-01-02T15:04:05Z07:00"))
		}
	}
	if len(clauses) > 0 {
		query += " WHERE " + joinClauses(clauses)
	}
	query += " ORDER BY b.starts_at"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	defer rows.Close()

	var blocks []*models.Block
	for rows.Next() {
		b, err := scanBlock(rows.Scan)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

// joinClauses is a local joiner to avoid pulling strings into this file.
func joinClauses(c []string) string {
	out := ""
	for i, s := range c {
		if i > 0 {
			out += " AND "
		}
		out += s
	}
	return out
}

// UpdateBlock overwrites mutable block fields by ID.
func (s *Store) UpdateBlock(ctx context.Context, b *models.Block) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE blocks SET
			task_id = ?, title = ?, notes = ?, starts_at = ?, ends_at = ?, updated_at = ?
		WHERE id = ?`,
		nullIfEmpty(ptrString(b.TaskID)), b.Title, b.Notes,
		b.StartsAt.Format("2006-01-02T15:04:05Z07:00"), b.EndsAt.Format("2006-01-02T15:04:05Z07:00"),
		b.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"), b.ID,
	)
	if err != nil {
		return fmt.Errorf("update block: %w", err)
	}
	return requireAffected(res, "block", b.ID)
}

// DeleteBlock removes a block by ID.
func (s *Store) DeleteBlock(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM blocks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	return requireAffected(res, "block", id)
}

const blockSelect = `SELECT b.id, b.task_id, b.title, b.notes, b.starts_at, b.ends_at,
		b.created_at, b.updated_at
	FROM blocks b`

func scanBlock(scan scanFn) (*models.Block, error) {
	var (
		b         models.Block
		taskID    sql.NullString
		startsAt  string
		endsAt    string
		createdAt string
		updatedAt string
	)
	err := scan(&b.ID, &taskID, &b.Title, &b.Notes, &startsAt, &endsAt, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if taskID.Valid {
		v := taskID.String
		b.TaskID = &v
	}
	b.StartsAt = parseTimeOrZero(startsAt)
	b.EndsAt = parseTimeOrZero(endsAt)
	b.CreatedAt = parseTimeOrZero(createdAt)
	b.UpdatedAt = parseTimeOrZero(updatedAt)
	return &b, nil
}
