package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
)

// reloadAll refreshes every view's data from the store. Called on launch and
// after any mutation.
func (a *app) reloadAll() {
	a.projectNames = loadProjectNames(a.ctx, a.store)
	a.reloadTasks()
	a.reloadProjects()
	a.reloadInitiatives()
	a.reloadSchedule()
}

// reloadTasks populates the Tasks list, hiding finished tasks (matching the CLI).
func (a *app) reloadTasks() {
	tasks, err := a.store.ListTasks(a.ctx, store.TaskQuery{})
	if err != nil {
		a.statusMsg = "load tasks: " + err.Error()
		return
	}
	items := make([]item, 0, len(tasks))
	for _, tk := range tasks {
		if tk.Status == models.StatusDone || tk.Status == models.StatusCancelled {
			continue
		}
		items = append(items, a.taskItem(tk))
	}
	a.setItems(&a.tasks, items, "Tasks")
}

// taskItem builds a list item from a task with status/priority styling.
func (a *app) taskItem(tk *models.Task) item {
	it := item{
		id:    tk.ID,
		kind:  "task",
		title: statusToken(tk.Status) + " " + tk.Title,
		raw:   tk,
	}
	var bits []string
	if tk.Priority != models.PriorityMedium {
		bits = append(bits, string(tk.Priority))
	}
	if tk.GitHubIssue != nil {
		bits = append(bits, fmt.Sprintf("GH#%d", *tk.GitHubIssue))
	}
	if tk.ProjectID != nil {
		if name, ok := a.projectNames[*tk.ProjectID]; ok {
			it.badge = "#" + name
			bits = append(bits, "#"+name)
		}
	}
	if len(tk.Tags) > 0 {
		bits = append(bits, ":"+strings.Join(tk.Tags, " :"))
	}
	if tk.DueAt != nil {
		bits = append(bits, "due "+shortDate(*tk.DueAt))
	}
	it.sub = strings.Join(bits, "  ") + "  " + shortID(tk.ID)
	return it
}

// reloadProjects populates the Projects list with open-task counts.
func (a *app) reloadProjects() {
	projects, err := a.store.ListProjects(a.ctx, nil)
	if err != nil {
		a.statusMsg = "load projects: " + err.Error()
		return
	}
	tasks, _ := a.store.ListTasks(a.ctx, store.TaskQuery{})
	counts := openTaskCounts(tasks)
	items := make([]item, 0, len(projects))
	for _, p := range projects {
		sub := fmt.Sprintf("%s  •  %d open", p.Status, counts[p.ID])
		if p.GitHubRepo != nil {
			sub += "  •  " + *p.GitHubRepo
		}
		items = append(items, item{
			id: p.ID, kind: "project", title: p.Name, sub: sub, badge: shortID(p.ID), raw: p,
		})
	}
	a.setItems(&a.projects, items, "Projects")
}

// reloadInitiatives populates the Initiatives list.
func (a *app) reloadInitiatives() {
	inits, err := a.store.ListInitiatives(a.ctx)
	if err != nil {
		a.statusMsg = "load initiatives: " + err.Error()
		return
	}
	items := make([]item, 0, len(inits))
	for _, i := range inits {
		sub := i.Status
		if i.Outcome != "" {
			sub += "  •  " + i.Outcome
		}
		if i.TargetAt != nil {
			sub += "  •  by " + shortDate(*i.TargetAt)
		}
		items = append(items, item{
			id: i.ID, kind: "initiative", title: i.Name, sub: sub, badge: shortID(i.ID), raw: i,
		})
	}
	a.setItems(&a.initiatives, items, "Initiatives")
}

// reloadSchedule populates the Schedule list with the next 7 days of blocks.
func (a *app) reloadSchedule() {
	blocks, err := a.store.ListBlocks(a.ctx, weekRange())
	if err != nil {
		a.statusMsg = "load schedule: " + err.Error()
		return
	}
	items := make([]item, 0, len(blocks))
	for _, b := range blocks {
		title := b.Title
		sub := fmt.Sprintf("%s–%s", b.StartsAt.Format("Mon 15:04"), b.EndsAt.Format("15:04"))
		if b.TaskID != nil {
			sub += "  •  task " + shortID(*b.TaskID)
		}
		if b.OutlookEventID != nil {
			sub += "  •  outlook ✓"
		}
		items = append(items, item{
			id: b.ID, kind: "block", title: title, sub: sub + "  " + shortID(b.ID), badge: shortID(b.ID), raw: b,
		})
	}
	a.setItems(&a.schedule, items, "Schedule")
}

// setItems replaces a list's items. The internal item type implements list.Item.
func (a *app) setItems(l *list.Model, items []item, title string) {
	if l == nil {
		return
	}
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}
	_ = l.SetItems(listItems)
	l.Title = title
}

// weekRange returns the [start-of-today, +7days) window for the Schedule view.
func weekRange() *store.BlockRange {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return &store.BlockRange{From: start, To: start.Add(7 * 24 * time.Hour)}
}

// openTaskCounts returns project-id → count of non-finished tasks.
func openTaskCounts(tasks []*models.Task) map[string]int {
	m := make(map[string]int)
	for _, tk := range tasks {
		if tk.ProjectID == nil || tk.Status == models.StatusDone || tk.Status == models.StatusCancelled {
			continue
		}
		m[*tk.ProjectID]++
	}
	return m
}

// statusToken returns a compact glyph for a task status (mirrors the CLI).
func statusToken(s models.TaskStatus) string {
	switch s {
	case models.StatusTodo:
		return "○"
	case models.StatusDoing:
		return "◐"
	case models.StatusDone:
		return "✓"
	case models.StatusBlocked:
		return "✗"
	case models.StatusInbox:
		return "?"
	case models.StatusCancelled:
		return "–"
	}
	return " "
}

// shortID returns the last 6 chars of an ID.
func shortID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

// shortDate renders a short relative-or-absolute date label.
func shortDate(t interface{ Format(string) string }) string {
	return t.Format("Jan 2")
}
