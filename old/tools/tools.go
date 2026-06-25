package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openai/openai-go/v3"
	ignore "github.com/sabhiram/go-gitignore"
)

// ToolCall is the handler signature for a tool.
type ToolCall func(args map[string]any) string

// ToolDefinition is a single-source-of-truth for a tool.
type ToolDefinition struct {
	Name             string
	Description      string
	Parameters       openai.FunctionParameters
	Handler          ToolCall
	PrimaryParamName string // the main parameter used for logging via GetToolParamValue
}

// registeredTools is the single place where all tools are declared.
var registeredTools = []ToolDefinition{
	{
		Name:        "Read",
		Description: "Read and return the content of a file",
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
		Handler:          handleReadTool,
		PrimaryParamName: "file_path",
	},
	{
		Name:        "Write",
		Description: "Write content to a file, create the file if it does not exist",
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
		Handler:          handleWriteTool,
		PrimaryParamName: "file_path",
	},
	{
		Name:        "Mkdir",
		Description: "Create a directory (including parent directories if needed)",
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"directory_path": map[string]any{
					"type":        "string",
					"description": "The path to the directory to create",
				},
			},
			"required": []string{"directory_path"},
		},
		Handler:          handleMkdirTool,
		PrimaryParamName: "directory_path",
	},
	{
		Name:        "Bash",
		Description: "Execute a shell command",
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
		Handler:          handleBashTool,
		PrimaryParamName: "command",
	},
	{
		Name:        "List",
		Description: "List files in a directory",
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
		Handler:          handleListTool,
		PrimaryParamName: "directory",
	},
	{
		Name:        "Tree",
		Description: "Display a visual tree representation of a directory structure (respects .gitignore)",
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"directory": map[string]any{
					"type":        "string",
					"description": "The directory path to display the tree for",
				},
				"max_depth": map[string]any{
					"type":        "integer",
					"description": "Maximum depth to traverse (default: 5)",
				},
			},
			"required": []string{"directory"},
		},
		Handler:          handleTreeTool,
		PrimaryParamName: "directory",
	},
	{
		Name:        "FetchWebPage",
		Description: "Fetch the content of a webpage",
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
		Handler:          handleWebSearchTool,
		PrimaryParamName: "url",
	},
	{
		Name:        "GoogleSearch",
		Description: "Search Google for a given query and return a list of search results (titles, URLs, snippets)",
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
		Handler:          handleGoogleSearchTool,
		PrimaryParamName: "query",
	},
	{
		Name:        "FileSearch",
		Description: "Search for a pattern or keyword recursively within a directory or file",
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
		Handler:          handleFileSearchTool,
		PrimaryParamName: "pattern",
	},
	{
		Name:        "ReadRange",
		Description: "Read a specific line range from a file, avoiding loading the entire file",
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
		Handler:          handleReadRangeTool,
		PrimaryParamName: "file_path",
	},
	{
		Name:        "ReplaceInFile",
		Description: "Replace a specific block of text in a file with another block (uniquely identified)",
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
		Handler:          handleReplaceInFileTool,
		PrimaryParamName: "file_path",
	},

}

// toolCalls is built automatically from registeredTools.
var toolCalls map[string]ToolCall

func init() {
	toolCalls = make(map[string]ToolCall, len(registeredTools))
	for _, td := range registeredTools {
		toolCalls[td.Name] = td.Handler
	}
}

// GetToolParamValue returns the value of the primary parameter for a tool.
func GetToolParamValue(name string, argumentsJSON string) string {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return ""
	}

	for _, td := range registeredTools {
		if td.Name == name && td.PrimaryParamName != "" {
			if v, ok := args[td.PrimaryParamName]; ok {
				if str, ok := v.(string); ok {
					return str
				}
				return fmt.Sprintf("%v", v)
			}
			break
		}
	}

	// Fallback: return first string param found
	if len(args) > 0 {
		for _, v := range args {
			return fmt.Sprintf("%v", v)
		}
	}

	return ""
}

// GetRegisteredTools builds the OpenAI tool definitions from registeredTools.
func GetRegisteredTools() []openai.ChatCompletionToolUnionParam {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(registeredTools))
	for _, td := range registeredTools {
		result = append(result, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        td.Name,
			Description: openai.String(td.Description),
			Parameters:  td.Parameters,
		}))
	}
	return result
}

// HandleToolCall dispatches a tool call to its registered handler.
func HandleToolCall(toolCall openai.ChatCompletionMessageToolCallUnion) (result string, toolCallID string) {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		exitError("error parsing tool arguments: %v", err)
	}
	handler, ok := toolCalls[toolCall.Function.Name]
	if !ok {
		exitError("error: unknown tool call %s", toolCall.Function.Name)
	}
	result = handler(args)
	toolCallID = toolCall.ID
	return result, toolCallID
}

// ---------------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------------

func exitError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func executeShellCommand(command string) (stdout string, stderr string) {
	cmd := exec.Command("sh", "-c", command)

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	err := cmd.Run()
	stdout = stdoutBuffer.String()
	stderr = stderrBuffer.String()

	if err != nil && stderr == "" {
		stderr = err.Error()
	}

	return stdout, stderr
}

func findGitIgnore(startPath string) (string, bool) {
	absStart, err := filepath.Abs(startPath)
	if err != nil {
		return "", false
	}
	curr := absStart
	for {
		ignorePath := filepath.Join(curr, ".gitignore")
		if _, err := os.Stat(ignorePath); err == nil {
			return ignorePath, true
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Tool implementations
// ---------------------------------------------------------------------------

func handleReadTool(args map[string]any) string {
	filePath := args["file_path"].(string)
	content, err := os.ReadFile(filePath)
	if err != nil {
		exitError("error reading file: %v", err)
	}
	return string(content)
}

func handleWriteTool(args map[string]any) string {
	filePath := args["file_path"].(string)
	content := args["content"].(string)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		exitError("error writing file: %v", err)
	}
	return content
}

func handleMkdirTool(args map[string]any) string {
	dirPath := args["directory_path"].(string)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		exitError("error creating directory: %v", err)
	}
	return dirPath
}

func handleBashTool(args map[string]any) string {
	command := args["command"].(string)
	stdout, stderr := executeShellCommand(command)
	if stderr != "" {
		return stderr
	}
	return stdout
}

func handleListTool(args map[string]any) string {
	directory := args["directory"].(string)
	files, err := os.ReadDir(directory)
	if err != nil {
		exitError("error listing directory: %v", err)
	}

	var gitignoreObj *ignore.GitIgnore
	var gitignoreDir string
	if gitignorePath, found := findGitIgnore(directory); found {
		if obj, err := ignore.CompileIgnoreFile(gitignorePath); err == nil {
			gitignoreObj = obj
			gitignoreDir = filepath.Dir(gitignorePath)
		}
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
		filePath := filepath.Join(directory, file.Name())
		if gitignoreObj != nil {
			relPath, err := filepath.Rel(gitignoreDir, filePath)
			if err == nil {
				if gitignoreObj.MatchesPath(relPath) {
					continue
				}
			}
		}
		entryType := "file"
		if file.IsDir() {
			entryType = "folder"
		}
		if file.Type()&os.ModeSymlink != 0 {
			entryType = "symlink"
		}
		entry := listEntry{
			Name:      file.Name(),
			Path:      filePath,
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

func handleTreeTool(args map[string]any) string {
	directory, ok := args["directory"].(string)
	if !ok {
		return "error: directory parameter is missing or not a string"
	}

	var gitignoreObj *ignore.GitIgnore
	var gitignoreDir string
	if gitignorePath, found := findGitIgnore(directory); found {
		if obj, err := ignore.CompileIgnoreFile(gitignorePath); err == nil {
			gitignoreObj = obj
			gitignoreDir = filepath.Dir(gitignorePath)
		}
	}

	var buf strings.Builder
	maxDepth := 5
	if d, hasDepth := args["max_depth"]; hasDepth {
		if f, ok := d.(float64); ok {
			maxDepth = int(f)
		}
	}

	walkTree(&buf, directory, 0, maxDepth, gitignoreObj, gitignoreDir, "")
	return buf.String()
}

type treeLine struct {
	prefix string
	isLast bool
}

func walkTree(buf *strings.Builder, dir string, depth int, maxDepth int, gitignoreObj *ignore.GitIgnore, gitignoreDir string, prefix string) {
	if depth > maxDepth {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(buf, "%s[error: %v]\n", prefix, err)
		return
	}

	var dirs []os.DirEntry
	var files []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		if name != "." && name != ".." && len(name) > 0 && name[0] == '.' {
			continue
		}
		if gitignoreObj != nil {
			relPath, err := filepath.Rel(gitignoreDir, filepath.Join(dir, name))
			if err == nil && gitignoreObj.MatchesPath(relPath) {
				continue
			}
		}
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	all := append(dirs, files...)

	for i, entry := range all {
		isLast := i == len(all)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		fmt.Fprintf(buf, "%s%s%s\n", prefix, connector, entry.Name())

		if entry.IsDir() {
			nextPrefix := prefix
			if isLast {
				nextPrefix += "    "
			} else {
				nextPrefix += "│   "
			}
			walkTree(buf, filepath.Join(dir, entry.Name()), depth+1, maxDepth, gitignoreObj, gitignoreDir, nextPrefix)
		}
	}
}

func handleWebSearchTool(args map[string]any) string {
	url := args["url"].(string)
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

func handleGoogleSearchTool(args map[string]any) string {
	query, ok := args["query"].(string)
	if !ok {
		return "error: query parameter is missing or not a string"
	}

	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return fmt.Sprintf("error creating request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("error performing search request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("error: search request failed with status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("error reading search response body: %v", err)
	}

	html := string(body)

	cleanHTML := func(input string) string {
		re := regexp.MustCompile(`<[^>]*>`)
		res := re.ReplaceAllString(input, "")
		res = strings.ReplaceAll(res, "&amp;", "&")
		res = strings.ReplaceAll(res, "&quot;", "\"")
		res = strings.ReplaceAll(res, "&#x27;", "'")
		res = strings.ReplaceAll(res, "&lt;", "<")
		res = strings.ReplaceAll(res, "&gt;", ">")
		return strings.TrimSpace(res)
	}

	alternativeBlocks := strings.Split(html, `<div class="result results_links`)
	if len(alternativeBlocks) <= 1 {
		return "[]"
	}

	type SearchResult struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
	}

	var results []SearchResult
	titleRe := regexp.MustCompile(`class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`class="result__snippet"[^>]*>(.*?)</a>`)

	for _, block := range alternativeBlocks[1:] {
		titleMatch := titleRe.FindStringSubmatch(block)
		if len(titleMatch) < 3 {
			continue
		}
		resURL := titleMatch[1]
		title := cleanHTML(titleMatch[2])

		snippet := ""
		snippetMatch := snippetRe.FindStringSubmatch(block)
		if len(snippetMatch) >= 2 {
			snippet = cleanHTML(snippetMatch[1])
		}

		results = append(results, SearchResult{
			Title:   title,
			URL:     resURL,
			Snippet: snippet,
		})
	}

	jsonData, err := json.Marshal(results)
	if err != nil {
		return fmt.Sprintf("error serializing search results: %v", err)
	}

	return string(jsonData)
}

func handleFileSearchTool(args map[string]any) string {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "error: pattern parameter is missing or not a string"
	}
	dirPath, ok := args["directory"].(string)
	if !ok {
		return "error: directory parameter is missing or not a string"
	}

	type Match struct {
		FilePath string `json:"file_path"`
		Line     int    `json:"line"`
		Content  string `json:"content"`
	}

	var matches []Match
	maxMatches := 100
	patternLower := strings.ToLower(pattern)

	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Sprintf("error checking path: %v", err)
	}

	searchFile := func(path string) error {
		fi, err := os.Stat(path)
		if err != nil {
			return nil
		}
		if fi.Size() > 1024*1024 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == 0 {
				return nil
			}
		}
		_, _ = file.Seek(0, 0)

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), patternLower) {
				matches = append(matches, Match{
					FilePath: path,
					Line:     lineNum,
					Content:  strings.TrimSpace(line),
				})
				if len(matches) >= maxMatches {
					break
				}
			}
		}
		return nil
	}

	if !info.IsDir() {
		err = searchFile(dirPath)
		if err != nil {
			return fmt.Sprintf("error searching file: %v", err)
		}
	} else {
		err = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if len(matches) >= maxMatches {
				return filepath.SkipDir
			}
			if d.IsDir() {
				name := d.Name()
				if name != "." && name != ".." && ((len(name) > 0 && name[0] == '.') || name == "node_modules" || name == "vendor") {
					return filepath.SkipDir
				}
				return nil
			}
			return searchFile(path)
		})
		if err != nil {
			return fmt.Sprintf("error traversing directory: %v", err)
		}
	}

	jsonData, err := json.Marshal(matches)
	if err != nil {
		return fmt.Sprintf("error marshaling search results: %v", err)
	}

	return string(jsonData)
}

func handleReadRangeTool(args map[string]any) string {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return "error: file_path parameter is missing or not a string"
	}

	startLineVal, hasStart := args["start_line"]
	var startLine int
	if hasStart {
		if sf, ok := startLineVal.(float64); ok {
			startLine = int(sf)
		} else if si, ok := startLineVal.(int); ok {
			startLine = si
		}
	}
	if startLine <= 0 {
		startLine = 1
	}

	endLineVal, hasEnd := args["end_line"]
	var endLine int
	if hasEnd {
		if ef, ok := endLineVal.(float64); ok {
			endLine = int(ef)
		} else if ei, ok := endLineVal.(int); ok {
			endLine = ei
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Sprintf("error opening file: %v", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	currentLine := 0
	for scanner.Scan() {
		currentLine++
		if currentLine >= startLine {
			if endLine > 0 && currentLine > endLine {
				break
			}
			lines = append(lines, scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Sprintf("error reading file: %v", err)
	}

	return strings.Join(lines, "\n")
}

func handleReplaceInFileTool(args map[string]any) string {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return "error: file_path parameter is missing or not a string"
	}
	oldContent, ok := args["old_content"].(string)
	if !ok {
		return "error: old_content parameter is missing or not a string"
	}
	newContent, ok := args["new_content"].(string)
	if !ok {
		return "error: new_content parameter is missing or not a string"
	}

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("error reading file: %v", err)
	}
	content := string(contentBytes)

	count := strings.Count(content, oldContent)
	if count == 0 {
		return "error: old_content was not found in the file"
	}
	if count > 1 {
		return fmt.Sprintf("error: old_content matches multiple locations (%d occurrences); please provide more surrounding lines to uniquely identify the block to replace", count)
	}

	updatedContent := strings.Replace(content, oldContent, newContent, 1)

	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Sprintf("error writing updated content to file: %v", err)
	}

	return "success: content replaced successfully"
}