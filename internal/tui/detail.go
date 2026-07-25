package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
)

// detailState holds the item being viewed in the detail overlay.
type detailState struct {
	it item
}

// openDetail switches to detail mode for the selected item.
func (a *app) openDetail(it item) (tea.Model, tea.Cmd, bool) {
	a.mode = modeDetail
	a.detail = &detailState{it: it}
	return a, nil, true
}

// detailUpdate handles keys in detail mode (any key returns to browse).
func (a *app) detailUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		a.mode = modeBrowse
		a.detail = nil
		return a, nil
	}
	// Allow e (edit) and x (delete) from within detail too.
	switch {
	case key.Matches(msg, km.Edit):
		d := a.detail
		a.detail = nil
		m, _, _ := a.actEdit(d.it)
		return m, nil
	case key.Matches(msg, km.Delete):
		d := a.detail
		a.detail = nil
		m, _, _ := a.actDelete(d.it)
		return m, nil
	}
	return a, nil
}

// renderDetail draws the full item detail overlay.
func (a *app) renderDetail() string {
	if a.detail == nil {
		return ""
	}
	it := a.detail.it
	var b strings.Builder
	b.WriteString(t.bold.Render(it.title) + "\n\n")
	switch it.kind {
	case "task":
		a.renderTaskDetail(&b, it.raw.(*models.Task))
	case "project":
		a.renderProjectDetail(&b, it.raw.(*models.Project))
	case "initiative":
		a.renderInitiativeDetail(&b, it.raw.(*models.Initiative))
	case "block":
		a.renderBlockDetail(&b, it.raw.(*models.Block))
	}
	b.WriteString("\n" + t.hint.Render("[esc/q] back   [e] edit   [x] delete"))
	return b.String()
}

func (a *app) renderTaskDetail(b *strings.Builder, tk *models.Task) {
	row := func(k, v string) { fmt.Fprintf(b, "%s %s\n", t.label.Render(k+":"), v) }
	row("id", tk.ID)
	row("status", string(tk.Status))
	row("priority", string(tk.Priority))
	if tk.ProjectID != nil {
		if name, ok := a.projectNames[*tk.ProjectID]; ok {
			row("project", "#"+name)
		} else {
			row("project", *tk.ProjectID)
		}
	}
	if tk.InitiativeID != nil {
		row("initiative", *tk.InitiativeID)
	}
	if len(tk.Tags) > 0 {
		row("tags", strings.Join(tk.Tags, ", "))
	}
	if tk.DueAt != nil {
		row("due", tk.DueAt.Local().Format("Mon Jan 2 15:04"))
	}
	if tk.GitHubRepo != nil && tk.GitHubIssue != nil {
		row("github", fmt.Sprintf("%s#%d", *tk.GitHubRepo, *tk.GitHubIssue))
	}
	if tk.Notes != "" {
		b.WriteString("\n" + tk.Notes + "\n")
	}
}

func (a *app) renderProjectDetail(b *strings.Builder, p *models.Project) {
	row := func(k, v string) { fmt.Fprintf(b, "%s %s\n", t.label.Render(k+":"), v) }
	row("id", p.ID)
	row("status", p.Status)
	if p.Description != "" {
		row("description", p.Description)
	}
	if p.GitHubRepo != nil {
		row("github", *p.GitHubRepo)
	}
	if p.InitiativeID != nil {
		row("initiative", *p.InitiativeID)
	}
	// Count tasks.
	pid := p.ID
	tasks, _ := a.store.ListTasks(a.ctx, store.TaskQuery{Filter: store.TaskFilter{ProjectID: &pid}})
	open := 0
	for _, tk := range tasks {
		if tk.Status != models.StatusDone && tk.Status != models.StatusCancelled {
			open++
		}
	}
	row("tasks", fmt.Sprintf("%d open / %d total", open, len(tasks)))
}

func (a *app) renderInitiativeDetail(b *strings.Builder, i *models.Initiative) {
	row := func(k, v string) { fmt.Fprintf(b, "%s %s\n", t.label.Render(k+":"), v) }
	row("id", i.ID)
	row("status", i.Status)
	if i.Outcome != "" {
		row("outcome", i.Outcome)
	}
	if i.TargetAt != nil {
		row("target", i.TargetAt.Format("Mon Jan 2"))
	}
}

func (a *app) renderBlockDetail(b *strings.Builder, blk *models.Block) {
	row := func(k, v string) { fmt.Fprintf(b, "%s %s\n", t.label.Render(k+":"), v) }
	row("id", blk.ID)
	row("when", fmt.Sprintf("%s → %s", blk.StartsAt.Format("Mon Jan 2 15:04"), blk.EndsAt.Format("15:04")))
	if blk.TaskID != nil {
		row("task", *blk.TaskID)
	}
	if blk.OutlookEventID != nil {
		row("outlook", "synced ✓")
	}
	if blk.Notes != "" {
		row("notes", blk.Notes)
	}
}

