package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

func TestCreateOrUpdateTask_InsertThenUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	tk := &models.Task{ID: "T1", Title: "orig", Status: models.StatusTodo, Priority: models.PriorityMedium, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateOrUpdateTask(ctx, tk); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, _ := s.GetTask(ctx, "T1")
	if got.Title != "orig" {
		t.Fatal("insert failed")
	}
	// Update: change title, keep ID.
	got.Title = "changed"
	got.UpdatedAt = now
	if err := s.CreateOrUpdateTask(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := s.GetTask(ctx, "T1")
	if got2.Title != "changed" {
		t.Fatalf("update not persisted: %q", got2.Title)
	}
}

func TestCreateOrUpdateProject(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now()
	p := &models.Project{ID: "P1", Name: "api", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateOrUpdateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.Description = "updated"
	if err := s.CreateOrUpdateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetProject(ctx, "P1")
	if got.Description != "updated" {
		t.Fatalf("desc = %q", got.Description)
	}
}
