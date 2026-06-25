package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type ConversationSummary struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Messages  int    `json:"messages"`
}

func DefaultSettings() Settings {
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

func LoadSettings() (Settings, error) {
	dir := ".talos"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return DefaultSettings(), err
	}

	path := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		settings := DefaultSettings()
		_ = SaveSettings(settings)
		return settings, nil
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings(), err
	}

	var legacy LegacySettings
	_ = json.Unmarshal(data, &legacy)

	if len(settings.Providers) == 0 {
		settings = DefaultSettings()
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
		_ = SaveSettings(settings)
	}

	return settings, nil
}

func SaveSettings(settings Settings) error {
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

func ToLocalMessages(messages []openai.ChatCompletionMessageParamUnion) []LocalMessage {
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

func ToOpenAIMessages(localMsgs []LocalMessage) []openai.ChatCompletionMessageParamUnion {
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

func SaveConversation(convID string, messages []openai.ChatCompletionMessageParamUnion) error {
	dir := filepath.Join(".talos", "conversations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	conv := Conversation{
		ID:        convID,
		CreatedAt: time.Now().Format(time.RFC3339),
		Messages:  ToLocalMessages(messages),
	}

	path := filepath.Join(dir, fmt.Sprintf("%s.json", convID))
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func ListConversations() ([]ConversationSummary, error) {
	dir := filepath.Join(".talos", "conversations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var summaries []ConversationSummary
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		convPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(convPath)
		if err != nil {
			continue
		}
		var conv Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			continue
		}
		summaries = append(summaries, ConversationSummary{
			ID:        id,
			CreatedAt: conv.CreatedAt,
			Messages:  len(conv.Messages),
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt > summaries[j].CreatedAt
	})

	return summaries, nil
}

func LoadConversation(convID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	dir := filepath.Join(".talos", "conversations")
	path := filepath.Join(dir, fmt.Sprintf("%s.json", convID))

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("conversation '%s' not found: %w", convID, err)
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("failed to parse conversation '%s': %w", convID, err)
	}

	return ToOpenAIMessages(conv.Messages), nil
}

func GetLatestConversation() (string, []openai.ChatCompletionMessageParamUnion, error) {
	summaries, err := ListConversations()
	if err != nil {
		return "", nil, err
	}
	if len(summaries) == 0 {
		return "", nil, fmt.Errorf("no conversations found")
	}

	latestID := summaries[0].ID
	messages, err := LoadConversation(latestID)
	if err != nil {
		return "", nil, err
	}
	return latestID, messages, nil
}