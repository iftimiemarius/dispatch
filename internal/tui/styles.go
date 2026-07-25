package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds the lipgloss styles the TUI uses. It mirrors the palette from
// internal/ui so the TUI and CLI output feel consistent.
type theme struct {
	// App chrome
	tabBar       lipgloss.Style
	tabActive    lipgloss.Style
	statusBar    lipgloss.Style
	helpBar      lipgloss.Style

	// Content
	title    lipgloss.Style
	subtitle lipgloss.Style
	dim      lipgloss.Style
	bold     lipgloss.Style

	// Statuses
	done     lipgloss.Style
	doing    lipgloss.Style
	blocked  lipgloss.Style
	inbox    lipgloss.Style
	urgent   lipgloss.Style
	high     lipgloss.Style
	medium   lipgloss.Style
	low      lipgloss.Style
	accent   lipgloss.Style // purple, used for selection + GitHub badge

	// Form
	label    lipgloss.Style
	field    lipgloss.Style
	hint     lipgloss.Style
}

var t = newTheme()

func newTheme() theme {
	return theme{
		tabBar:    lipgloss.NewStyle().Padding(0, 1),
		tabActive: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Padding(0, 1),
		statusBar: lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1),
		helpBar:   lipgloss.NewStyle().Faint(true).Padding(0, 1),

		title:    lipgloss.NewStyle().Bold(true),
		subtitle: lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		dim:      lipgloss.NewStyle().Faint(true),
		bold:     lipgloss.NewStyle().Bold(true),

		done:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		doing:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		blocked: lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		inbox:   lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		urgent:  lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		high:    lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
		medium:  lipgloss.NewStyle().Foreground(lipgloss.Color("221")),
		low:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		accent:  lipgloss.NewStyle().Foreground(lipgloss.Color("63")),

		label: lipgloss.NewStyle().Bold(true).Width(10),
		field: lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		hint:  lipgloss.NewStyle().Faint(true),
	}
}
