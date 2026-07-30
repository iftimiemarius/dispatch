package tui

import "github.com/charmbracelet/bubbles/key"

// keymap bundles all TUI keybindings. We keep them in one place so the help
// bar and the key handler stay in sync.
type keymap struct {
	Quit      key.Binding
	ForceQuit key.Binding // Ctrl+C
	Help      key.Binding

	NextTab key.Binding
	PrevTab key.Binding

	Up     key.Binding
	Down   key.Binding
	Filter key.Binding
	ClearFilter key.Binding

	New    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Done   key.Binding
	Start  key.Binding
	Move   key.Binding
	Detail key.Binding
	Sync   key.Binding
}

var km = keymap{
	Quit:      key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
	ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),

	NextTab: key.NewBinding(key.WithKeys("tab", "right", "L", "l"), key.WithHelp("→/L", "next tab")),
	PrevTab: key.NewBinding(key.WithKeys("shift+tab", "left", "H", "h"), key.WithHelp("←/H", "prev tab")),

	Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "prev field")),
	Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "next field")),
	Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	ClearFilter: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),

	New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
	Done:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
	Start:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
	Move:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move")),
	Detail: key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "detail")),
	Sync:   key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sync")),
}

// helpKeyMap implements help.KeyMap for the one-line + full help bars.
type helpKeyMap struct{}

func (helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		km.Up, km.Down, km.NextTab, km.PrevTab,
		km.New, km.Edit, km.Done, km.Delete,
		km.Filter, km.Help, km.Quit,
	}
}

// FullHelp returns grouped bindings for the expanded (?) help view.
func (helpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Up, km.Down, km.NextTab, km.PrevTab},       // navigation
		{km.New, km.Edit, km.Done, km.Start, km.Delete, km.Move, km.Detail}, // actions
		{km.Filter, km.ClearFilter, km.Sync},            // view ops
		{km.Help, km.Quit},                              // global
	}
}
