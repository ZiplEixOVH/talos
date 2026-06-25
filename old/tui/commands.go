package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/ZiplEix/talos/storage"
)

type SlashCommand struct {
	Name string
	Desc string
}

var allSlashCommands = []SlashCommand{
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

func filterCommands(input string) []SlashCommand {
	input = strings.TrimSpace(input)
	if input == "/" {
		return allSlashCommands
	}
	var filtered []SlashCommand
	for _, cmd := range allSlashCommands {
		if strings.HasPrefix(cmd.Name, input) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

func (m *Model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
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
		m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: helpText})

	case "/clear", "/new":
		cwd := "."
		if d, err := GetCwd(); err == nil {
			cwd = d
		}
		sysPrompt := fmt.Sprintf(`Tu es Talos, un assistant de code intelligent en ligne de commande.
Le répertoire de travail actuel (CWD) est : %s.
Tu as accès à des outils pour lire, écrire, lister, rechercher des fichiers, et exécuter des commandes via Bash.
Utilise ces outils de manière ciblée, intelligente et sécurisée pour répondre aux demandes de l'utilisateur.`, cwd)
		m.oaiMessages = []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(sysPrompt)}
		m.chatMessages = []ChatMessage{}
		m.convID = fmt.Sprintf("conv_%d", time.Now().UnixNano())
		m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: "Conversation cleared."})

	case "/model":
		if len(parts) < 2 {
			activeProv := m.settings.Providers[m.settings.CurrentProvider]
			if len(activeProv.Models) == 0 {
				m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: "No models configured. Use /provider add-model to add one."})
			} else {
				m.state = StateSelectModel
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
			_ = storage.SaveSettings(m.settings)
			m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Model → %s", newModelName)})
		}

	case "/provider":
		if len(parts) < 2 {
			var provNames []string
			for name := range m.settings.Providers {
				provNames = append(provNames, name)
			}
			if len(provNames) == 0 {
				m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: "No providers configured."})
			} else {
				m.state = StateSelectProvider
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
						m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: "No providers configured."})
					} else {
						m.state = StateSelectProvider
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
						m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Provider '%s' not found.", newName)})
					} else {
						m.settings.CurrentProvider = newName
						if len(prov.Models) > 0 {
							m.settings.CurrentModel = prov.Models[0]
						}
						_ = storage.SaveSettings(m.settings)
						m.client = openai.NewClient(option.WithAPIKey(prov.APIKey), option.WithBaseURL(prov.BaseURL))
						m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Provider → %s (model: %s)", newName, m.settings.CurrentModel)})
					}
				}
			case "set":
				if len(parts) < 5 {
					m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: "Usage: /provider set <name> <api_key> <base_url>"})
				} else {
					name, apiKey, baseURL := parts[2], parts[3], parts[4]
					prov, exists := m.settings.Providers[name]
					if exists {
						prov.APIKey = apiKey
						prov.BaseURL = baseURL
					} else {
						prov = storage.Provider{Name: name, APIKey: apiKey, BaseURL: baseURL, Models: []string{}}
					}
					m.settings.Providers[name] = prov
					_ = storage.SaveSettings(m.settings)
					if name == m.settings.CurrentProvider {
						m.client = openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))
					}
					m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Provider '%s' configured.", name)})
				}
			case "add-model":
				if len(parts) < 4 {
					m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: "Usage: /provider add-model <name> <model>"})
				} else {
					name, mdl := parts[2], parts[3]
					prov, exists := m.settings.Providers[name]
					if !exists {
						m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Provider '%s' not found.", name)})
					} else {
						found := false
						for _, md := range prov.Models {
							if md == mdl {
								found = true
								break
							}
						}
						if !found {
							prov.Models = append(prov.Models, mdl)
							m.settings.Providers[name] = prov
							_ = storage.SaveSettings(m.settings)
							m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Model '%s' added to '%s'.", mdl, name)})
						} else {
							m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Model '%s' already in '%s'.", mdl, name)})
						}
					}
				}
			case "remove-model":
				if len(parts) < 4 {
					m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: "Usage: /provider remove-model <name> <model>"})
				} else {
					name, mdl := parts[2], parts[3]
					prov, exists := m.settings.Providers[name]
					if !exists {
						m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Provider '%s' not found.", name)})
					} else {
						idx := -1
						for i, md := range prov.Models {
							if md == mdl {
								idx = i
								break
							}
						}
						if idx != -1 {
							prov.Models = append(prov.Models[:idx], prov.Models[idx+1:]...)
							m.settings.Providers[name] = prov
							_ = storage.SaveSettings(m.settings)
							m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Model '%s' removed from '%s'.", mdl, name)})
						} else {
							m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Model '%s' not found in '%s'.", mdl, name)})
						}
					}
				}
			default:
				m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Unknown subcommand: %s", subCmd)})
			}
		}

	default:
		m.chatMessages = append(m.chatMessages, ChatMessage{Role: "system", Content: fmt.Sprintf("Unknown command: %s. Type /help.", cmd)})
	}

	m.updateViewport()
	m.viewport.GotoBottom()
	return m, nil
}

func accentStyle(s string) string {
	return lipgloss.NewStyle().Foreground(accentColor).Render(s)
}