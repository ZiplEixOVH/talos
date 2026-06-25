package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/ZiplEix/talos/storage"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, m.handleResize()

	case streamTokenMsg:
		m.streamBuf.WriteString(string(msg))
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil

	case streamDoneMsg:
		return m, m.handleStreamDone(msg)

	case streamErrMsg:
		if m.state != StateLoading && m.state != StateExecutingTools {
			return m, nil
		}
		m.state = StateInput
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:    "error",
			Content: fmt.Sprintf("API error: %v", msg.err),
		})
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil

	case toolResultsMsg:
		m.oaiMessages = append(m.oaiMessages, msg.results...)
		_ = storage.SaveConversation(m.convID, m.oaiMessages)
		return m, m.startStreaming()

	case askUserRequestMsg:
		m.state = StateAskUser
		m.askQuestion = msg.question
		m.askOptions = msg.options
		m.askIndex = 0
		m.askResultChan = msg.resultCh
		return m, nil

	case toolResultMsg:
		if m.state != StateExecutingTools {
			return m, nil
		}
		m.toolResults = append(m.toolResults, openai.ToolMessage(msg.result, msg.toolCallID))
		m.toolQueueIndex++
		if m.toolQueueIndex < len(m.toolQueue) {
			return m, m.executeNextToolCmd()
		}
		m.state = StateLoading
		m.oaiMessages = append(m.oaiMessages, m.toolResults...)
		m.toolQueue = nil
		m.toolResults = nil
		m.toolQueueIndex = 0
		_ = storage.SaveConversation(m.convID, m.oaiMessages)
		return m, m.startStreaming()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if (m.state == StateLoading && m.streamBuf.Len() == 0) || m.state == StateExecutingTools {
			m.updateViewport()
		}
		return m, cmd

	case tea.KeyMsg:
		switch m.state {
		case StateInput:
			return m.updateInput(msg)
		case StateLoading, StateExecutingTools:
			return m.updateLoading(msg)
		case StateSelectModel, StateSelectProvider:
			return m.updateSelect(msg)
		case StateAskUser:
			return m.updateAskUser(msg)
		}
	}

	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	cmds = append(cmds, spinnerCmd)

	if m.state == StateInput {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Escape):
		if m.showSuggestions {
			m.showSuggestions = false
			return m, nil
		}
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		if m.showSuggestions && len(m.suggestions) > 0 {
			selected := m.suggestions[m.selectedSuggestion]
			m.textarea.SetValue(selected.Name + " ")
			m.showSuggestions = false
			m.selectedSuggestion = 0
			return m, nil
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.showSuggestions && len(m.suggestions) > 0 {
			m.selectedSuggestion--
			if m.selectedSuggestion < 0 {
				m.selectedSuggestion = len(m.suggestions) - 1
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case key.Matches(msg, m.keys.Down):
		if m.showSuggestions && len(m.suggestions) > 0 {
			m.selectedSuggestion++
			if m.selectedSuggestion >= len(m.suggestions) {
				m.selectedSuggestion = 0
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case key.Matches(msg, m.keys.Send):
		if m.showSuggestions && len(m.suggestions) > 0 {
			selected := m.suggestions[m.selectedSuggestion]
			m.textarea.SetValue(selected.Name + " ")
			m.showSuggestions = false
			m.selectedSuggestion = 0
			return m, nil
		}

		input := strings.TrimSpace(m.textarea.Value())
		if input == "" {
			return m, nil
		}
		m.textarea.Reset()
		m.showSuggestions = false

		if strings.HasPrefix(input, "/") {
			return m.handleSlashCommand(input)
		}

		return m, m.sendUserMessage(input)
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)

	val := m.textarea.Value()
	if strings.HasPrefix(val, "/") {
		m.suggestions = filterCommands(val)
		m.showSuggestions = len(m.suggestions) > 0
		if m.selectedSuggestion >= len(m.suggestions) {
			m.selectedSuggestion = 0
		}
	} else {
		m.showSuggestions = false
	}

	return m, cmd
}

func (m *Model) updateLoading(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		m.cancelFunc()
		m.state = StateInput
		m.streamBuf.Reset()
		m.toolQueue = nil
		m.toolResults = nil
		m.toolQueueIndex = 0
		m.cancelCtx, m.cancelFunc = context.WithCancel(context.Background())
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:    "system",
			Content: "Cancelled.",
		})
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil
	}
	return m, nil
}

func (m *Model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Escape):
		m.state = StateInput
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.selectIndex--
		if m.selectIndex < 0 {
			m.selectIndex = len(m.selectItems) - 1
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.selectIndex++
		if m.selectIndex >= len(m.selectItems) {
			m.selectIndex = 0
		}
		return m, nil
	case key.Matches(msg, m.keys.Send):
		if len(m.selectItems) == 0 {
			m.state = StateInput
			return m, nil
		}
		selected := m.selectItems[m.selectIndex]
		return m.handleSelection(selected)
	}
	return m, nil
}

func (m *Model) handleSelection(selected string) (tea.Model, tea.Cmd) {
	if m.selectKind == "model" {
		m.settings.CurrentModel = selected
		_ = storage.SaveSettings(m.settings)
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("Model → %s", selected),
		})
	} else if m.selectKind == "provider" {
		prov, exists := m.settings.Providers[selected]
		if !exists {
			m.state = StateInput
			return m, nil
		}
		m.settings.CurrentProvider = selected
		if len(prov.Models) > 0 {
			m.settings.CurrentModel = prov.Models[0]
		}
		_ = storage.SaveSettings(m.settings)
		m.client = openai.NewClient(option.WithAPIKey(prov.APIKey), option.WithBaseURL(prov.BaseURL))
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("Provider → %s (model: %s)", selected, m.settings.CurrentModel),
		})
	}
	m.state = StateInput
	m.updateViewport()
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) updateAskUser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.askIndex--
		if m.askIndex < 0 {
			m.askIndex = len(m.askOptions) - 1
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.askIndex++
		if m.askIndex >= len(m.askOptions) {
			m.askIndex = 0
		}
		return m, nil
	case key.Matches(msg, m.keys.Send):
		if len(m.askOptions) > 0 && m.askResultChan != nil {
			selected := m.askOptions[m.askIndex]
			m.askResultChan <- selected
			m.askResultChan = nil
			m.state = StateLoading
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:    "system",
				Content: fmt.Sprintf("→ %s", selected),
			})
			m.updateViewport()
			m.viewport.GotoBottom()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) handleResize() tea.Cmd {
	headerH := 2
	inputH := 1
	helpH := 1
	padding := 1
	vpH := m.height - headerH - inputH - helpH - padding

	if m.showSuggestions {
		suggestH := min(len(m.suggestions), 6)
		vpH -= suggestH
	}

	if vpH < 1 {
		vpH = 1
	}

	if !m.ready {
		m.viewport = viewport.New(m.width, vpH)
		m.ready = true
	} else {
		m.viewport.Width = m.width
		m.viewport.Height = vpH
	}

	m.textarea.SetWidth(m.width)

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(m.width-4),
	)
	if err == nil {
		m.mdRenderer = renderer
	}

	m.updateViewport()
	return nil
}

func (m *Model) updateViewport() {
	var sb strings.Builder

	for _, msg := range m.chatMessages {
		switch msg.Role {
		case "user":
			sb.WriteString("\n")
			sb.WriteString(userLabelStyle.Render("  ❯ "))
			sb.WriteString(lipgloss.NewStyle().Foreground(textColor).Render(msg.Content))
			sb.WriteString("\n")
		case "assistant":
			sb.WriteString("\n")
			rendered := msg.Content
			if m.mdRenderer != nil {
				if md, err := m.mdRenderer.Render(msg.Content); err == nil {
					rendered = strings.TrimRight(md, "\n")
				}
			}
			sb.WriteString(rendered)
			sb.WriteString("\n")
		case "thought":
			sb.WriteString("\n")
			rendered := msg.Content
			if m.mdRenderer != nil {
				if md, err := m.mdRenderer.Render(msg.Content); err == nil {
					rendered = strings.TrimRight(md, "\n")
				}
			}
			sb.WriteString(thoughtStyle.Render(rendered))
			sb.WriteString("\n")
		case "tool":
			parts := strings.SplitN(msg.Content, "(", 2)
			if len(parts) == 2 {
				sb.WriteString("  ")
				sb.WriteString(toolCallStyle.Render(parts[0]))
				sb.WriteString(toolCallDimStyle.Render("(" + parts[1]))
			} else {
				sb.WriteString("  ")
				sb.WriteString(toolCallStyle.Render(msg.Content))
			}
			sb.WriteString("\n")
		case "system":
			sb.WriteString("\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n")
		case "error":
			sb.WriteString("\n")
			sb.WriteString(errorMsgStyle.Render("  ✗ " + msg.Content))
			sb.WriteString("\n")
		}
	}

	if m.state == StateLoading {
		sb.WriteString("\n")
		if m.streamBuf.Len() > 0 {
			sb.WriteString(wrapText(m.streamBuf.String(), m.width-4))
		} else {
			sb.WriteString("  ")
			sb.WriteString(m.spinner.View())
			sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" Thinking..."))
		}
	}

	if m.state == StateExecutingTools {
		sb.WriteString("\n")
		sb.WriteString("  ")
		sb.WriteString(m.spinner.View())
		sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" Executing tools..."))
	}

	m.viewport.SetContent(sb.String())
}