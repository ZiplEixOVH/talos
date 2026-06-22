# ◆ Talos

[![Go Version](https://img.shields.io/badge/go-1.26-blue)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](#license)

**Talos** is an intelligent AI coding assistant that lives in your terminal. It connects to any OpenAI-compatible LLM provider and gives the model powerful tool access — file system operations, shell commands, web search, and more — all through a beautiful TUI experience.

```shell
talos                  # Launch the interactive REPL
talos -p "hello world" # One-shot mode (legacy)
```

![Talos TUI Demo]()

---

## ✨ Features

- **Beautiful TUI** — Markdown-rendered chat with syntax highlighting, powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Glamour](https://github.com/charmbracelet/glamour)
- **Tool-using AI** — The model can read, write, search files, run shell commands, browse the web, search Google, and ask you interactive questions
- **Multi-Provider** — Swap between Ollama, OpenRouter, OpenAI, or any OpenAI-compatible API at runtime
- **Provider Management** — Full CRUD for providers and models via `/provider` slash commands
- **Conversation History** — Conversations are saved locally in `.talos/conversations/`
- **Gitignore-Aware** — File listing respects `.gitignore` rules
- **Lightweight & Offline-First** — Works with local models (Ollama) just as well as cloud APIs

---

## 🚀 Quick Start

### 1. Install

```shell
# Clone the repository
git clone https://github.com/ZiplEix/talos.git
cd talos

# Build
go build -o talos .
```

Or download a prebuilt binary from the [releases page](https://github.com/ZiplEix/talos/releases).

### 2. Configure a provider

Talos ships with sensible defaults for Ollama. If you're using another provider:

```shell
# Using OpenRouter (example)
talos
> /provider set openrouter sk-or-v1-... https://openrouter.ai/api/v1
> /provider add-model openrouter deepseek/deepseek-v4-flash
> /provider use openrouter
```

You can also edit `.talos/settings.json` directly.

### 3. Start chatting

```shell
talos
```

Type a message and press `Enter`. The AI will respond and can use tools as needed.

---

## ⌨️ Slash Commands

| Command | Description |
|---|---|
| `/help` | Show available commands |
| `/model` | Switch model (opens interactive picker) |
| `/model <name>` | Switch model directly |
| `/provider` | Switch provider (opens interactive picker) |
| `/provider use <name>` | Switch provider directly |
| `/provider set <name> <api_key> <base_url>` | Configure a provider |
| `/provider add-model <name> <model>` | Add a model to a provider |
| `/provider remove-model <name> <model>` | Remove a model from a provider |
| `/clear` or `/new` | Start a new conversation |
| `/exit` | Quit |

### Key Bindings

| Key | Action |
|---|---|
| `Enter` | Send message |
| `Shift+Enter` | Newline in input |
| `Tab` | Accept autocomplete suggestion |
| `↑` / `↓` | Navigate suggestions, scroll chat, navigate menus |
| `Esc` | Cancel suggestions / selection menus |
| `Ctrl+C` | Quit |

---

## 🛠️ Available Tools

When Talos calls the AI, the model has access to these tools:

| Tool | Description |
|---|---|
| `Read(file_path)` | Read the full content of a file |
| `Write(file_path, content)` | Write content to a file (creates if missing) |
| `Bash(command)` | Execute a shell command |
| `List(directory)` | List directory contents (JSON, gitignore-aware) |
| `FetchWebPage(url)` | Fetch the raw content of a webpage |
| `GoogleSearch(query)` | Search Google via DuckDuckGo HTML scraping |
| `FileSearch(pattern, directory)` | Recursively search for text in files |
| `ReadRange(file_path, start_line, end_line)` | Read a specific line range from a file |
| `ReplaceInFile(file_path, old_content, new_content)` | Replace a unique block of text in a file |
| `AskUser(question, options)` | Prompt the user to choose from a list of options |

---

## 📁 Project Structure

```
.
├── main.go          # Entry point, CLI flags, tool registration
├── tui.go           # Bubble Tea TUI model, views, streaming
├── tools.go         # Tool implementations (Read, Write, Bash, etc.)
├── storage.go       # Settings & conversation persistence
├── misc.go          # Shell command execution helper
├── tools_test.go    # Tests for tool implementations
├── go.mod / go.sum  # Go module dependencies
└── .talos/          # Local data directory
    ├── settings.json
    └── conversations/
```

---

## ⚙️ Configuration

Settings are stored in `.talos/settings.json`:

```json
{
  "current_provider": "ollama",
  "current_model": "gemma4:12b",
  "providers": {
    "ollama": {
      "name": "ollama",
      "api_key": "fake_api_key",
      "base_url": "http://localhost:11434/v1",
      "models": ["gemma4:12b"]
    }
  }
}
```

You can also use a `.env` file (sourced at build time) for API keys:

```env
OPENROUTER_BASE_URL="https://openrouter.ai/api/v1"
OPENROUTER_API_KEY="sk-or-v1-..."
MODEL_NAME="deepseek/deepseek-v4-flash"
```

---

## 🧪 Running Tests

```shell
go test -v ./...
```

---

## 📜 License

MIT — see the [LICENSE](./LICENSE) file for details.

---

*Built with ❤️ using [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and the [OpenAI Go SDK](https://github.com/openai/openai-go).*