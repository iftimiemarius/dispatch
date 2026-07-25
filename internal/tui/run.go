package tui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iftimiemarius/dispatch/internal/store"
)

// Run launches the interactive TUI over the given store. It takes over the
// terminal (alt-screen) until the user quits. Returns a non-nil error only if
// the program fails to start.
func Run(ctx context.Context, st *store.Store) error {
	// Bubble Tea auto-detects TTY; if stdin isn't a terminal, bail with a hint
	// rather than hanging.
	if !isInteractive() {
		return fmt.Errorf("the TUI needs an interactive terminal; run `dispatch ls` for a one-shot listing")
	}
	p := tea.NewProgram(newApp(ctx, st), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// isInteractive reports whether stdin and stdout are connected to a terminal.
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
