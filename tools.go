package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/openai/openai-go/v3"
)

type ToolCall func(args map[string]any) (result string)

var toolCalls = map[string]ToolCall{
	"Read":      handleReadTool,
	"Write":     handleWriteTool,
	"Bash":      handleBashTool,
	"List":      handleListTool,
	"WebSearch": handleWebSearchTool,
}

func handleReadTool(args map[string]any) string {
	filePath := args["file_path"].(string)
	fmt.Printf("Reading file '%s'...\n", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
		os.Exit(1)
	}
	return string(content)
}

func handleWriteTool(args map[string]any) string {
	filePath := args["file_path"].(string)
	fmt.Printf("Writing file '%s'...\n", filePath)
	content := args["content"].(string)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing file: %v\n", err)
		os.Exit(1)
	}
	return content
}

func handleBashTool(args map[string]any) string {
	command := args["command"].(string)
	fmt.Printf("Running command: '%s'...\n", command)
	stdout, stderr := executeShellCommand(command)
	if stderr != "" {
		return stderr
	}

	return stdout
}

func handleListTool(args map[string]any) string {
	directory := args["directory"].(string)
	fmt.Printf("Listing directory '%s'...\n", directory)
	files, err := os.ReadDir(directory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing directory: %v\n", err)
		os.Exit(1)
	}

	type listEntry struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Type      string `json:"type"`
		SizeBytes int64  `json:"size_bytes"`
		Mode      string `json:"mode"`
		ModTime   string `json:"modified_at"`
		IsHidden  bool   `json:"is_hidden"`
		IsSymlink bool   `json:"is_symlink"`
		Extension string `json:"extension,omitempty"`
	}

	entries := make([]listEntry, 0, len(files))
	for _, file := range files {
		entryType := "file"
		if file.IsDir() {
			entryType = "folder"
		}
		if file.Type()&os.ModeSymlink != 0 {
			entryType = "symlink"
		}

		entry := listEntry{
			Name:      file.Name(),
			Path:      filepath.Join(directory, file.Name()),
			Type:      entryType,
			IsHidden:  len(file.Name()) > 0 && file.Name()[0] == '.',
			IsSymlink: file.Type()&os.ModeSymlink != 0,
			Extension: filepath.Ext(file.Name()),
		}

		info, infoErr := file.Info()
		if infoErr == nil {
			entry.SizeBytes = info.Size()
			entry.Mode = info.Mode().String()
			entry.ModTime = info.ModTime().Format("2006-01-02T15:04:05Z07:00")
		}

		entries = append(entries, entry)
	}

	result, _ := json.Marshal(entries)
	return string(result)
}

func handleWebSearchTool(args map[string]any) string {
	url := args["url"].(string)
	fmt.Printf("Fetching webpage: '%s'...\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("error fetching page: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("error reading body: %v", err)
	}

	return string(body)
}

func handleToolCall(toolCall openai.ChatCompletionMessageToolCallUnion) (result string, toolCallID string) {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing tool arguments: %v\n", err)
		os.Exit(1)
	}

	if _, ok := toolCalls[toolCall.Function.Name]; !ok {
		fmt.Fprintf(os.Stderr, "error: unknown tool call %s\n", toolCall.Function.Name)
		os.Exit(1)
	}

	result = toolCalls[toolCall.Function.Name](args)
	toolCallID = toolCall.ID
	return result, toolCallID
}
