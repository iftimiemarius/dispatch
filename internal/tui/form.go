package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/timeparse"
)

// formState is the in-TUI editor for a single entity. It owns a set of fields
// (text inputs + enum cyclers) and, on save, writes through the store.
type formState struct {
	app      *app
	kind     string // "task" | "project" | "initiative" | "block"
	editing  any    // the existing *models.Task/... (nil = creating new)

	fields   []*formField // ordered; Tab cycles focus
	focus    int
	errMsg   string
}

// formField is one editable row: either a free-text input or a cycler enum.
type formField struct {
	label    string
	input    *textinput.Model // non-nil for text fields
	cycler   *enumCycler      // non-nil for enum fields
	isText   bool
}

// enumCycler holds a fixed set of options and the current index.
type enumCycler struct {
	options []string
	current int
}

func (c *enumCycler) value() string { return c.options[c.current] }
func (c *enumCycler) next()         { c.current = (c.current + 1) % len(c.options) }
func (c *enumCycler) prev() {
	c.current = (c.current - 1 + len(c.options)) % len(c.options)
}

// newFormState builds a form for the given kind. If existing is nil, the form
// creates a new entity; otherwise it edits the provided one.
func newFormState(a *app, kind string, existing any) *formState {
	f := &formState{app: a, kind: kind, editing: existing}
	f.buildFields()
	if len(f.fields) > 0 {
		f.fields[0].focus(true)
	}
	return f
}

func (f *formState) buildFields() {
	switch f.kind {
	case "task":
		f.buildTaskFields()
	case "project":
		f.buildProjectFields()
	case "initiative":
		f.buildInitiativeFields()
	case "block":
		f.buildBlockFields()
	}
}

func (f *formState) addText(label, value, placeholder string) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.CharLimit = 120
	ti.Width = 50
	f.fields = append(f.fields, &formField{label: label, input: &ti, isText: true})
}

func (f *formState) addEnum(label string, options []string, current string) {
	idx := 0
	for i, o := range options {
		if o == current {
			idx = i
			break
		}
	}
	f.fields = append(f.fields, &formField{
		label:  label,
		cycler: &enumCycler{options: options, current: idx},
	})
}

func (f *formState) buildTaskFields() {
	var t *models.Task
	if f.editing != nil {
		t = f.editing.(*models.Task)
	}
	title, notes, prio, status, due, tags := "", "", "medium", "inbox", "", ""
	if t != nil {
		title, notes, prio, status = t.Title, t.Notes, string(t.Priority), string(t.Status)
		if t.DueAt != nil {
			due = t.DueAt.Format("2006-01-02 15:04")
		}
		tags = strings.Join(t.Tags, ", ")
	}
	f.addText("Title", title, "what needs doing?")
	f.addText("Notes", notes, "details")
	f.addEnum("Priority", []string{"low", "medium", "high", "urgent"}, prio)
	f.addEnum("Status", []string{"inbox", "todo", "doing", "done", "blocked", "cancelled"}, status)
	f.addText("Due", due, "e.g. tomorrow 9am, fri, +2h")
	f.addText("Tags", tags, "comma-separated")
}

func (f *formState) buildProjectFields() {
	var p *models.Project
	if f.editing != nil {
		p = f.editing.(*models.Project)
	}
	name, desc, ghRepo := "", "", ""
	if p != nil {
		name, desc = p.Name, p.Description
		if p.GitHubRepo != nil {
			ghRepo = *p.GitHubRepo
		}
	}
	f.addText("Name", name, "project name")
	f.addText("Description", desc, "what is it?")
	f.addText("GitHub repo", ghRepo, "owner/name")
}

func (f *formState) buildInitiativeFields() {
	var i *models.Initiative
	if f.editing != nil {
		i = f.editing.(*models.Initiative)
	}
	name, outcome := "", ""
	if i != nil {
		name, outcome = i.Name, i.Outcome
	}
	f.addText("Name", name, "initiative name")
	f.addText("Outcome", outcome, "the outcome it drives")
}

func (f *formState) buildBlockFields() {
	var b *models.Block
	if f.editing != nil {
		b = f.editing.(*models.Block)
	}
	title, from, dur := "", "", "30m"
	autoSync := "yes"
	if b != nil {
		title = b.Title
		from = b.StartsAt.Format("15:04")
		dur = fmt.Sprintf("%dm", int(b.EndsAt.Sub(b.StartsAt).Minutes()))
		if !b.AutoSync {
			autoSync = "no"
		}
	}
	f.addText("Title", title, "focus block")
	f.addText("From", from, "e.g. 9am or 14:30")
	f.addText("Duration", dur, "e.g. 30m, 2h")
	f.addEnum("Sync to Outlook", []string{"yes", "no"}, autoSync)
}

// --- key handling ---

func (f *formField) focus(on bool) {
	if f.isText && f.input != nil {
		if on {
			f.input.Focus()
		} else {
			f.input.Blur()
		}
	}
}

// formUpdate handles keys while the form is active.
func (a *app) formUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := a.form
	if f == nil {
		a.mode = modeBrowse
		return a, nil
	}
	switch msg.String() {
	case "esc":
		a.statusMsg = "cancelled"
		a.mode = modeBrowse
		a.form = nil
		return a, nil
	case "ctrl+c":
		return a, tea.Quit
	case "enter":
		// Enter on the last field saves; otherwise moves to the next field.
		if f.focus == len(f.fields)-1 {
			return a.formSave()
		}
		f.nextField()
		return a, nil
	case "tab":
		f.nextField()
		return a, nil
	case "shift+tab":
		f.prevField()
		return a, nil
	case "up", "k":
		f.prevField()
		return a, nil
	case "down", "j":
		f.nextField()
		return a, nil
	case "left", "h":
		// On enum fields, cycle the selection left; on text fields, fall
		// through so the textinput handles cursor movement.
		cur := f.fields[f.focus]
		if !cur.isText && cur.cycler != nil {
			cur.cycler.prev()
			return a, nil
		}
	case "right", "l":
		cur := f.fields[f.focus]
		if !cur.isText && cur.cycler != nil {
			cur.cycler.next()
			return a, nil
		}
	}

	// Forward typing (and left/right on text fields) to the focused input.
	cur := f.fields[f.focus]
	if cur.isText && cur.input != nil {
		m, cmd := cur.input.Update(msg)
		*cur.input = m
		return a, cmd
	}
	return a, nil
}

func (f *formState) nextField() {
	f.fields[f.focus].focus(false)
	f.focus = (f.focus + 1) % len(f.fields)
	f.fields[f.focus].focus(true)
}

func (f *formState) prevField() {
	f.fields[f.focus].focus(false)
	f.focus = (f.focus - 1 + len(f.fields)) % len(f.fields)
	f.fields[f.focus].focus(true)
}

// fieldValue returns the string value of a field by label (first match).
func (f *formState) fieldValue(label string) string {
	for _, fl := range f.fields {
		if fl.label == label {
			if fl.isText {
				return fl.input.Value()
			}
			return fl.cycler.value()
		}
	}
	return ""
}

// formSave writes the form values to the store.
func (a *app) formSave() (tea.Model, tea.Cmd) {
	f := a.form
	now := time.Now().UTC()
	switch f.kind {
	case "task":
		return a.saveTask(f, now)
	case "project":
		return a.saveProject(f, now)
	case "initiative":
		return a.saveInitiative(f, now)
	case "block":
		return a.saveBlock(f, now)
	}
	a.mode = modeBrowse
	a.form = nil
	return a, nil
}

func (a *app) saveTask(f *formState, now time.Time) (tea.Model, tea.Cmd) {
	title := f.fieldValue("Title")
	if title == "" {
		f.errMsg = "title is required"
		return a, nil
	}
	var t *models.Task
	if f.editing != nil {
		t = f.editing.(*models.Task)
	} else {
		t = &models.Task{ID: models.NewID(), Status: models.StatusInbox, Priority: models.PriorityMedium, CreatedAt: now}
	}
	t.Title = title
	t.Notes = f.fieldValue("Notes")
	t.Priority = models.Priority(f.fieldValue("Priority"))
	t.Status = models.TaskStatus(f.fieldValue("Status"))
	if due := f.fieldValue("Due"); due != "" {
		parsed, err := timeparse.Parse(due, time.Now())
		if err == nil && !parsed.IsZero() {
			t.DueAt = &parsed
		}
	}
	tagStr := f.fieldValue("Tags")
	t.Tags = nil
	for _, tag := range strings.Split(tagStr, ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			t.Tags = append(t.Tags, tag)
		}
	}
	t.UpdatedAt = now
	if err := a.store.CreateOrUpdateTask(a.ctx, t); err != nil {
		f.errMsg = err.Error()
		return a, nil
	}
	a.statusMsg = "saved"
	a.afterSave()
	return a, nil
}

func (a *app) saveProject(f *formState, now time.Time) (tea.Model, tea.Cmd) {
	name := f.fieldValue("Name")
	if name == "" {
		f.errMsg = "name is required"
		return a, nil
	}
	var p *models.Project
	if f.editing != nil {
		p = f.editing.(*models.Project)
	} else {
		p = &models.Project{ID: models.NewID(), Status: "active", CreatedAt: now}
	}
	p.Name = name
	p.Description = f.fieldValue("Description")
	if gh := f.fieldValue("GitHub repo"); gh != "" {
		p.GitHubRepo = &gh
	} else {
		p.GitHubRepo = nil
	}
	p.UpdatedAt = now
	if err := a.store.CreateOrUpdateProject(a.ctx, p); err != nil {
		f.errMsg = err.Error()
		return a, nil
	}
	a.statusMsg = "saved"
	a.afterSave()
	return a, nil
}

func (a *app) saveInitiative(f *formState, now time.Time) (tea.Model, tea.Cmd) {
	name := f.fieldValue("Name")
	if name == "" {
		f.errMsg = "name is required"
		return a, nil
	}
	var i *models.Initiative
	if f.editing != nil {
		i = f.editing.(*models.Initiative)
	} else {
		i = &models.Initiative{ID: models.NewID(), Status: "active", CreatedAt: now}
	}
	i.Name = name
	i.Outcome = f.fieldValue("Outcome")
	i.UpdatedAt = now
	if err := a.store.CreateOrUpdateInitiative(a.ctx, i); err != nil {
		f.errMsg = err.Error()
		return a, nil
	}
	a.statusMsg = "saved"
	a.afterSave()
	return a, nil
}

func (a *app) saveBlock(f *formState, now time.Time) (tea.Model, tea.Cmd) {
	from := f.fieldValue("From")
	if from == "" {
		f.errMsg = "from time is required"
		return a, nil
	}
	start, err := timeparse.Parse(from, time.Now())
	if err != nil || start.IsZero() {
		f.errMsg = "invalid from time"
		return a, nil
	}
	durStr := f.fieldValue("Duration")
	dur, err := time.ParseDuration(durStr)
	if err != nil || dur <= 0 {
		f.errMsg = "invalid duration"
		return a, nil
	}
	var b *models.Block
	if f.editing != nil {
		b = f.editing.(*models.Block)
	} else {
		b = &models.Block{ID: models.NewID(), CreatedAt: now}
	}
	b.Title = f.fieldValue("Title")
	b.StartsAt = start
	b.EndsAt = start.Add(dur)
	b.AutoSync = f.fieldValue("Sync to Outlook") == "yes"
	b.UpdatedAt = now
	if err := a.store.CreateOrUpdateBlock(a.ctx, b); err != nil {
		f.errMsg = err.Error()
		return a, nil
	}
	// Auto-sync: if enabled and Outlook is connected, push the block now.
	if b.AutoSync {
		if id, err := syncBlockToOutlook(a.ctx, b); err == nil {
			b.OutlookEventID = &id
			b.UpdatedAt = now
			_ = a.store.UpdateBlock(a.ctx, b)
			a.statusMsg = "saved • synced to outlook"
		} else if !isOutlookNotConfigured(err) {
			// Connected but failed — surface the error; keep the local block.
			a.statusMsg = "saved • outlook sync failed: " + err.Error()
		} else {
			a.statusMsg = "saved"
		}
	} else {
		a.statusMsg = "saved"
	}
	a.afterSave()
	return a, nil
}

// renderForm draws the editor overlay.
func (a *app) renderForm() string {
	f := a.form
	if f == nil {
		return ""
	}
	var b strings.Builder
	verb := "New"
	if f.editing != nil {
		verb = "Edit"
	}
	b.WriteString(t.bold.Render(fmt.Sprintf("%s %s", verb, f.kind)) + "\n\n")
	// Size the label column to the longest label (+2 for the colon and a gap)
	// so labels never wrap and values always start in a clean column.
	labelW := 0
	for _, fl := range f.fields {
		if l := len(fl.label) + 1; l > labelW {
			labelW = l
		}
	}
	labelStyle := t.label.Width(labelW)
	for i, fl := range f.fields {
		cursor := " "
		if i == f.focus {
			cursor = t.accent.Render("▸")
		}
		var val string
		if fl.isText {
			val = fl.input.View()
		} else {
			// Prefix with the same ">" prompt text inputs show, so every field
			// has a uniform cursor affordance regardless of focus.
			opts := make([]string, len(fl.cycler.options))
			for j, o := range fl.cycler.options {
				if j == fl.cycler.current {
					opts[j] = t.accent.Render("["+o+"]")
				} else {
					opts[j] = t.dim.Render(o)
				}
			}
			val = "> " + strings.Join(opts, " ")
		}
		b.WriteString(fmt.Sprintf("%s %s %s\n", cursor, labelStyle.Render(fl.label+":"), val))
	}
	b.WriteString("\n" + t.hint.Render("↑↓ move field • ←→ cycle • Tab next • Enter save • Esc cancel"))
	if f.errMsg != "" {
		b.WriteString("\n" + t.urgent.Render("  ⚠ "+f.errMsg))
	}
	return b.String()
}

// keep strconv referenced (used by future numeric fields).
var _ = strconv.Atoi
