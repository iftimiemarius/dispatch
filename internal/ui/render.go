// Package ui provides terminal rendering helpers: colored tables and labels
// for rendering tasks, projects, initiatives, and blocks.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/iftimiemarius/dispatch/internal/models"
)

// Styling primitives (kept subtle so output stays readable in any theme).
var (
	bold       = lipgloss.NewStyle().Bold(true)
	dim        = lipgloss.NewStyle().Faint(true)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	header     = lipgloss.NewStyle().Bold(true).Faint(true)

	statusDone  = lipgloss.NewStyle().Faint(true).Strikethrough(true)
	statusDoing = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	statusBlock = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	statusInbox = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

// priorityStyle maps a priority to a color.
func priorityStyle(p models.Priority) lipgloss.Style {
	switch p {
	case models.PriorityUrgent:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	case models.PriorityHigh:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	case models.PriorityMedium:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	}
}

// statusStyle maps a task status to a style.
func statusStyle(s models.TaskStatus) lipgloss.Style {
	switch s {
	case models.StatusDone:
		return statusDone
	case models.StatusDoing:
		return statusDoing
	case models.StatusBlocked:
		return statusBlock
	case models.StatusInbox:
		return statusInbox
	default:
		return lipgloss.NewStyle()
	}
}

// PriorityLabel is a compact 4-char-ish token for columns.
func PriorityLabel(p models.Priority) string {
	switch p {
	case models.PriorityUrgent:
		return "URGENT"
	case models.PriorityHigh:
		return "high"
	case models.PriorityMedium:
		return "med"
	case models.PriorityLow:
		return "low"
	}
	return string(p)
}

// TaskRow renders a single task as a one-line row.
type TaskRow struct {
	ID       string
	Priority models.Priority
	Status   models.TaskStatus
	Title    string
	Tags     []string
	DueAt    *time.Time
	Project  string // resolved project name, optional
}

// RenderTaskTable renders a header plus rows for the given tasks.
func RenderTaskTable(rows []TaskRow) string {
	if len(rows) == 0 {
		return dim.Render("No tasks.")
	}
	var b strings.Builder
	for _, r := range rows {
		statusTok := statusToken(r.Status)
		prio := priorityStyle(r.Priority).Render(PriorityLabel(r.Priority))
		id := dim.Render(r.ID)
		title := r.Title
		if r.Status == models.StatusDone {
			title = statusDone.Render(title)
		}
		var parts []string
		parts = append(parts, fmt.Sprintf("%-8s", id))
		parts = append(parts, fmt.Sprintf("%-7s", statusTok))
		parts = append(parts, fmt.Sprintf("%-7s", prio))
		parts = append(parts, title)
		if r.Project != "" {
			parts = append(parts, dim.Render("#"+r.Project))
		}
		if len(r.Tags) > 0 {
			tags := make([]string, len(r.Tags))
			for i, t := range r.Tags {
				tags[i] = ":" + t
			}
			parts = append(parts, dim.Render(strings.Join(tags, " ")))
		}
		if r.DueAt != nil {
			parts = append(parts, dueLabel(*r.DueAt))
		}
		b.WriteString(strings.Join(parts, "  "))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func statusToken(s models.TaskStatus) string {
	switch s {
	case models.StatusTodo:
		return "[ ]"
	case models.StatusDoing:
		return statusDoing.Render("[~]")
	case models.StatusDone:
		return "[x]"
	case models.StatusBlocked:
		return statusBlock.Render("[!]")
	case models.StatusInbox:
		return statusInbox.Render("[?]")
	case models.StatusCancelled:
		return dim.Render("[-]")
	}
	return string(s)
}

func dueLabel(t time.Time) string {
	days := int(time.Until(t).Hours() / 24)
	style := lipgloss.NewStyle()
	switch {
	case days < 0:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case days == 0:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	case days <= 2:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
	default:
		style = dim
	}
	label := "due " + RelativeDate(t)
	return style.Render(label)
}

// RelativeDate turns a time into a short human label: "today", "tomorrow",
// "in 3d", "2d ago", or an absolute date for far times.
func RelativeDate(t time.Time) string {
	t = t.Local()
	now := time.Now().Local()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	delta := int(day.Sub(today).Hours() / 24)
	switch delta {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	case -1:
		return "yesterday"
	}
	if delta > 1 && delta <= 7 {
		return fmt.Sprintf("in %dd", delta)
	}
	if delta < -1 {
		return fmt.Sprintf("%dd ago", -delta)
	}
	return t.Format("Jan 2")
}

// Section renders a titled section header.
func Section(name string) string {
	return titleStyle.Render(name)
}

// Bold and Dim are exposed for command output.
func Bold(s string) string  { return bold.Render(s) }
func Dim(s string) string    { return dim.Render(s) }
func Header(s string) string { return header.Render(s) }
