package tui

import (
	"context"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iftimiemarius/dispatch/internal/store"
)

// RenderOnce builds the app model, sends it a WindowSizeMsg so it lays out,
// and writes one rendered frame to w. Intended for headless testing and
// smoke-checks; not used by the live TUI.
func RenderOnce(ctx context.Context, st *store.Store, w io.Writer) error {
	a := newApp(ctx, st)
	// Simulate a typical terminal size so layout code runs.
	win := tea.WindowSizeMsg{Width: 90, Height: 24}
	ma, _ := a.Update(win)
	a = ma.(*app)
	_, _ = io.WriteString(w, a.View())
	_ = time.Now // silence unused import in some build configs
	return nil
}
