package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
)

func TestRenderOnce_ShowsTasks(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	// Seed two tasks.
	now := time.Now()
	mustCreate(t, st.CreateTask(ctx, &models.Task{
		ID: "T1", Title: "fix login bug", Status: models.StatusTodo,
		Priority: models.PriorityHigh, Tags: []string{"bug"}, CreatedAt: now, UpdatedAt: now,
	}))
	mustCreate(t, st.CreateTask(ctx, &models.Task{
		ID: "T2", Title: "write docs", Status: models.StatusInbox,
		Priority: models.PriorityLow, CreatedAt: now, UpdatedAt: now,
	}))

	var buf bytes.Buffer
	if err := RenderOnce(ctx, st, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Should contain the tab labels and both task titles.
	for _, want := range []string{"Tasks", "Projects", "Initiatives", "Schedule", "fix login bug", "write docs"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func mustCreate(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderOnce_PrintOutput(t *testing.T) {
	// This test exists to print a sample frame during development; it asserts nothing.
	if testing.Short() {
		t.Skip("print test")
	}
	dir := t.TempDir()
	st, _ := store.Open(filepath.Join(dir, "t.db"))
	defer st.Close()
	ctx := context.Background()
	now := time.Now()
	_ = st.CreateTask(ctx, &models.Task{ID: "A", Title: "fix login bug", Status: models.StatusTodo,
		Priority: models.PriorityHigh, Tags: []string{"bug", "auth"}, CreatedAt: now, UpdatedAt: now})
	_ = st.CreateTask(ctx, &models.Task{ID: "B", Title: "write docs", Status: models.StatusInbox,
		Priority: models.PriorityLow, CreatedAt: now, UpdatedAt: now})
	var buf bytes.Buffer
	_ = RenderOnce(ctx, st, &buf)
	t.Log("\n" + buf.String())
}
