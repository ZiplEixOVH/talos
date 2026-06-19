package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	// apiKey := os.Getenv("OPENROUTER_API_KEY")
	// baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	// if baseUrl == "" {
	// 	baseUrl = "https://openrouter.ai/api/v1"
	// }

	apiKey := "fake_api_key"
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "http://localhost:11434/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	for {
		resp := callApi(client, messages)

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

func callApi(client openai.Client, messages []openai.ChatCompletionMessageParamUnion) *openai.ChatCompletion {
	resp, err := client.Chat.Completions.New(context.Background(),
		openai.ChatCompletionNewParams{
			Model:    "gemma4:12b",
			Messages: messages,
			Tools: []openai.ChatCompletionToolUnionParam{
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
					Name:        "WebSearch",
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
			},
		},
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(resp.Choices) == 0 {
		panic("No choices in response")
	}

	return resp
}
