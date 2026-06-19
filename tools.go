package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openai/openai-go/v3"
)

var inReader io.Reader = os.Stdin

type ToolCall func(args map[string]any) (result string)

var toolCalls = map[string]ToolCall{
	"Read":         handleReadTool,
	"Write":        handleWriteTool,
	"Bash":         handleBashTool,
	"List":         handleListTool,
	"FetchWebPage": handleWebSearchTool,
	"GoogleSearch": handleGoogleSearchTool,
	"FileSearch":   handleFileSearchTool,
	"ReadRange":    handleReadRangeTool,
	"ReplaceInFile": handleReplaceInFileTool,
	"AskUser":       handleAskUserTool,
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

func handleGoogleSearchTool(args map[string]any) string {
	query, ok := args["query"].(string)
	if !ok {
		return "error: query parameter is missing or not a string"
	}
	fmt.Printf("Searching for: '%s'...\n", query)

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

	// Clean HTML tags helper
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

	fmt.Printf("Searching for pattern '%s' in '%s'...\n", pattern, dirPath)

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

	fmt.Printf("Reading file '%s' from line %d to %d...\n", filePath, startLine, endLine)

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

	fmt.Printf("Replacing content in file '%s'...\n", filePath)

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

func handleAskUserTool(args map[string]any) string {
	question, ok := args["question"].(string)
	if !ok {
		return "error: question parameter is missing or not a string"
	}

	optionsVal, ok := args["options"]
	if !ok {
		return "error: options parameter is missing"
	}

	var options []string
	if list, ok := optionsVal.([]any); ok {
		for _, item := range list {
			if str, ok := item.(string); ok {
				options = append(options, str)
			}
		}
	} else if strList, ok := optionsVal.([]string); ok {
		options = strList
	}

	if len(options) == 0 {
		return "error: options parameter must be a non-empty array of strings"
	}

	fmt.Println("\n=================== TALOS INTERACTIVE QUESTION ===================")
	fmt.Println(question)
	for i, opt := range options {
		fmt.Printf("[%d] %s\n", i+1, opt)
	}
	fmt.Println("==================================================================")

	reader := bufio.NewReader(inReader)
	for {
		fmt.Printf("Select an option (1-%d or type the option name): ", len(options))
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Sprintf("error reading user input: %v", err)
		}
		input = strings.TrimSpace(input)

		var selectedIdx int
		_, scanErr := fmt.Sscanf(input, "%d", &selectedIdx)
		if scanErr == nil && selectedIdx >= 1 && selectedIdx <= len(options) {
			return options[selectedIdx-1]
		}

		inputLower := strings.ToLower(input)
		for _, opt := range options {
			if strings.ToLower(opt) == inputLower {
				return opt
			}
		}

		fmt.Println("Invalid selection. Please try again.")
	}
}



