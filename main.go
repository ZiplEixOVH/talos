package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Legacy: Prompt to send to LLM in one-shot mode")
	flag.Parse()

	// Load settings from .talos
	settings, err := loadSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
	}

	activeProv := settings.Providers[settings.CurrentProvider]
	client := openai.NewClient(option.WithAPIKey(activeProv.APIKey), option.WithBaseURL(activeProv.BaseURL))

	if prompt != "" {
		runLegacyOneShot(client, settings, prompt)
		return
	}

	m := newModel(client, settings)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program = p
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runLegacyOneShot(client openai.Client, settings Settings, prompt string) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	for {
		resp, err := client.Chat.Completions.New(context.Background(),
			openai.ChatCompletionNewParams{
				Model:    openai.ChatModel(settings.CurrentModel),
				Messages: messages,
				Tools:    getRegisteredTools(),
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
			result, toolCallID := handleToolCall(toolCall)
			messages = append(messages, openai.ToolMessage(result, toolCallID))
		}

		if choice.FinishReason == "stop" {
			break
		}
	}
}

func getRegisteredTools() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "Read",
			Description: openai.String("Read and return the content of a file"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"file_path"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "Write",
			Description: openai.String("Write content to a file, create the file if it does not exist"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to write",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The content to write to the file",
					},
				},
				"required": []string{"file_path", "content"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "Bash",
			Description: openai.String("Execute a shell command"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command to execute",
					},
				},
				"required": []string{"command"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "List",
			Description: openai.String("List files in a directory"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"directory": map[string]any{
						"type":        "string",
						"description": "The directory path to list files from",
					},
				},
				"required": []string{"directory"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "FetchWebPage",
			Description: openai.String("Fetch the content of a webpage"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The URL of the webpage to fetch",
					},
				},
				"required": []string{"url"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "GoogleSearch",
			Description: openai.String("Search Google for a given query and return a list of search results (titles, URLs, snippets)"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query",
					},
				},
				"required": []string{"query"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "FileSearch",
			Description: openai.String("Search for a pattern or keyword recursively within a directory or file"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "The pattern or keyword to search for",
					},
					"directory": map[string]any{
						"type":        "string",
						"description": "The directory or file path to search inside",
					},
				},
				"required": []string{"pattern", "directory"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "ReadRange",
			Description: openai.String("Read a specific line range from a file, avoiding loading the entire file"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to read",
					},
					"start_line": map[string]any{
						"type":        "integer",
						"description": "The first line to read (1-indexed)",
					},
					"end_line": map[string]any{
						"type":        "integer",
						"description": "The last line to read (inclusive)",
					},
				},
				"required": []string{"file_path", "start_line", "end_line"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "ReplaceInFile",
			Description: openai.String("Replace a specific block of text in a file with another block (uniquely identified)"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "The path to the file to modify",
					},
					"old_content": map[string]any{
						"type":        "string",
						"description": "The exact content in the file to be replaced",
					},
					"new_content": map[string]any{
						"type":        "string",
						"description": "The new content to replace it with",
					},
				},
				"required": []string{"file_path", "old_content", "new_content"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "AskUser",
			Description: openai.String("Ask the user a question with a list of options to choose from, blocking until they answer"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "The question to ask the user",
					},
					"options": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"description": "The list of valid options the user can select",
					},
				},
				"required": []string{"question", "options"},
			},
		}),
	}
}
