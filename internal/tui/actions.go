package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iftimiemarius/dispatch/internal/models"
)

// handleAction dispatches per-item action keys (done/start/delete/move/new/edit)
// when in browse mode. Returns handled=true if the key was consumed.
func (a *app) handleAction(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	it, hasSel := a.selectedItem()
	switch {
	case key.Matches(msg, km.Done) && hasSel && it.kind == "task":
		return a.actTaskStatus(it, models.StatusDone, "done")
	case key.Matches(msg, km.Start) && hasSel && it.kind == "task":
		return a.actTaskStatus(it, models.StatusDoing, "in-progress")
	case key.Matches(msg, km.Delete) && hasSel:
		return a.actDelete(it)
	case key.Matches(msg, km.New):
		return a.actNew()
	case key.Matches(msg, km.Edit) && hasSel:
		return a.actEdit(it)
	}
	return a, nil, false
}

// actTaskStatus flips a task's status and reloads.
func (a *app) actTaskStatus(it item, status models.TaskStatus, label string) (tea.Model, tea.Cmd, bool) {
	tk := it.raw.(*models.Task)
	if status == models.StatusDone && tk.Status == models.StatusDone {
		// toggle: reopen
		tk.Status = models.StatusTodo
		tk.CompletedAt = nil
		a.statusMsg = "reopened"
	} else {
		tk.Status = status
		if status == models.StatusDone {
			now := time.Now().UTC()
			tk.CompletedAt = &now
		}
		a.statusMsg = label
	}
	tk.UpdatedAt = time.Now().UTC()
	if err := a.store.UpdateTask(a.ctx, tk); err != nil {
		a.statusMsg = "error: " + err.Error()
	}
	a.reloadAll()
	return a, nil, true
}

// actDelete starts a confirmation for the selected item.
func (a *app) actDelete(it item) (tea.Model, tea.Cmd, bool) {
	a.mode = modeConfirm
	a.confirm = &confirmState{
		itemKind: it.kind,
		itemTitle: it.title,
		onConfirm: func() {
			switch it.kind {
			case "task":
				_ = a.store.DeleteTask(a.ctx, it.id)
			case "project":
				_ = a.store.DeleteProject(a.ctx, it.id)
			case "initiative":
				_ = a.store.DeleteInitiative(a.ctx, it.id)
			case "block":
				_ = a.store.DeleteBlock(a.ctx, it.id)
			}
			a.statusMsg = "deleted"
			a.reloadAll()
		},
	}
	return a, nil, true
}

// confirmState holds the pending delete confirmation.
type confirmState struct {
	itemKind  string
	itemTitle string
	onConfirm func()
}

// confirmUpdate handles y/n during a delete confirmation.
func (a *app) confirmUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		a.confirm.onConfirm()
	case "n", "N", "esc":
		a.statusMsg = "cancelled"
	}
	a.mode = modeBrowse
	a.confirm = nil
	return a, nil
}

// actNew opens the new-item form for the current view's entity type.
func (a *app) actNew() (tea.Model, tea.Cmd, bool) {
	kind := a.activeKind()
	a.mode = modeForm
	a.form = newFormState(a, kind, nil) // nil = create new
	return a, nil, true
}

// actEdit opens the edit form for the selected item.
func (a *app) actEdit(it item) (tea.Model, tea.Cmd, bool) {
	a.mode = modeForm
	a.form = newFormState(a, it.kind, it.raw)
	return a, nil, true
}

// activeKind maps the active view to its entity kind string.
func (a *app) activeKind() string {
	switch a.active {
	case viewTasks:
		return "task"
	case viewProjects:
		return "project"
	case viewInitiatives:
		return "initiative"
	case viewSchedule:
		return "block"
	}
	return "task"
}

// setStatusMsg sets a transient status message.
func (a *app) setStatusMsg(s string) { a.statusMsg = s }

// afterSave reloads all views, clears the form, returns to browse mode.
func (a *app) afterSave() {
	a.reloadAll()
	a.mode = modeBrowse
	a.form = nil
}

// renderConfirm renders the delete-confirmation overlay.
func (a *app) renderConfirm() string {
	if a.confirm == nil {
		return ""
	}
	return "\n  " + t.bold.Render("Delete "+a.confirm.itemKind+"?") +
		"  " + a.confirm.itemTitle + "\n" +
		"  " + t.dim.Render("[y] yes   [n/esc] cancel")
}

// keep imports referenced.
var (
	_ = fmt.Sprintf
	_ = strings.Join
	_ = list.Item(nil)
	_ = tea.Quit
)