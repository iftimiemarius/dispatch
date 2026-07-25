package store

import (
	"context"

	"github.com/iftimiemarius/dispatch/internal/models"
)

// CreateOrUpdateTask inserts a new task or updates it if it already exists.
// It decides based on whether a task with the same ID is present.
func (s *Store) CreateOrUpdateTask(ctx context.Context, t *models.Task) error {
	if existing, err := s.GetTask(ctx, t.ID); err == nil && existing != nil {
		return s.UpdateTask(ctx, t)
	}
	return s.CreateTask(ctx, t)
}

// CreateOrUpdateProject inserts or updates a project by ID.
func (s *Store) CreateOrUpdateProject(ctx context.Context, p *models.Project) error {
	if existing, err := s.GetProject(ctx, p.ID); err == nil && existing != nil {
		return s.UpdateProject(ctx, p)
	}
	return s.CreateProject(ctx, p)
}

// CreateOrUpdateInitiative inserts or updates an initiative by ID.
func (s *Store) CreateOrUpdateInitiative(ctx context.Context, i *models.Initiative) error {
	if existing, err := s.GetInitiative(ctx, i.ID); err == nil && existing != nil {
		return s.UpdateInitiative(ctx, i)
	}
	return s.CreateInitiative(ctx, i)
}

// CreateOrUpdateBlock inserts or updates a block by ID.
func (s *Store) CreateOrUpdateBlock(ctx context.Context, b *models.Block) error {
	if existing, err := s.GetBlock(ctx, b.ID); err == nil && existing != nil {
		return s.UpdateBlock(ctx, b)
	}
	return s.CreateBlock(ctx, b)
}
