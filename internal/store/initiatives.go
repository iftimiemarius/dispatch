package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/iftimiemarius/dispatch/internal/models"
)

// CreateInitiative inserts a new initiative.
func (s *Store) CreateInitiative(ctx context.Context, i *models.Initiative) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO initiatives (id, name, outcome, status, start_at, target_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID, i.Name, i.Outcome, i.Status, timeStr(i.StartAt), timeStr(i.TargetAt),
		i.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), i.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	if err != nil {
		return fmt.Errorf("create initiative: %w", err)
	}
	return nil
}

// GetInitiative returns a single initiative by ID.
func (s *Store) GetInitiative(ctx context.Context, id string) (*models.Initiative, error) {
	row := s.db.QueryRowContext(ctx, initiativeSelect+" WHERE i.id = ?", id)
	i, err := scanInitiative(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return i, nil
}

// ListInitiatives returns all initiatives ordered by creation.
func (s *Store) ListInitiatives(ctx context.Context) ([]*models.Initiative, error) {
	rows, err := s.db.QueryContext(ctx, initiativeSelect+" ORDER BY i.created_at")
	if err != nil {
		return nil, fmt.Errorf("list initiatives: %w", err)
	}
	defer rows.Close()

	var initiatives []*models.Initiative
	for rows.Next() {
		i, err := scanInitiative(rows.Scan)
		if err != nil {
			return nil, err
		}
		initiatives = append(initiatives, i)
	}
	return initiatives, rows.Err()
}

// UpdateInitiative overwrites mutable initiative fields by ID.
func (s *Store) UpdateInitiative(ctx context.Context, i *models.Initiative) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE initiatives SET
			name = ?, outcome = ?, status = ?, start_at = ?, target_at = ?, updated_at = ?
		WHERE id = ?`,
		i.Name, i.Outcome, i.Status, timeStr(i.StartAt), timeStr(i.TargetAt),
		i.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"), i.ID,
	)
	if err != nil {
		return fmt.Errorf("update initiative: %w", err)
	}
	return requireAffected(res, "initiative", i.ID)
}

// DeleteInitiative removes an initiative by ID.
func (s *Store) DeleteInitiative(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM initiatives WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete initiative: %w", err)
	}
	return requireAffected(res, "initiative", id)
}

const initiativeSelect = `SELECT i.id, i.name, i.outcome, i.status, i.start_at, i.target_at,
		i.created_at, i.updated_at
	FROM initiatives i`

func scanInitiative(scan scanFn) (*models.Initiative, error) {
	var (
		i         models.Initiative
		startAt   sql.NullString
		targetAt  sql.NullString
		createdAt string
		updatedAt string
	)
	err := scan(&i.ID, &i.Name, &i.Outcome, &i.Status, &startAt, &targetAt, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if startAt.Valid {
		i.StartAt = parseTime(startAt.String)
	}
	if targetAt.Valid {
		i.TargetAt = parseTime(targetAt.String)
	}
	i.CreatedAt = parseTimeOrZero(createdAt)
	i.UpdatedAt = parseTimeOrZero(updatedAt)
	return &i, nil
}
