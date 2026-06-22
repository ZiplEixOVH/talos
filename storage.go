package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

type Provider struct {
	Name    string   `json:"name"`
	APIKey  string   `json:"api_key"`
	BaseURL string   `json:"base_url"`
	Models  []string `json:"models"`
}

type Settings struct {
	CurrentProvider string              `json:"current_provider"`
	CurrentModel    string              `json:"current_model"`
	Providers       map[string]Provider `json:"providers"`
}

type LegacySettings struct {
	Model   string `json:"model"`
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

type LocalMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []LocalToolCall `json:"tool_calls,omitempty"`
}

type LocalToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Conversation struct {
	ID        string         `json:"id"`
	CreatedAt string         `json:"created_at"`
	Messages  []LocalMessage `json:"messages"`
}

func defaultSettings() Settings {
	providers := map[string]Provider{
		"ollama": {
			Name:    "ollama",
			APIKey:  "fake_api_key",
			BaseURL: "http://localhost:11434/v1",
			Models:  []string{"gemma4:12b"},
		},
	}

	return Settings{
		CurrentProvider: "ollama",
		CurrentModel:    "gemma4:12b",
		Providers:       providers,
	}
}


func loadSettings() (Settings, error) {
	dir := ".talos"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return defaultSettings(), err
	}

	path := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		settings := defaultSettings()
		_ = saveSettings(settings)
		return settings, nil
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultSettings(), err
	}

	var legacy LegacySettings
	_ = json.Unmarshal(data, &legacy)

	if len(settings.Providers) == 0 {
		settings = defaultSettings()
		if legacy.Model != "" {
			settings.CurrentModel = legacy.Model
		}
		if legacy.APIKey != "" || legacy.BaseURL != "" {
			prov := settings.Providers[settings.CurrentProvider]
			if legacy.APIKey != "" {
				prov.APIKey = legacy.APIKey
			}
			if legacy.BaseURL != "" {
				prov.BaseURL = legacy.BaseURL
			}
			settings.Providers[settings.CurrentProvider] = prov
		}
		_ = saveSettings(settings)
	}

	return settings, nil
}

func saveSettings(settings Settings) error {
	dir := ".talos"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "settings.json")
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func toLocalMessages(messages []openai.ChatCompletionMessageParamUnion) []LocalMessage {
	localMsgs := make([]LocalMessage, 0, len(messages))
	for _, m := range messages {
		jsonData, err := json.Marshal(m)
		if err != nil {
			continue
		}

		var localMsg LocalMessage
		if err := json.Unmarshal(jsonData, &localMsg); err == nil {
			localMsgs = append(localMsgs, localMsg)
		}
	}
	return localMsgs
}

func toOpenAIMessages(localMsgs []LocalMessage) []openai.ChatCompletionMessageParamUnion {
	oaMsgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(localMsgs))
	for _, lm := range localMsgs {
		switch lm.Role {
		case "system":
			oaMsgs = append(oaMsgs, openai.SystemMessage(lm.Content))
		case "user":
			oaMsgs = append(oaMsgs, openai.UserMessage(lm.Content))
		case "assistant":
			if len(lm.ToolCalls) > 0 {
				var tcs []openai.ChatCompletionMessageToolCallUnionParam
				for _, tc := range lm.ToolCalls {
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
				if lm.Content != "" {
					assistantParam.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: param.NewOpt(lm.Content),
					}
				}
				oaMsgs = append(oaMsgs, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &assistantParam,
				})
			} else {
				oaMsgs = append(oaMsgs, openai.AssistantMessage(lm.Content))
			}
		case "tool":
			oaMsgs = append(oaMsgs, openai.ToolMessage(lm.Content, lm.ToolCallID))
		}
	}
	return oaMsgs
}

func saveConversation(convID string, messages []openai.ChatCompletionMessageParamUnion) error {
	dir := filepath.Join(".talos", "conversations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	conv := Conversation{
		ID:        convID,
		CreatedAt: time.Now().Format(time.RFC3339),
		Messages:  toLocalMessages(messages),
	}

	path := filepath.Join(dir, fmt.Sprintf("%s.json", convID))
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
