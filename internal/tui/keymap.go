package tui

import "charm.land/bubbles/v2/key"

// KeyMap defines all TUI keybindings with self-documenting help text.
type KeyMap struct {
	Quit         key.Binding
	Up           key.Binding
	Down         key.Binding
	Enter        key.Binding
	Esc          key.Binding
	Advance      key.Binding
	OpenPR       key.Binding
	OpenSession  key.Binding
	OpenTerm     key.Binding
	OpenEditor   key.Binding
	AddToday     key.Binding
	RemoveToday  key.Binding
	FilterNext   key.Binding
	FilterPrev   key.Binding
	Search       key.Binding
	Help         key.Binding
	Refresh      key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Top          key.Binding
	Bottom       key.Binding
	Done         key.Binding
	Modify       key.Binding
	Annotate     key.Binding
	ToggleNext   key.Binding
	Copy         key.Binding
	Delete       key.Binding
	Heatmap      key.Binding
	Left         key.Binding
	Right        key.Binding
}

// DefaultKeyMap returns the default keybindings for the TUI.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Up:           key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:         key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Enter:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
		Esc:          key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Advance:      key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "advance")),
		OpenPR:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "open PR")),
		OpenSession:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "attach session")),
		OpenTerm:     key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open terminal")),
		OpenEditor:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit task")),
		AddToday:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add to today")),
		RemoveToday:  key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "remove from today")),
		FilterNext:   key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next filter")),
		FilterPrev:   key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev filter")),
		Search:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Refresh:      key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
		PageUp:       key.NewBinding(key.WithKeys("ctrl+b", "pgup"), key.WithHelp("pgup", "page up")),
		PageDown:     key.NewBinding(key.WithKeys("ctrl+f", "pgdown"), key.WithHelp("pgdn", "page down")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "½ page up")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "½ page down")),
		Top:          key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:       key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Done:         key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
		Modify:       key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "modify")),
		Annotate:     key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "annotate")),
		ToggleNext:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle +next")),
		Copy:         key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
		Delete:       key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
		Heatmap:      key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "heatmap")),
		Left:         key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h", "collapse")),
		Right:        key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "expand")),
	}
}

// ShortHelp returns bindings for the status bar (context-sensitive).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Search, k.FilterNext, k.Quit}
}

// FullHelp returns bindings grouped by category for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom, k.PageUp, k.PageDown, k.HalfPageUp, k.HalfPageDown},
		{k.Enter, k.Advance, k.Done, k.Modify, k.Annotate, k.Delete, k.ToggleNext, k.Copy},
		{k.Right, k.Left, k.OpenPR, k.OpenSession, k.OpenTerm, k.OpenEditor},
		{k.AddToday, k.RemoveToday, k.FilterNext, k.FilterPrev, k.Search, k.Refresh, k.Heatmap, k.Help},
	}
}
