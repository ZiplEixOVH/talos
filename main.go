package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/manifoldco/promptui"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	userStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#50FA7B"))

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2"))

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555"))
)

func runRepl(client openai.Client, settings Settings) {
	fmt.Println(titleStyle.Render("=== TALOS COOPERATIVE ASSISTANT REPL ==="))
	fmt.Printf("Active Provider: %s\n", settings.CurrentProvider)
	fmt.Printf("Active Model: %s\n", settings.CurrentModel)
	fmt.Println("Type /help for a list of available commands.")
	fmt.Println()

	convID := fmt.Sprintf("conv_%d", time.Now().UnixNano())
	messages := []openai.ChatCompletionMessageParamUnion{}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}
	sysPrompt := fmt.Sprintf(`Tu es Talos, un assistant de code intelligent en ligne de commande.
Le répertoire de travail actuel (CWD) est : %s.
Tu as accès à des outils pour lire, écrire, lister, rechercher des fichiers, et exécuter des commandes via Bash.
Utilise ces outils de manière ciblée, intelligente et sécurisée pour répondre aux demandes de l'utilisateur.`, cwd)
	messages = append(messages, openai.SystemMessage(sysPrompt))

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(userStyle.Render("Talos > "))
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error reading input: %v", err)))
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle Slash Commands
		if strings.HasPrefix(input, "/") {
			parts := strings.Fields(input)
			cmd := parts[0]

			switch cmd {
			case "/exit":
				fmt.Println("Goodbye!")
				return

			case "/help":
				fmt.Println("Available commands:")
				fmt.Println("  /exit                             Quit the program")
				fmt.Println("  /clear, /new                      Start a new conversation")
				fmt.Println("  /model                            List active and available models for the provider")
				fmt.Println("  /model <name>                     Switch to the specified model")
				fmt.Println("  /provider                         List active and configured providers")
				fmt.Println("  /provider use <name>              Switch to the specified provider")
				fmt.Println("  /provider set <name> <key> <url>  Configure/update a provider")
				fmt.Println("  /provider add-model <name> <mdl>  Add a model to a provider")
				fmt.Println("  /provider remove-model <n> <mdl>  Remove a model from a provider")
				fmt.Println("  /help                             Show this help message")

			case "/clear", "/new":
				convID = fmt.Sprintf("conv_%d", time.Now().UnixNano())
				messages = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(sysPrompt)}
				fmt.Println(systemStyle.Render("Conversation cleared. Started a new session."))

			case "/model":
				activeProvider := settings.Providers[settings.CurrentProvider]
				if len(parts) < 2 {
					if len(activeProvider.Models) == 0 {
						fmt.Println("No models configured for the current provider. Use '/provider add-model' to add one.")
						break
					}

					activeIdx := 0
					for i, mdl := range activeProvider.Models {
						if mdl == settings.CurrentModel {
							activeIdx = i
							break
						}
					}

					prompt := promptui.Select{
						Label:     "Select active model (Arrow keys to navigate, Enter to select)",
						Items:     activeProvider.Models,
						CursorPos: activeIdx,
					}

					_, selected, err := prompt.Run()
					if err != nil {
						break
					}

					settings.CurrentModel = selected
					_ = saveSettings(settings)
					fmt.Println(systemStyle.Render(fmt.Sprintf("Model changed to: %s", settings.CurrentModel)))
				} else {
					newModelName := parts[1]
					settings.CurrentModel = newModelName

					found := false
					for _, mdl := range activeProvider.Models {
						if mdl == newModelName {
							found = true
							break
						}
					}
					if !found {
						activeProvider.Models = append(activeProvider.Models, newModelName)
						settings.Providers[settings.CurrentProvider] = activeProvider
					}

					_ = saveSettings(settings)
					fmt.Println(systemStyle.Render(fmt.Sprintf("Model changed to: %s", settings.CurrentModel)))
				}

			case "/provider":
				if len(parts) < 2 {
					fmt.Printf("Active provider: %s\n", settings.CurrentProvider)
					fmt.Println("Configured providers:")
					for name, prov := range settings.Providers {
						activeMarker := " "
						if name == settings.CurrentProvider {
							activeMarker = "*"
						}
						fmt.Printf("  %s %s (%s)\n", activeMarker, name, prov.BaseURL)
					}
					fmt.Println("\nCommands to manage providers:")
					fmt.Println("  /provider use <name>")
					fmt.Println("  /provider set <name> <api_key> <base_url>")
					fmt.Println("  /provider add-model <name> <model>")
					fmt.Println("  /provider remove-model <name> <model>")
				} else {
					subCmd := parts[1]
					switch subCmd {
					case "use":
						var newName string
						if len(parts) < 3 {
							var provNames []string
							for name := range settings.Providers {
								provNames = append(provNames, name)
							}
							if len(provNames) == 0 {
								fmt.Println("No providers configured.")
								break
							}

							activeIdx := 0
							for i, n := range provNames {
								if n == settings.CurrentProvider {
									activeIdx = i
									break
								}
							}

							prompt := promptui.Select{
								Label:     "Select active provider (Arrow keys to navigate, Enter to select)",
								Items:     provNames,
								CursorPos: activeIdx,
							}
							_, selected, err := prompt.Run()
							if err != nil {
								break
							}
							newName = selected
						} else {
							newName = parts[2]
						}

						prov, exists := settings.Providers[newName]
						if !exists {
							fmt.Printf("Provider '%s' does not exist. Use '/provider set' to configure it.\n", newName)
							break
						}
						settings.CurrentProvider = newName
						if len(prov.Models) > 0 {
							settings.CurrentModel = prov.Models[0]
						}
						_ = saveSettings(settings)
						client = openai.NewClient(option.WithAPIKey(prov.APIKey), option.WithBaseURL(prov.BaseURL))
						fmt.Println(systemStyle.Render(fmt.Sprintf("Switched to provider '%s' (active model: %s)", newName, settings.CurrentModel)))

					case "set":
						if len(parts) < 5 {
							fmt.Println("Usage: /provider set <name> <api_key> <base_url>")
							break
						}
						name := parts[2]
						apiKey := parts[3]
						baseURL := parts[4]

						prov, exists := settings.Providers[name]
						if exists {
							prov.APIKey = apiKey
							prov.BaseURL = baseURL
						} else {
							prov = Provider{
								Name:    name,
								APIKey:  apiKey,
								BaseURL: baseURL,
								Models:  []string{},
							}
						}
						settings.Providers[name] = prov
						_ = saveSettings(settings)

						if name == settings.CurrentProvider {
							client = openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))
						}
						fmt.Println(systemStyle.Render(fmt.Sprintf("Provider '%s' configured successfully.", name)))

					case "add-model":
						if len(parts) < 4 {
							fmt.Println("Usage: /provider add-model <name> <model>")
							break
						}
						name := parts[2]
						mdl := parts[3]

						prov, exists := settings.Providers[name]
						if !exists {
							fmt.Printf("Provider '%s' does not exist.\n", name)
							break
						}

						found := false
						for _, m := range prov.Models {
							if m == mdl {
								found = true
								break
							}
						}
						if !found {
							prov.Models = append(prov.Models, mdl)
							settings.Providers[name] = prov
							_ = saveSettings(settings)
							fmt.Println(systemStyle.Render(fmt.Sprintf("Model '%s' added to provider '%s'.", mdl, name)))
						} else {
							fmt.Println(systemStyle.Render(fmt.Sprintf("Model '%s' is already in provider '%s'.", mdl, name)))
						}

					case "remove-model":
						if len(parts) < 4 {
							fmt.Println("Usage: /provider remove-model <name> <model>")
							break
						}
						name := parts[2]
						mdl := parts[3]

						prov, exists := settings.Providers[name]
						if !exists {
							fmt.Printf("Provider '%s' does not exist.\n", name)
							break
						}

						idx := -1
						for i, m := range prov.Models {
							if m == mdl {
								idx = i
								break
							}
						}
						if idx != -1 {
							prov.Models = append(prov.Models[:idx], prov.Models[idx+1:]...)
							settings.Providers[name] = prov
							_ = saveSettings(settings)
							fmt.Println(systemStyle.Render(fmt.Sprintf("Model '%s' removed from provider '%s'.", mdl, name)))
						} else {
							fmt.Println(systemStyle.Render(fmt.Sprintf("Model '%s' not found in provider '%s'.", mdl, name)))
						}

					default:
						fmt.Printf("Unknown provider subcommand: %s. Type /provider for help.\n", subCmd)
					}
				}

			default:
				fmt.Printf("Unknown slash command: %s. Type /help for help.\n", cmd)
			}
			continue
		}

		// Regular chat input
		messages = append(messages, openai.UserMessage(input))

		// Loop to allow handling tools repeatedly
		for {
			ctx := context.Background()
			params := openai.ChatCompletionNewParams{
				Model:    openai.ChatModel(settings.CurrentModel),
				Messages: messages,
				Tools:    getRegisteredTools(),
			}

			fmt.Print(assistantStyle.Render("Talos: "))
			os.Stdout.Sync()

			stream := client.Chat.Completions.NewStreaming(ctx, params)
			acc := openai.ChatCompletionAccumulator{}

			var streamedContent strings.Builder
			for stream.Next() {
				chunk := stream.Current()
				acc.AddChunk(chunk)

				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta
					if delta.Content != "" {
						fmt.Print(delta.Content)
						os.Stdout.Sync()
						streamedContent.WriteString(delta.Content)
					}
				}
			}

			if err := stream.Err(); err != nil {
				fmt.Println()
				fmt.Println(errorStyle.Render(fmt.Sprintf("Error from API: %v", err)))
				break
			}

			fmt.Println() // Print newline after output is fully streamed

			var toolCalls []openai.ChatCompletionMessageToolCallUnion
			if len(acc.Choices) > 0 {
				toolCalls = acc.Choices[0].Message.ToolCalls
			}

			// Add assistant response to messages
			if len(toolCalls) > 0 {
				var tcs []openai.ChatCompletionMessageToolCallUnionParam
				for _, tc := range toolCalls {
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
				if streamedContent.Len() > 0 {
					assistantParam.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: param.NewOpt(streamedContent.String()),
					}
				}
				messages = append(messages, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &assistantParam,
				})

				// Execute tools synchronously and append result messages
				for _, tc := range toolCalls {
					paramVal := getToolParamValue(tc.Function.Name, tc.Function.Arguments)
					fmt.Printf("%s(%s)\n", toolStyle.Render(tc.Function.Name), paramVal)
					result, toolCallID := handleToolCall(tc)
					messages = append(messages, openai.ToolMessage(result, toolCallID))
				}

				_ = saveConversation(convID, messages)

				// Loop back immediately to feed tool outputs back to LLM
				continue
			} else {
				if streamedContent.Len() > 0 {
					messages = append(messages, openai.AssistantMessage(streamedContent.String()))
				}
				_ = saveConversation(convID, messages)
				break // Done with this turn
			}
		}
	}
}

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

	runRepl(client, settings)
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
