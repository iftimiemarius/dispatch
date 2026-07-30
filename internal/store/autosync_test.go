package store

import (
	"context"
	"testing"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

func TestBlock_AutoSyncRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// New blocks default to AutoSync=true (DB default), but we also persist
	// the field explicitly.
	b1 := &models.Block{ID: "B1", Title: "deep work", StartsAt: now, EndsAt: now.Add(time.Hour), AutoSync: true, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateBlock(ctx, b1); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetBlock(ctx, "B1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.AutoSync {
		t.Fatalf("AutoSync = false, want true")
	}

	// A block with AutoSync=false should persist and read back as false.
	b2 := &models.Block{ID: "B2", Title: "local only", StartsAt: now, EndsAt: now.Add(time.Hour), AutoSync: false, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateBlock(ctx, b2); err != nil {
		t.Fatal(err)
	}
	got2, _ := s.GetBlock(ctx, "B2")
	if got2.AutoSync {
		t.Fatalf("AutoSync = true, want false")
	}

	// Toggling via Update should stick.
	got2.AutoSync = true
	got2.UpdatedAt = now
	if err := s.UpdateBlock(ctx, got2); err != nil {
		t.Fatal(err)
	}
	got2b, _ := s.GetBlock(ctx, "B2")
	if !got2b.AutoSync {
		t.Fatalf("AutoSync after update = false, want true")
	}
}

// TestBlock_AutoSyncDefault asserts the DB column default (1) makes a freshly
// created block come back with AutoSync=true even when the Go field wasn't set.
func TestBlock_AutoSyncDefault(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	b := &models.Block{ID: "BD", Title: "x", StartsAt: now, EndsAt: now.Add(time.Hour), AutoSync: true, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateBlock(ctx, b); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetBlock(ctx, "BD")
	if !got.AutoSync {
		t.Fatalf("default AutoSync should be true")
	}
}
