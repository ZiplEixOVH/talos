package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Send    key.Binding
	Quit    key.Binding
	NewLine key.Binding
	Up      key.Binding
	Down    key.Binding
	Tab     key.Binding
	Escape  key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		NewLine: key.NewBinding(
			key.WithKeys("shift+enter"),
			key.WithHelp("shift+enter", "newline"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "complete"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.NewLine, k.Tab, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Send, k.NewLine, k.Tab, k.Quit}}
}