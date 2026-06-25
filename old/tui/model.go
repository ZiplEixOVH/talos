package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/openai/openai-go/v3"

	"github.com/ZiplEix/talos/storage"
	"github.com/ZiplEix/talos/tools"
)

const (
	StateInput = iota
	StateLoading
	StateExecutingTools
	StateSelectModel
	StateSelectProvider
	StateAskUser
)

type ChatMessage struct {
	Role    string
	Content string
}

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

type Model struct {
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model
	help     help.Model
	keys     keyMap

	state  int
	width  int
	height int
	ready  bool

	chatMessages []ChatMessage
	streamBuf    strings.Builder
	oaiMessages  []openai.ChatCompletionMessageParamUnion

	client   openai.Client
	settings storage.Settings
	Program  *tea.Program
	convID   string

	suggestions        []SlashCommand
	selectedSuggestion int
	showSuggestions    bool

	selectItems []string
	selectIndex int
	selectLabel string
	selectKind  string

	askQuestion   string
	askOptions    []string
	askIndex      int
	askResultChan chan string

	toolQueue      []openai.ChatCompletionMessageToolCallUnion
	toolResults    []openai.ChatCompletionMessageParamUnion
	toolQueueIndex int

	mdRenderer *glamour.TermRenderer

	cancelCtx  context.Context
	cancelFunc context.CancelFunc
}

func New(client openai.Client, settings storage.Settings, initialMessages []openai.ChatCompletionMessageParamUnion, initialConvID string) *Model {
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
	if d, err := GetCwd(); err == nil {
		cwd = d
	}

	sysPrompt := fmt.Sprintf(`Tu es Talos, un assistant de code intelligent en ligne de commande.
Le répertoire de travail actuel (CWD) est : %s.
Tu as accès à des outils pour lire, écrire, lister, rechercher des fichiers, et exécuter des commandes via Bash.
Utilise ces outils de manière ciblée, intelligente et sécurisée pour répondre aux demandes de l'utilisateur.`, cwd)

	var oaiMessages []openai.ChatCompletionMessageParamUnion
	var chatMessages []ChatMessage
	convID := fmt.Sprintf("conv_%d", time.Now().UnixNano())

	if len(initialMessages) > 0 && initialConvID != "" {
		oaiMessages = initialMessages
		convID = initialConvID
		chatMessages = RebuildChatDisplay(initialMessages)
	} else {
		oaiMessages = []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(sysPrompt),
		}
	}

	m := &Model{
		textarea:     ta,
		spinner:      s,
		help:         h,
		keys:         newKeyMap(),
		state:        StateInput,
		chatMessages: chatMessages,
		oaiMessages:  oaiMessages,
		client:       client,
		settings:     settings,
		convID:       convID,
		toolQueue:    nil,
		toolResults:  nil,
	}

	m.cancelCtx, m.cancelFunc = context.WithCancel(context.Background())

	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

func RebuildChatDisplay(messages []openai.ChatCompletionMessageParamUnion) []ChatMessage {
	var display []ChatMessage
	for _, msg := range messages {
		jsonData, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		var local storage.LocalMessage
		if err := json.Unmarshal(jsonData, &local); err != nil {
			continue
		}
		switch local.Role {
		case "system":
			display = append(display, ChatMessage{Role: "system", Content: local.Content})
		case "user":
			display = append(display, ChatMessage{Role: "user", Content: local.Content})
		case "assistant":
			if len(local.ToolCalls) > 0 {
				if local.Content != "" {
					display = append(display, ChatMessage{Role: "thought", Content: local.Content})
				}
				for _, tc := range local.ToolCalls {
					paramVal := tools.GetToolParamValue(tc.Function.Name, tc.Function.Arguments)
					display = append(display, ChatMessage{
						Role:    "tool",
						Content: fmt.Sprintf("%s(%s)", tc.Function.Name, paramVal),
					})
				}
			} else if local.Content != "" {
				display = append(display, ChatMessage{Role: "assistant", Content: local.Content})
			}
		case "tool":
		}
	}
	return display
}

func GetCwd() (string, error) {
	return os.Getwd()
}
