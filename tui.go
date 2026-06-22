package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

// ─── States ─────────────────────────────────────────────────────────────────

const (
	stateInput = iota
	stateLoading
	stateExecutingTools
	stateSelectModel
	stateSelectProvider
	stateAskUser
)

// ─── Styles ─────────────────────────────────────────────────────────────────
// Minimal, clean aesthetic inspired by Claude Code / Antigravity CLI

var (
	// Accent colors
	accentColor = lipgloss.Color("#A78BFA") // soft violet
	dimColor    = lipgloss.Color("#525264") // muted gray
	subtleColor = lipgloss.Color("#6B6B80") // slightly lighter gray
	textColor   = lipgloss.Color("#E2E2E9") // light text
	greenColor  = lipgloss.Color("#34D399") // emerald green
	orangeColor = lipgloss.Color("#FB923C") // warm orange
	redColor    = lipgloss.Color("#F87171") // soft red
	cyanColor   = lipgloss.Color("#67E8F9") // light cyan

	// Header
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	headerDimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	headerValueStyle = lipgloss.NewStyle().
				Foreground(subtleColor)

	// Chat
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

	// Suggestions
	suggestionStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	suggestionActiveStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true).
				PaddingLeft(2)

	suggestionDescStyle = lipgloss.NewStyle().
				Foreground(dimColor)

	// Selection list
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

	// Help bar
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3E3E4E"))

	// Separator
	sepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2A2A3A"))
)

// ─── Slash Commands ─────────────────────────────────────────────────────────

type slashCommand struct {
	name string
	desc string
}

var allSlashCommands = []slashCommand{
	{"/help", "Show available commands"},
	{"/exit", "Quit"},
	{"/clear", "New conversation"},
	{"/new", "New conversation"},
	{"/model", "Switch model"},
	{"/provider", "Switch provider"},
	{"/provider set", "Configure a provider"},
	{"/provider add-model", "Add a model to a provider"},
	{"/provider remove-model", "Remove a model from a provider"},
}

// ─── Chat message for display ───────────────────────────────────────────────

type chatMessage struct {
	role    string // "user", "assistant", "tool", "system", "error"
	content string
}

// ─── Custom Msg types ───────────────────────────────────────────────────────

type streamTokenMsg string

type streamDoneMsg struct {
	content   string
	toolCalls []openai.ChatCompletionMessageToolCallUnion
}

type streamErrMsg struct{ err error }

type toolResultsMsg struct {
	results []openai.ChatCompletionMessageParamUnion
}

type askUserRequestMsg struct {
	question string
	options  []string
	resultCh chan string
}

type toolResultMsg struct {
	result     string
	toolCallID string
	toolName   string
}

// ─── Key bindings ───────────────────────────────────────────────────────────

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

// ─── Model ──────────────────────────────────────────────────────────────────

type model struct {
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model
	help     help.Model
	keys     keyMap

	state  int
	width  int
	height int
	ready  bool

	chatMessages []chatMessage
	streamBuf    strings.Builder
	oaiMessages  []openai.ChatCompletionMessageParamUnion

	client   openai.Client
	settings Settings
	program  *tea.Program
	convID   string

	// Auto-completion
	suggestions        []slashCommand
	selectedSuggestion int
	showSuggestions    bool

	// Model/Provider selection
	selectItems []string
	selectIndex int
	selectLabel string
	selectKind  string // "model" or "provider"

	// AskUser
	askQuestion   string
	askOptions    []string
	askIndex      int
	askResultChan chan string

	// Tool execution queue — process tools one at a time in the event loop
	toolQueue      []openai.ChatCompletionMessageToolCallUnion
	toolResults    []openai.ChatCompletionMessageParamUnion
	toolQueueIndex int

	// Glamour renderer
	mdRenderer *glamour.TermRenderer
}

func newModel(client openai.Client, settings Settings) *model {
	ta := textarea.New()
	ta.Placeholder = "Message Talos..."
	ta.Focus()
	ta.Prompt = "❯ "
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(dimColor)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(textColor)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(dimColor)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(dimColor)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(dimColor)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	h := help.New()
	h.Styles.ShortKey = helpKeyStyle
	h.Styles.ShortDesc = helpDescStyle
	h.Styles.ShortSeparator = helpDescStyle

	cwd := "."
	if d, err := getCwd(); err == nil {
		cwd = d
	}

	sysPrompt := fmt.Sprintf(`Tu es Talos, un assistant de code intelligent en ligne de commande.
Le répertoire de travail actuel (CWD) est : %s.
Tu as accès à des outils pour lire, écrire, lister, rechercher des fichiers, et exécuter des commandes via Bash.
Utilise ces outils de manière ciblée, intelligente et sécurisée pour répondre aux demandes de l'utilisateur.`, cwd)

	oaiMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(sysPrompt),
	}

	m := &model{
		textarea:     ta,
		spinner:      s,
		help:         h,
		keys:         newKeyMap(),
		state:        stateInput,
		chatMessages: []chatMessage{},
		oaiMessages:  oaiMessages,
		client:       client,
		settings:     settings,
		convID:       fmt.Sprintf("conv_%d", time.Now().UnixNano()),
		toolQueue:    nil,
		toolResults:  nil,
	}

	AskUserHandler = func(question string, options []string) string {
		ch := make(chan string)
		if m.program != nil {
			m.program.Send(askUserRequestMsg{
				question: question,
				options:  options,
				resultCh: ch,
			})
		}
		return <-ch
	}

	return m
}

func getCwd() (string, error) {
	return os.Getwd()
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

// ─── Update ─────────────────────────────────────────────────────────────────

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.state = stateInput
		m.chatMessages = append(m.chatMessages, chatMessage{
			role:    "error",
			content: fmt.Sprintf("API error: %v", msg.err),
		})
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil

	case toolResultsMsg:
		m.oaiMessages = append(m.oaiMessages, msg.results...)
		_ = saveConversation(m.convID, m.oaiMessages)
		return m, m.startStreaming()

	case askUserRequestMsg:
		m.state = stateAskUser
		m.askQuestion = msg.question
		m.askOptions = msg.options
		m.askIndex = 0
		m.askResultChan = msg.resultCh
		return m, nil

	case toolResultMsg:
		// Append this tool result and process the next tool in queue
		m.toolResults = append(m.toolResults, openai.ToolMessage(msg.result, msg.toolCallID))
		m.toolQueueIndex++
		if m.toolQueueIndex < len(m.toolQueue) {
			// More tools to execute
			return m, m.executeNextToolCmd()
		}
		// All tools executed — send results back to the model
		m.state = stateLoading
		m.oaiMessages = append(m.oaiMessages, m.toolResults...)
		m.toolQueue = nil
		m.toolResults = nil
		m.toolQueueIndex = 0
		_ = saveConversation(m.convID, m.oaiMessages)
		return m, m.startStreaming()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if (m.state == stateLoading && m.streamBuf.Len() == 0) || m.state == stateExecutingTools {
			m.updateViewport()
		}
		return m, cmd

	case tea.KeyMsg:
		switch m.state {
		case stateInput:
			return m.updateInput(msg)
		case stateLoading, stateExecutingTools:
			return m.updateLoading(msg)
		case stateSelectModel, stateSelectProvider:
			return m.updateSelect(msg)
		case stateAskUser:
			return m.updateAskUser(msg)
		}
	}

	// Always update the spinner model, regardless of state,
	// so the spinner.Tick chain stays alive.
	// The spinner only advances on spinner.TickMsg, so this is harmless
	// when the spinner is not visible.
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	cmds = append(cmds, spinnerCmd)

	// Update textarea only in input state
	if m.state == stateInput {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// ─── Input state ────────────────────────────────────────────────────────────

func (m *model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.textarea.SetValue(selected.name + " ")
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
			m.textarea.SetValue(selected.name + " ")
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

	// Update textarea and check for suggestions
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

func filterCommands(input string) []slashCommand {
	input = strings.TrimSpace(input)
	if input == "/" {
		return allSlashCommands
	}
	var filtered []slashCommand
	for _, cmd := range allSlashCommands {
		if strings.HasPrefix(cmd.name, input) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

// ─── Loading state ──────────────────────────────────────────────────────────

func (m *model) updateLoading(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	return m, nil
}

// ─── Select state (model/provider) ──────────────────────────────────────────

func (m *model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Escape):
		m.state = stateInput
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
			m.state = stateInput
			return m, nil
		}
		selected := m.selectItems[m.selectIndex]
		return m.handleSelection(selected)
	}
	return m, nil
}

func (m *model) handleSelection(selected string) (tea.Model, tea.Cmd) {
	if m.selectKind == "model" {
		m.settings.CurrentModel = selected
		_ = saveSettings(m.settings)
		m.chatMessages = append(m.chatMessages, chatMessage{
			role:    "system",
			content: fmt.Sprintf("Model → %s", selected),
		})
	} else if m.selectKind == "provider" {
		prov, exists := m.settings.Providers[selected]
		if !exists {
			m.state = stateInput
			return m, nil
		}
		m.settings.CurrentProvider = selected
		if len(prov.Models) > 0 {
			m.settings.CurrentModel = prov.Models[0]
		}
		_ = saveSettings(m.settings)
		m.client = openai.NewClient(option.WithAPIKey(prov.APIKey), option.WithBaseURL(prov.BaseURL))
		m.chatMessages = append(m.chatMessages, chatMessage{
			role:    "system",
			content: fmt.Sprintf("Provider → %s (model: %s)", selected, m.settings.CurrentModel),
		})
	}
	m.state = stateInput
	m.updateViewport()
	m.viewport.GotoBottom()
	return m, nil
}

// ─── AskUser state ──────────────────────────────────────────────────────────

func (m *model) updateAskUser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.state = stateLoading
			m.chatMessages = append(m.chatMessages, chatMessage{
				role:    "system",
				content: fmt.Sprintf("→ %s", selected),
			})
			m.updateViewport()
			m.viewport.GotoBottom()
		}
		return m, nil
	}
	return m, nil
}

// ─── Slash commands ─────────────────────────────────────────────────────────

func (m *model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/exit":
		return m, tea.Quit

	case "/help":
		helpText := headerDimStyle.Render("─── Commands ") + headerDimStyle.Render(strings.Repeat("─", max(0, m.width-15))) + "\n"
		helpText += fmt.Sprintf("  %s  %s\n", accentStyle("/exit"), "Quit")
		helpText += fmt.Sprintf("  %s  %s\n", accentStyle("/clear"), "New conversation")
		helpText += fmt.Sprintf("  %s  %s\n", accentStyle("/model"), "Select or switch model")
		helpText += fmt.Sprintf("  %s  %s\n", accentStyle("/model <n>"), "Switch to model directly")
		helpText += fmt.Sprintf("  %s  %s\n", accentStyle("/provider"), "Select or switch provider")
		helpText += fmt.Sprintf("  %s  %s\n", accentStyle("/provider set <n> <k> <u>"), "Configure provider")
		helpText += fmt.Sprintf("  %s  %s\n", accentStyle("/provider add-model <n> <m>"), "Add model to provider")
		helpText += fmt.Sprintf("  %s  %s", accentStyle("/provider remove-model <n> <m>"), "Remove model from provider")
		m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: helpText})

	case "/clear", "/new":
		cwd := "."
		if d, err := getCwd(); err == nil {
			cwd = d
		}
		sysPrompt := fmt.Sprintf(`Tu es Talos, un assistant de code intelligent en ligne de commande.
Le répertoire de travail actuel (CWD) est : %s.
Tu as accès à des outils pour lire, écrire, lister, rechercher des fichiers, et exécuter des commandes via Bash.
Utilise ces outils de manière ciblée, intelligente et sécurisée pour répondre aux demandes de l'utilisateur.`, cwd)
		m.oaiMessages = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(sysPrompt)}
		m.chatMessages = []chatMessage{}
		m.convID = fmt.Sprintf("conv_%d", time.Now().UnixNano())
		m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: "Conversation cleared."})

	case "/model":
		if len(parts) < 2 {
			activeProv := m.settings.Providers[m.settings.CurrentProvider]
			if len(activeProv.Models) == 0 {
				m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: "No models configured. Use /provider add-model to add one."})
			} else {
				m.state = stateSelectModel
				m.selectItems = activeProv.Models
				m.selectLabel = "Select model"
				m.selectKind = "model"
				m.selectIndex = 0
				for i, mdl := range activeProv.Models {
					if mdl == m.settings.CurrentModel {
						m.selectIndex = i
						break
					}
				}
			}
		} else {
			newModelName := parts[1]
			m.settings.CurrentModel = newModelName
			activeProv := m.settings.Providers[m.settings.CurrentProvider]
			found := false
			for _, mdl := range activeProv.Models {
				if mdl == newModelName {
					found = true
					break
				}
			}
			if !found {
				activeProv.Models = append(activeProv.Models, newModelName)
				m.settings.Providers[m.settings.CurrentProvider] = activeProv
			}
			_ = saveSettings(m.settings)
			m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Model → %s", newModelName)})
		}

	case "/provider":
		if len(parts) < 2 {
			// Open interactive provider selection menu (same as /model)
			var provNames []string
			for name := range m.settings.Providers {
				provNames = append(provNames, name)
			}
			if len(provNames) == 0 {
				m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: "No providers configured."})
			} else {
				m.state = stateSelectProvider
				m.selectItems = provNames
				m.selectLabel = "Select provider"
				m.selectKind = "provider"
				m.selectIndex = 0
				for i, n := range provNames {
					if n == m.settings.CurrentProvider {
						m.selectIndex = i
						break
					}
				}
			}
		} else {
			subCmd := parts[1]
			switch subCmd {
			case "use":
				if len(parts) < 3 {
					var provNames []string
					for name := range m.settings.Providers {
						provNames = append(provNames, name)
					}
					if len(provNames) == 0 {
						m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: "No providers configured."})
					} else {
						m.state = stateSelectProvider
						m.selectItems = provNames
						m.selectLabel = "Select provider"
						m.selectKind = "provider"
						m.selectIndex = 0
						for i, n := range provNames {
							if n == m.settings.CurrentProvider {
								m.selectIndex = i
								break
							}
						}
					}
				} else {
					newName := parts[2]
					prov, exists := m.settings.Providers[newName]
					if !exists {
						m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Provider '%s' not found.", newName)})
					} else {
						m.settings.CurrentProvider = newName
						if len(prov.Models) > 0 {
							m.settings.CurrentModel = prov.Models[0]
						}
						_ = saveSettings(m.settings)
						m.client = openai.NewClient(option.WithAPIKey(prov.APIKey), option.WithBaseURL(prov.BaseURL))
						m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Provider → %s (model: %s)", newName, m.settings.CurrentModel)})
					}
				}
			case "set":
				if len(parts) < 5 {
					m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: "Usage: /provider set <name> <api_key> <base_url>"})
				} else {
					name, apiKey, baseURL := parts[2], parts[3], parts[4]
					prov, exists := m.settings.Providers[name]
					if exists {
						prov.APIKey = apiKey
						prov.BaseURL = baseURL
					} else {
						prov = Provider{Name: name, APIKey: apiKey, BaseURL: baseURL, Models: []string{}}
					}
					m.settings.Providers[name] = prov
					_ = saveSettings(m.settings)
					if name == m.settings.CurrentProvider {
						m.client = openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))
					}
					m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Provider '%s' configured.", name)})
				}
			case "add-model":
				if len(parts) < 4 {
					m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: "Usage: /provider add-model <name> <model>"})
				} else {
					name, mdl := parts[2], parts[3]
					prov, exists := m.settings.Providers[name]
					if !exists {
						m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Provider '%s' not found.", name)})
					} else {
						found := false
						for _, m := range prov.Models {
							if m == mdl {
								found = true
								break
							}
						}
						if !found {
							prov.Models = append(prov.Models, mdl)
							m.settings.Providers[name] = prov
							_ = saveSettings(m.settings)
							m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Model '%s' added to '%s'.", mdl, name)})
						} else {
							m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Model '%s' already in '%s'.", mdl, name)})
						}
					}
				}
			case "remove-model":
				if len(parts) < 4 {
					m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: "Usage: /provider remove-model <name> <model>"})
				} else {
					name, mdl := parts[2], parts[3]
					prov, exists := m.settings.Providers[name]
					if !exists {
						m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Provider '%s' not found.", name)})
					} else {
						idx := -1
						for i, m := range prov.Models {
							if m == mdl {
								idx = i
								break
							}
						}
						if idx != -1 {
							prov.Models = append(prov.Models[:idx], prov.Models[idx+1:]...)
							m.settings.Providers[name] = prov
							_ = saveSettings(m.settings)
							m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Model '%s' removed from '%s'.", mdl, name)})
						} else {
							m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Model '%s' not found in '%s'.", mdl, name)})
						}
					}
				}
			default:
				m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Unknown subcommand: %s", subCmd)})
			}
		}

	default:
		m.chatMessages = append(m.chatMessages, chatMessage{role: "system", content: fmt.Sprintf("Unknown command: %s. Type /help.", cmd)})
	}

	m.updateViewport()
	m.viewport.GotoBottom()
	return m, nil
}

func accentStyle(s string) string {
	return lipgloss.NewStyle().Foreground(accentColor).Render(s)
}

// ─── Streaming ──────────────────────────────────────────────────────────────

func (m *model) sendUserMessage(input string) tea.Cmd {
	m.chatMessages = append(m.chatMessages, chatMessage{role: "user", content: input})
	m.oaiMessages = append(m.oaiMessages, openai.UserMessage(input))
	m.state = stateLoading
	m.streamBuf.Reset()
	m.updateViewport()
	m.viewport.GotoBottom()
	return m.startStreaming()
}

func (m *model) startStreaming() tea.Cmd {
	client := m.client
	messages := make([]openai.ChatCompletionMessageParamUnion, len(m.oaiMessages))
	copy(messages, m.oaiMessages)
	modelName := m.settings.CurrentModel
	p := m.program

	return func() tea.Msg {
		ctx := context.Background()
		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(modelName),
			Messages: messages,
			Tools:    getRegisteredTools(),
		}

		stream := client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}
		var contentBuf strings.Builder

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Content != "" {
					contentBuf.WriteString(delta.Content)
					if p != nil {
						p.Send(streamTokenMsg(delta.Content))
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			return streamErrMsg{err: err}
		}

		var toolCalls []openai.ChatCompletionMessageToolCallUnion
		if len(acc.Choices) > 0 {
			toolCalls = acc.Choices[0].Message.ToolCalls
		}

		return streamDoneMsg{
			content:   contentBuf.String(),
			toolCalls: toolCalls,
		}
	}
}

func (m *model) handleStreamDone(msg streamDoneMsg) tea.Cmd {
	if len(msg.toolCalls) > 0 {
		var tcs []openai.ChatCompletionMessageToolCallUnionParam
		for _, tc := range msg.toolCalls {
			tcs = append(tcs, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID:   tc.ID,
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				},
			})
		}
		assistantParam := openai.ChatCompletionAssistantMessageParam{
			ToolCalls: tcs,
		}
		if msg.content != "" {
			assistantParam.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: param.NewOpt(msg.content),
			}
		}
		m.oaiMessages = append(m.oaiMessages, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &assistantParam,
		})

		if msg.content != "" {
			m.chatMessages = append(m.chatMessages, chatMessage{
				role:    "thought",
				content: msg.content,
			})
		}

		for _, tc := range msg.toolCalls {
			paramVal := getToolParamValue(tc.Function.Name, tc.Function.Arguments)
			m.chatMessages = append(m.chatMessages, chatMessage{
				role:    "tool",
				content: fmt.Sprintf("%s(%s)", tc.Function.Name, paramVal),
			})
		}

		_ = saveConversation(m.convID, m.oaiMessages)

		// Set up the tool queue and start executing one by one
		m.toolQueue = msg.toolCalls
		m.toolResults = nil
		m.toolQueueIndex = 0
		m.state = stateExecutingTools
		m.streamBuf.Reset()
		m.updateViewport()
		m.viewport.GotoBottom()
		return m.executeNextToolCmd()
	}

	m.state = stateInput
	content := msg.content
	if content == "" {
		content = m.streamBuf.String()
	}

	if content != "" {
		m.chatMessages = append(m.chatMessages, chatMessage{role: "assistant", content: content})
		m.oaiMessages = append(m.oaiMessages, openai.AssistantMessage(content))
	}

	m.streamBuf.Reset()
	_ = saveConversation(m.convID, m.oaiMessages)
	m.updateViewport()
	m.viewport.GotoBottom()
	return nil
}

// executeNextToolCmd runs one tool at a time via the event loop,
// so the spinner/animation stays alive between tool calls.
func (m *model) executeNextToolCmd() tea.Cmd {
	if m.toolQueueIndex >= len(m.toolQueue) {
		return nil
	}
	tc := m.toolQueue[m.toolQueueIndex]
	return func() tea.Msg {
		// Execute synchronously (blocking is fine here since this runs in a goroutine)
		result, toolCallID := handleToolCall(tc)
		return toolResultMsg{
			result:     result,
			toolCallID: toolCallID,
			toolName:   tc.Function.Name,
		}
	}
}

// ─── Layout ─────────────────────────────────────────────────────────────────

func (m *model) handleResize() tea.Cmd {
	headerH := 2 // header + separator
	inputH := 1  // single-line textarea
	helpH := 1
	padding := 1 // spacing
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

func (m *model) updateViewport() {
	var sb strings.Builder

	for _, msg := range m.chatMessages {
		switch msg.role {
		case "user":
			sb.WriteString("\n")
			sb.WriteString(userLabelStyle.Render("  ❯ "))
			sb.WriteString(lipgloss.NewStyle().Foreground(textColor).Render(msg.content))
			sb.WriteString("\n")
		case "assistant":
			sb.WriteString("\n")
			rendered := msg.content
			if m.mdRenderer != nil {
				if md, err := m.mdRenderer.Render(msg.content); err == nil {
					rendered = strings.TrimRight(md, "\n")
				}
			}
			sb.WriteString(rendered)
			sb.WriteString("\n")
		case "thought":
			sb.WriteString("\n")
			rendered := msg.content
			if m.mdRenderer != nil {
				if md, err := m.mdRenderer.Render(msg.content); err == nil {
					rendered = strings.TrimRight(md, "\n")
				}
			}
			sb.WriteString(thoughtStyle.Render(rendered))
			sb.WriteString("\n")
		case "tool":
			// Compact tool display: ToolName(param) in dim style
			parts := strings.SplitN(msg.content, "(", 2)
			if len(parts) == 2 {
				sb.WriteString("  ")
				sb.WriteString(toolCallStyle.Render(parts[0]))
				sb.WriteString(toolCallDimStyle.Render("(" + parts[1]))
			} else {
				sb.WriteString("  ")
				sb.WriteString(toolCallStyle.Render(msg.content))
			}
			sb.WriteString("\n")
		case "system":
			sb.WriteString("\n")
			sb.WriteString(msg.content)
			sb.WriteString("\n")
		case "error":
			sb.WriteString("\n")
			sb.WriteString(errorMsgStyle.Render("  ✗ " + msg.content))
			sb.WriteString("\n")
		}
	}

	// Show streaming content
	if m.state == stateLoading {
		sb.WriteString("\n")
		if m.streamBuf.Len() > 0 {
			sb.WriteString(wrapText(m.streamBuf.String(), m.width-4))
		} else {
			sb.WriteString("  ")
			sb.WriteString(m.spinner.View())
			sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" Thinking..."))
		}
	}

	if m.state == stateExecutingTools {
		sb.WriteString("\n")
		sb.WriteString("  ")
		sb.WriteString(m.spinner.View())
		sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" Executing tools..."))
	}

	m.viewport.SetContent(sb.String())
}

// ─── View ───────────────────────────────────────────────────────────────────

func (m *model) View() string {
	if !m.ready {
		return ""
	}

	var sections []string

	// Header - clean, minimal
	provModel := headerDimStyle.Render("  ") +
		headerValueStyle.Render(m.settings.CurrentProvider) +
		headerDimStyle.Render(" / ") +
		headerValueStyle.Render(m.settings.CurrentModel)

	header := logoStyle.Render("  ◆ Talos") + provModel
	sections = append(sections, header)

	// Thin separator
	sep := sepStyle.Render(strings.Repeat("─", m.width))
	sections = append(sections, sep)

	// Viewport (chat history)
	sections = append(sections, m.viewport.View())

	// Select overlay (model/provider)
	if m.state == stateSelectModel || m.state == stateSelectProvider {
		sections = append(sections, m.viewSelect())
	}

	// AskUser overlay
	if m.state == stateAskUser {
		sections = append(sections, m.viewAskUser())
	}

	// Suggestion overlay
	if m.showSuggestions && m.state == stateInput {
		sections = append(sections, m.viewSuggestions())
	}

	// Input area
	if m.state == stateInput {
		sections = append(sections, m.textarea.View())
	} else {
		sections = append(sections, "")
	}

	// Help bar
	helpView := m.help.ShortHelpView(m.keys.ShortHelp())
	sections = append(sections, helpView)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *model) viewSuggestions() string {
	var lines []string
	maxItems := min(len(m.suggestions), 6)
	for i := 0; i < maxItems; i++ {
		cmd := m.suggestions[i]
		desc := suggestionDescStyle.Render("  " + cmd.desc)
		if i == m.selectedSuggestion {
			lines = append(lines, suggestionActiveStyle.Render("▸ "+cmd.name)+desc)
		} else {
			lines = append(lines, suggestionStyle.Render("  "+cmd.name)+desc)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *model) viewSelect() string {
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

func (m *model) viewAskUser() string {
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

// ─── Helpers ────────────────────────────────────────────────────────────────

func getToolParamValue(name string, argumentsJSON string) string {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return ""
	}

	var primaryVal interface{}
	switch name {
	case "Read", "Write", "ReadRange", "ReplaceInFile":
		primaryVal = args["file_path"]
	case "Bash":
		primaryVal = args["command"]
	case "List":
		primaryVal = args["directory"]
	case "FetchWebPage":
		primaryVal = args["url"]
	case "GoogleSearch":
		primaryVal = args["query"]
	case "FileSearch":
		primaryVal = args["pattern"]
	case "AskUser":
		primaryVal = args["question"]
	}

	if primaryVal != nil {
		if str, ok := primaryVal.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", primaryVal)
	}

	if len(args) > 0 {
		for _, v := range args {
			return fmt.Sprintf("%v", v)
		}
	}

	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func wrapText(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = wrapLine(line, limit)
	}
	return strings.Join(lines, "\n")
}

func wrapLine(line string, limit int) string {
	if len(line) <= limit {
		return line
	}

	var sb strings.Builder
	runes := []rune(line)
	start := 0

	for start < len(runes) {
		if len(runes)-start <= limit {
			sb.WriteString(string(runes[start:]))
			break
		}

		end := start + limit
		spaceIdx := -1
		for i := end; i > start; i-- {
			if runes[i] == ' ' || runes[i] == '\t' {
				spaceIdx = i
				break
			}
		}

		if spaceIdx != -1 {
			sb.WriteString(string(runes[start:spaceIdx]))
			sb.WriteString("\n")
			start = spaceIdx + 1
		} else {
			sb.WriteString(string(runes[start:end]))
			sb.WriteString("\n")
			start = end
		}
	}
	return sb.String()
}
