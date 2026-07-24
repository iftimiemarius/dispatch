package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/iftimiemarius/dispatch/internal/models"
)

// CreateProject inserts a new project.
func (s *Store) CreateProject(ctx context.Context, p *models.Project) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, status, color, initiative_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.Status, p.Color,
		nullIfEmpty(ptrString(p.InitiativeID)), p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// GetProject returns a single project by ID.
func (s *Store) GetProject(ctx context.Context, id string) (*models.Project, error) {
	row := s.db.QueryRowContext(ctx, projectSelect+" WHERE p.id = ?", id)
	p, err := scanProject(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// GetProjectByName looks up a project by its unique name.
func (s *Store) GetProjectByName(ctx context.Context, name string) (*models.Project, error) {
	row := s.db.QueryRowContext(ctx, projectSelect+" WHERE p.name = ?", name)
	p, err := scanProject(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// ListProjects returns all projects, optionally filtered by initiative.
func (s *Store) ListProjects(ctx context.Context, initiativeID *string) ([]*models.Project, error) {
	query := projectSelect
	var args []any
	if initiativeID != nil {
		query += " WHERE p.initiative_id = ?"
		args = append(args, *initiativeID)
	}
	query += " ORDER BY p.name COLLATE NOCASE"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		p, err := scanProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateProject overwrites mutable project fields by ID.
func (s *Store) UpdateProject(ctx context.Context, p *models.Project) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE projects SET
			name = ?, description = ?, status = ?, color = ?,
			initiative_id = ?, updated_at = ?
		WHERE id = ?`,
		p.Name, p.Description, p.Status, p.Color,
		nullIfEmpty(ptrString(p.InitiativeID)), p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"), p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return requireAffected(res, "project", p.ID)
}

// DeleteProject removes a project by ID. Linked tasks have project_id set NULL.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return requireAffected(res, "project", id)
}

const projectSelect = `SELECT p.id, p.name, p.description, p.status, p.color,
		p.initiative_id, p.created_at, p.updated_at
	FROM projects p`

func scanProject(scan scanFn) (*models.Project, error) {
	var (
		p            models.Project
		initiativeID sql.NullString
		createdAt    string
		updatedAt    string
	)
	err := scan(&p.ID, &p.Name, &p.Description, &p.Status, &p.Color,
		&initiativeID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if initiativeID.Valid {
		v := initiativeID.String
		p.InitiativeID = &v
	}
	p.CreatedAt = parseTimeOrZero(createdAt)
	p.UpdatedAt = parseTimeOrZero(updatedAt)
	return &p, nil
}
