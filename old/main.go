package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/ZiplEix/talos/storage"
	"github.com/ZiplEix/talos/tools"
	"github.com/ZiplEix/talos/tui"
)

func main() {
	var prompt string
	var listConvs bool
	var _convDummy bool

	flag.StringVar(&prompt, "p", "", "Legacy: Prompt to send to LLM in one-shot mode")
	flag.BoolVar(&listConvs, "l", false, "List all conversations")
	flag.BoolVar(&_convDummy, "c", false, "Load conversation (use -c for latest, -c <id> for specific)")
	flag.Parse()

	convID, loadConv := parseConvFlag()

	settings, err := storage.LoadSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
	}

	if listConvs {
		summaries, err := storage.ListConversations()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing conversations: %v\n", err)
			os.Exit(1)
		}
		if len(summaries) == 0 {
			fmt.Println("No conversations found.")
			return
		}
		fmt.Println("Conversations:")
		for _, s := range summaries {
			fmt.Printf("  %s  (%d messages, %s)\n", s.ID, s.Messages, s.CreatedAt)
		}
		return
	}

	activeProv := settings.Providers[settings.CurrentProvider]
	client := openai.NewClient(option.WithAPIKey(activeProv.APIKey), option.WithBaseURL(activeProv.BaseURL))

	if prompt != "" {
		runLegacyOneShot(client, settings, prompt)
		return
	}

	var (
		initialMessages []openai.ChatCompletionMessageParamUnion
		initialConvID   string
	)

	if loadConv {
		if convID != "" {
			initialConvID = convID
			initialMessages, err = storage.LoadConversation(initialConvID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading conversation: %v\n", err)
				os.Exit(1)
			}
		} else {
			initialConvID, initialMessages, err = storage.GetLatestConversation()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading latest conversation: %v\n", err)
				os.Exit(1)
			}
		}
	}

	m := tui.New(client, settings, initialMessages, initialConvID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.Program = p
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseConvFlag() (id string, found bool) {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-p" || arg == "--prompt" {
			i++
			continue
		}
		if arg == "-l" || arg == "--list" {
			continue
		}

		if arg == "-c" || arg == "--conv" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return args[i+1], true
			}
			return "", true
		}
	}
	return "", false
}

func runLegacyOneShot(client openai.Client, settings storage.Settings, prompt string) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	for {
		resp, err := client.Chat.Completions.New(context.Background(),
			openai.ChatCompletionNewParams{
				Model:    openai.ChatModel(settings.CurrentModel),
				Messages: messages,
				Tools:    tools.GetRegisteredTools(),
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error calling api: %v\n", err)
			os.Exit(1)
		}

		choice := resp.Choices[0]
		messages = append(messages, choice.Message.ToParam())

		if len(choice.Message.ToolCalls) == 0 {
			fmt.Print(choice.Message.Content)
		}

		for _, toolCall := range choice.Message.ToolCalls {
			result, toolCallID := tools.HandleToolCall(toolCall)
			messages = append(messages, openai.ToolMessage(result, toolCallID))
		}

		if choice.FinishReason == "stop" {
			break
		}
	}
}