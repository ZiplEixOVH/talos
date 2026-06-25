package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() string {
	if !m.ready {
		return ""
	}

	var sections []string

	provModel := headerDimStyle.Render("  ") +
		headerValueStyle.Render(m.settings.CurrentProvider) +
		headerDimStyle.Render(" / ") +
		headerValueStyle.Render(m.settings.CurrentModel)

	header := logoStyle.Render("  ◆ Talos") + provModel
	sections = append(sections, header)

	sep := sepStyle.Render(strings.Repeat("─", m.width))
	sections = append(sections, sep)

	sections = append(sections, m.viewport.View())

	if m.state == StateSelectModel || m.state == StateSelectProvider {
		sections = append(sections, m.viewSelect())
	}

	if m.state == StateAskUser {
		sections = append(sections, m.viewAskUser())
	}

	if m.showSuggestions && m.state == StateInput {
		sections = append(sections, m.viewSuggestions())
	}

	if m.state == StateInput {
		sections = append(sections, m.textarea.View())
	} else {
		sections = append(sections, "")
	}

	helpView := m.help.ShortHelpView(m.keys.ShortHelp())
	sections = append(sections, helpView)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) viewSuggestions() string {
	var lines []string
	maxItems := min(len(m.suggestions), 6)
	for i := 0; i < maxItems; i++ {
		cmd := m.suggestions[i]
		desc := suggestionDescStyle.Render("  " + cmd.Desc)
		if i == m.selectedSuggestion {
			lines = append(lines, suggestionActiveStyle.Render("▸ "+cmd.Name)+desc)
		} else {
			lines = append(lines, suggestionStyle.Render("  "+cmd.Name)+desc)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) viewSelect() string {
	var lines []string
	lines = append(lines, "")
	lines = append(lines, selectLabelStyle.Render(m.selectLabel))
	for i, item := range m.selectItems {
		isCurrent := (m.selectKind == "model" && item == m.settings.CurrentModel) ||
			(m.selectKind == "provider" && item == m.settings.CurrentProvider)

		if i == m.selectIndex {
			marker := "▸ "
			label := item
			if isCurrent {
				label += headerDimStyle.Render(" (current)")
			}
			lines = append(lines, selectActiveItemStyle.Render(marker+label))
		} else {
			marker := "  "
			label := item
			if isCurrent {
				label += headerDimStyle.Render(" (current)")
			}
			lines = append(lines, selectItemStyle.Render(marker+label))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func (m *Model) viewAskUser() string {
	var lines []string
	lines = append(lines, "")
	lines = append(lines, selectLabelStyle.Render("  "+m.askQuestion))
	for i, opt := range m.askOptions {
		if i == m.askIndex {
			lines = append(lines, selectActiveItemStyle.Render("▸ "+opt))
		} else {
			lines = append(lines, selectItemStyle.Render("  "+opt))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}