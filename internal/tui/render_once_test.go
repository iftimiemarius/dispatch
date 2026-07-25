package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
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

// renderForView builds the app, switches to the given view, renders, and
// returns the frame. Used to assert each tab populates.
func renderForView(t *testing.T, st *store.Store, v view) string {
	t.Helper()
	a := newApp(context.Background(), st)
	a.active = v
	ma, _ := a.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	a = ma.(*app)
	var buf bytes.Buffer
	_, _ = buf.WriteString(a.View())
	return buf.String()
}

func TestRenderEachView(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	now := time.Now()

	// Seed all entity types.
	mustCreate(t, st.CreateProject(ctx, &models.Project{ID: "P1", Name: "api", Status: "active", CreatedAt: now, UpdatedAt: now}))
	mustCreate(t, st.CreateInitiative(ctx, &models.Initiative{ID: "I1", Name: "Q3 Launch", Outcome: "ship v2", Status: "active", CreatedAt: now, UpdatedAt: now}))
	mustCreate(t, st.CreateTask(ctx, &models.Task{ID: "T1", Title: "fix login bug", Status: models.StatusTodo, Priority: models.PriorityHigh, CreatedAt: now, UpdatedAt: now}))
	pid := "P1"
	mustCreate(t, st.CreateBlock(ctx, &models.Block{ID: "B1", Title: "deep work", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), CreatedAt: now, UpdatedAt: now}))
	_ = pid

	// Tasks view.
	if out := renderForView(t, st, viewTasks); !strings.Contains(out, "fix login bug") {
		t.Errorf("tasks view missing task title:\n%s", out)
	}
	// Projects view.
	if out := renderForView(t, st, viewProjects); !strings.Contains(out, "api") {
		t.Errorf("projects view missing project name:\n%s", out)
	}
	// Initiatives view.
	if out := renderForView(t, st, viewInitiatives); !strings.Contains(out, "Q3 Launch") {
		t.Errorf("initiatives view missing initiative name:\n%s", out)
	}
	// Schedule view.
	if out := renderForView(t, st, viewSchedule); !strings.Contains(out, "deep work") {
		t.Errorf("schedule view missing block title:\n%s", out)
	}
}

func mustCreate(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderOnce_PrintOutput(t *testing.T) {
	// Render one frame and log it, so \`go test -v\` shows the layout. Skipped
	// in -short mode.
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
