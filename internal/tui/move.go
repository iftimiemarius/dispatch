package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
)

// moveState is the project picker shown when moving a task (press `m`). It
// lists all projects plus an "Inbox (no project)" option; selecting one
// reassigns the task and returns to browse mode.
type moveState struct {
	task      *models.Task
	projects  []*models.Project
	cursor    int
	// titles rendered for each option (option 0 is always "Inbox").
}

// actMoveTask opens the project picker for the selected task.
func (a *app) actMoveTask(it item) (tea.Model, tea.Cmd, bool) {
	projects, err := a.store.ListProjects(a.ctx, nil)
	if err != nil {
		a.statusMsg = "load projects: " + err.Error()
		return a, nil, true
	}
	a.mode = modeMove
	a.move = &moveState{
		task:     it.raw.(*models.Task),
		projects: projects,
		cursor:   0,
	}
	// Default cursor to the task's current project, if any.
	if it.raw.(*models.Task).ProjectID != nil {
		for i, p := range projects {
			if p.ID == *it.raw.(*models.Task).ProjectID {
				a.move.cursor = i + 1 // +1 because option 0 is Inbox
				break
			}
		}
	}
	return a, nil, true
}

// moveUpdate handles keys in the project picker.
func (a *app) moveUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m := a.move
	count := len(m.projects) + 1 // projects + Inbox option
	switch msg.String() {
	case "esc", "q":
		a.statusMsg = "cancelled"
		a.mode = modeBrowse
		a.move = nil
		return a, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < count-1 {
			m.cursor++
		}
	case "enter":
		return a.moveApply()
	}
	return a, nil
}

// moveApply reassigns the task to the chosen project (or Inbox) and saves.
func (a *app) moveApply() (tea.Model, tea.Cmd) {
	m := a.move
	if m.cursor == 0 {
		m.task.ProjectID = nil
		a.statusMsg = "moved to inbox"
	} else {
		p := m.projects[m.cursor-1]
		m.task.ProjectID = &p.ID
		a.statusMsg = "moved to " + p.Name
	}
	m.task.UpdatedAt = time.Now().UTC()
	if err := a.store.UpdateTask(a.ctx, m.task); err != nil {
		a.statusMsg = "error: " + err.Error()
	}
	a.mode = modeBrowse
	a.move = nil
	a.reloadAll()
	return a, nil
}

// renderMove draws the project picker overlay.
func (a *app) renderMove() string {
	m := a.move
	if m == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(t.bold.Render("Move task to project") + "\n")
	b.WriteString(t.dim.Render(m.task.Title) + "\n\n")
	// Option 0: Inbox.
	b.WriteString(a.moveRow(0, "Inbox", "(no project)"))
	for i, p := range m.projects {
		desc := p.Status
		if p.GitHubRepo != nil {
			desc += "  •  " + *p.GitHubRepo
		}
		b.WriteString(a.moveRow(i+1, p.Name, desc))
	}
	b.WriteString("\n" + t.hint.Render("↑↓ navigate • Enter select • Esc cancel"))
	return b.String()
}

func (a *app) moveRow(idx int, name, desc string) string {
	cursor := " "
	if idx == a.move.cursor {
		cursor = t.accent.Render("▸")
	}
	return fmt.Sprintf("%s %s  %s\n", cursor, t.bold.Render(name), t.dim.Render(desc))
}

// keep store referenced (used by actMoveTask above).
var _ = store.ErrNotFound
