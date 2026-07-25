// Package tui implements Dispatch's full-screen interactive terminal UI.
//
// It is a Bubble Tea program following the Elm architecture: the app Model
// holds the active view (tasks/projects/initiatives/schedule) and a mode
// (browse/form/detail). Key messages drive view switching and item actions;
// data is read/written through the existing internal/store layer so the TUI
// and CLI stay perfectly in sync.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
)

// view identifies a top-level tab.
type view int

const (
	viewTasks view = iota
	viewProjects
	viewInitiatives
	viewSchedule
	viewCount
)

func (v view) name() string {
	switch v {
	case viewTasks:
		return "Tasks"
	case viewProjects:
		return "Projects"
	case viewInitiatives:
		return "Initiatives"
	case viewSchedule:
		return "Schedule"
	}
	return ""
}

// mode is the interaction state within a view.
type mode int

const (
	modeBrowse mode = iota
	modeForm
	modeConfirm
	modeDetail
)

// app is the root Bubble Tea model.
type app struct {
	ctx    context.Context
	store  *store.Store
	active view
	mode   mode
	form   *formState     // non-nil in modeForm
	confirm *confirmState // non-nil in modeConfirm
	detail *detailState   // non-nil in modeDetail

	width, height int

	tasks        list.Model
	projects     list.Model
	initiatives  list.Model
	schedule     list.Model
	help         help.Model
	statusMsg    string // transient message shown in the status bar

	// Lazy-populated lookups for richer rendering.
	projectNames map[string]string
}

// newItem is a generic list item wrapping a dispatch entity. It implements
// list.Item and carries enough to render and act on rows across views.
type item struct {
	id     string // entity ID (full)
	kind   string // "task" | "project" | "initiative" | "block"
	title  string
	sub    string // secondary line (status, counts, time)
	badge  string // e.g. "GH#42" or "#api"
	raw    any    // the underlying *models.Task/Project/...
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.sub }
func (i item) FilterValue() string { return i.title + " " + i.sub + " " + i.badge }

// newApp builds the root model and loads initial data. Errors during load are
// non-fatal — the view shows an empty list with a status message.
func newApp(ctx context.Context, st *store.Store) *app {
	a := &app{
		ctx:          ctx,
		store:        st,
		active:       viewTasks,
		mode:         modeBrowse,
		help:         help.New(),
		projectNames: loadProjectNames(ctx, st),
	}
	a.tasks = a.newList("Tasks")
	a.projects = a.newList("Projects")
	a.initiatives = a.newList("Initiatives")
	a.schedule = a.newList("Schedule")
	a.reloadAll()
	return a
}

// newList returns a configured list.Model for a tab title.
func (a *app) newList(title string) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("63")).BorderLeftForeground(lipgloss.Color("63"))
	delegate.Styles.NormalTitle = lipgloss.NewStyle()
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Faint(true)
	l := list.New(nil, delegate, 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = t.bold.Padding(0, 1)
	return l
}

// Init starts the program.
func (a *app) Init() tea.Cmd { return nil }

// Update handles messages.
func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.resizeLists()
		return a, nil

	case tea.KeyMsg:
		// If a list is in filter-input mode, forward keys to it.
		if cur := a.curList(); cur != nil && cur.FilterState() == list.Filtering {
			m, cmd := cur.Update(msg)
			*cur = m
			return a, cmd
		}
		return a.handleKey(msg)
	}

	// Forward non-key messages (pagination, etc.) to the current list.
	if cur := a.curList(); cur != nil {
		m, cmd := cur.Update(msg)
		*cur = m
		return a, cmd
	}
	return a, nil
}

// handleKey dispatches top-level keys.
func (a *app) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Form mode handles its own keys and swallows everything else.
	if a.mode == modeForm && a.form != nil {
		return a.formUpdate(msg)
	}
	// Confirm mode intercepts y/n for delete confirmation.
	if a.mode == modeConfirm && a.confirm != nil {
		return a.confirmUpdate(msg)
	}
	// Detail mode handles its own keys.
	if a.mode == modeDetail && a.detail != nil {
		return a.detailUpdate(msg)
	}

	switch {
	case key.Matches(msg, km.ForceQuit):
		return a, tea.Quit
	case key.Matches(msg, km.Quit):
		if cur := a.curList(); cur != nil && cur.FilterState() == list.Filtering {
			cur.ResetFilter()
			return a, nil
		}
		return a, tea.Quit
	case key.Matches(msg, km.Help):
		a.help.ShowAll = !a.help.ShowAll
		return a, nil
	case key.Matches(msg, km.NextTab):
		a.active = (a.active + 1) % viewCount
		a.mode = modeBrowse
		return a, nil
	case key.Matches(msg, km.PrevTab):
		a.active = (a.active - 1 + viewCount) % viewCount
		a.mode = modeBrowse
		return a, nil
	}

	// Action keys (only in browse mode, on a selected item).
	if a.mode == modeBrowse {
		if m, cmd, handled := a.handleAction(msg); handled {
			return m, cmd
		}
	}

	// Otherwise, forward navigation keys to the current list.
	if cur := a.curList(); cur != nil {
		m, cmd := cur.Update(msg)
		*cur = m
		return a, cmd
	}
	return a, nil
}

// View renders the screen.
func (a *app) View() string {
	if a.height == 0 {
		return "loading…"
	}
	var b strings.Builder
	b.WriteString(a.renderTabs())
	b.WriteByte('\n')

	// Form mode replaces the list body with the editor.
	if a.mode == modeForm && a.form != nil {
		b.WriteString(a.renderForm())
		return b.String()
	}
	// Detail mode replaces the list body with the full-item view.
	if a.mode == modeDetail && a.detail != nil {
		b.WriteString(a.renderDetail())
		return b.String()
	}
	// Confirm mode overlays on top of the (dimmed) list body.
	cur := a.curList()
	if cur != nil {
		b.WriteString(cur.View())
	}
	if a.mode == modeConfirm {
		b.WriteString(a.renderConfirm())
	}
	b.WriteByte('\n')
	b.WriteString(a.renderStatus())
	b.WriteByte('\n')
	b.WriteString(a.renderHelp())
	return b.String()
}

// renderTabs draws the top tab bar.
func (a *app) renderTabs() string {
	var tabs []string
	for v := view(0); v < viewCount; v++ {
		label := v.name()
		if v == a.active {
			tabs = append(tabs, t.tabActive.Render("▸ "+label))
		} else {
			tabs = append(tabs, t.dim.Render("  "+label))
		}
	}
	bar := strings.Join(tabs, "  ")
	return lipgloss.JoinHorizontal(lipgloss.Left, bar)
}

// renderStatus shows the transient status message (if any).
func (a *app) renderStatus() string {
	if a.statusMsg != "" {
		return t.statusBar.Render(a.statusMsg)
	}
	cur := a.curList()
	if cur != nil {
		count := len(cur.Items())
		return t.statusBar.Render(fmt.Sprintf("%d items", count))
	}
	return ""
}

// renderHelp shows the one-line key hint.
func (a *app) renderHelp() string {
	return t.helpBar.Render(a.help.View(helpKeyMap{}))
}

// --- list accessors ---

// curList returns a pointer to the active view's list model.
func (a *app) curList() *list.Model {
	switch a.active {
	case viewTasks:
		return &a.tasks
	case viewProjects:
		return &a.projects
	case viewInitiatives:
		return &a.initiatives
	case viewSchedule:
		return &a.schedule
	}
	return nil
}

func (a *app) resizeLists() {
	// Reserve 4 lines: tab bar, list title, status bar, help bar.
	h := a.height - 4
	if h < 3 {
		h = 3
	}
	w := a.width
	for _, l := range []*list.Model{&a.tasks, &a.projects, &a.initiatives, &a.schedule} {
		l.SetSize(w, h)
	}
}

// selectedItem returns the currently-highlighted item, if any.
func (a *app) selectedItem() (item, bool) {
	cur := a.curList()
	if cur == nil {
		return item{}, false
	}
	sel := cur.SelectedItem()
	if sel == nil {
		return item{}, false
	}
	it, ok := sel.(item)
	return it, ok
}

// loadProjectNames returns a project-id → name map for resolving task rows.
func loadProjectNames(ctx context.Context, st *store.Store) map[string]string {
	projects, err := st.ListProjects(ctx, nil)
	if err != nil {
		return map[string]string{}
	}
	m := make(map[string]string, len(projects))
	for _, p := range projects {
		m[p.ID] = strings.ToLower(p.Name)
	}
	return m
}

// keep models import referenced (used by reload* functions in view files).
var _ = models.StatusTodo
