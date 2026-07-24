package cli

import (
	"testing"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

func task(id string, status models.TaskStatus, prio models.Priority, due *time.Time) *models.Task {
	return &models.Task{ID: id, Status: status, Priority: prio, DueAt: due}
}

func TestPickNext_PrefersDoing(t *testing.T) {
	due := time.Now().Add(time.Hour)
	tasks := []*models.Task{
		task("a", models.StatusTodo, models.PriorityUrgent, &due),
		task("b", models.StatusDoing, models.PriorityLow, nil),
	}
	if got := pickNext(tasks); got.ID != "b" {
		t.Fatalf("want b (doing), got %s", got.ID)
	}
}

func TestPickNext_HighestPriorityTodo(t *testing.T) {
	tasks := []*models.Task{
		task("low", models.StatusTodo, models.PriorityLow, nil),
		task("urgent", models.StatusTodo, models.PriorityUrgent, nil),
		task("high", models.StatusTodo, models.PriorityHigh, nil),
	}
	if got := pickNext(tasks); got.ID != "urgent" {
		t.Fatalf("want urgent, got %s", got.ID)
	}
}

func TestPickNext_TieBreakByDueDate(t *testing.T) {
	earlier := time.Now().Add(2 * time.Hour)
	later := time.Now().Add(48 * time.Hour)
	tasks := []*models.Task{
		task("later", models.StatusTodo, models.PriorityHigh, &later),
		task("earlier", models.StatusTodo, models.PriorityHigh, &earlier),
	}
	if got := pickNext(tasks); got.ID != "earlier" {
		t.Fatalf("want earlier due, got %s", got.ID)
	}
}

func TestPickNext_FallsBackToInbox(t *testing.T) {
	tasks := []*models.Task{
		task("inbox-low", models.StatusInbox, models.PriorityLow, nil),
		task("inbox-urgent", models.StatusInbox, models.PriorityUrgent, nil),
	}
	if got := pickNext(tasks); got.ID != "inbox-urgent" {
		t.Fatalf("want inbox-urgent, got %s", got.ID)
	}
}

func TestPickNext_NothingActionable(t *testing.T) {
	tasks := []*models.Task{
		task("done", models.StatusDone, models.PriorityUrgent, nil),
		task("blocked", models.StatusBlocked, models.PriorityHigh, nil),
	}
	if got := pickNext(tasks); got != nil {
		t.Fatalf("want nil, got %s", got.ID)
	}
}
