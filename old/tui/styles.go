package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	accentColor = lipgloss.Color("#A78BFA")
	dimColor    = lipgloss.Color("#525264")
	subtleColor = lipgloss.Color("#6B6B80")
	textColor   = lipgloss.Color("#E2E2E9")
	greenColor  = lipgloss.Color("#34D399")
	orangeColor = lipgloss.Color("#FB923C")
	redColor    = lipgloss.Color("#F87171")
	cyanColor   = lipgloss.Color("#67E8F9")

	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	headerDimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	headerValueStyle = lipgloss.NewStyle().
				Foreground(subtleColor)

	userLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(greenColor)

	assistantLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor)

	toolCallStyle = lipgloss.NewStyle().
			Foreground(orangeColor)

	toolCallDimStyle = lipgloss.NewStyle().
				Foreground(dimColor)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(redColor)

	systemMsgStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	thoughtStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(subtleColor)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	suggestionActiveStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true).
				PaddingLeft(2)

	suggestionDescStyle = lipgloss.NewStyle().
				Foreground(dimColor)

	selectItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(4)

	selectActiveItemStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true).
				PaddingLeft(2)

	selectLabelStyle = lipgloss.NewStyle().
				Foreground(subtleColor).
				PaddingLeft(2)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3E3E4E"))

	sepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2A2A3A"))
)