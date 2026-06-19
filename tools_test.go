package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGoogleSearchTool(t *testing.T) {
	args := map[string]any{
		"query": "golang",
	}

	result := handleGoogleSearchTool(args)
	if strings.HasPrefix(result, "error:") {
		t.Fatalf("handleGoogleSearchTool returned an error: %s", result)
	}

	var results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
	}

	err := json.Unmarshal([]byte(result), &results)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON result: %v, raw: %s", err, result)
	}

	if len(results) == 0 {
		t.Logf("No results returned. Note: This could be due to rate limiting or network issues, but the tool did not return an error.")
		return
	}

	t.Logf("Found %d search results", len(results))
	for i, res := range results {
		if res.Title == "" {
			t.Errorf("Result %d: empty title", i)
		}
		if res.URL == "" {
			t.Errorf("Result %d: empty URL", i)
		}
		t.Logf("- %s (%s)", res.Title, res.URL)
	}
}

func TestReadRangeTool(t *testing.T) {
	args := map[string]any{
		"file_path":  "tools_test.go",
		"start_line": 3,
		"end_line":   7,
	}

	result := handleReadRangeTool(args)
	if strings.HasPrefix(result, "error:") {
		t.Fatalf("handleReadRangeTool returned an error: %s", result)
	}

	expectedLines := []string{
		"import (",
		"\t\"encoding/json\"",
		"\t\"os\"",
		"\t\"strings\"",
		"\t\"testing\"",
	}

	expected := strings.Join(expectedLines, "\n")
	if strings.TrimSpace(result) != strings.TrimSpace(expected) {
		t.Errorf("ReadRange returned:\n%s\nExpected:\n%s", result, expected)
	}
}

func TestFileSearchTool(t *testing.T) {
	args := map[string]any{
		"pattern":   "TestGoogleSearchTool",
		"directory": ".",
	}

	result := handleFileSearchTool(args)
	if strings.HasPrefix(result, "error:") {
		t.Fatalf("handleFileSearchTool returned an error: %s", result)
	}

	var matches []struct {
		FilePath string `json:"file_path"`
		Line     int    `json:"line"`
		Content  string `json:"content"`
	}

	err := json.Unmarshal([]byte(result), &matches)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON result: %v, raw: %s", err, result)
	}

	if len(matches) == 0 {
		t.Errorf("expected to find at least one match for TestGoogleSearchTool, got 0")
	}

	foundTestFile := false
	for _, m := range matches {
		if strings.Contains(m.FilePath, "tools_test.go") {
			foundTestFile = true
		}
		t.Logf("Match in %s:%d: %s", m.FilePath, m.Line, m.Content)
	}

	if !foundTestFile {
		t.Errorf("expected to find match in tools_test.go")
	}
}

func TestReplaceInFileTool(t *testing.T) {
	tempFile := "test_replace_temp.txt"
	initialContent := "line 1\nline 2 (target)\nline 3\nline 2 (target)\n"
	err := os.WriteFile(tempFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp test file: %v", err)
	}
	defer os.Remove(tempFile)

	// Case 1: missing target content
	argsMissing := map[string]any{
		"file_path":   tempFile,
		"old_content": "non-existent line",
		"new_content": "replaced line",
	}
	resMissing := handleReplaceInFileTool(argsMissing)
	if !strings.Contains(resMissing, "error: old_content was not found") {
		t.Errorf("expected missing target error, got: %s", resMissing)
	}

	// Case 2: ambiguous content (multiple occurrences)
	argsAmbiguous := map[string]any{
		"file_path":   tempFile,
		"old_content": "line 2 (target)",
		"new_content": "replaced line",
	}
	resAmbiguous := handleReplaceInFileTool(argsAmbiguous)
	if !strings.Contains(resAmbiguous, "error: old_content matches multiple locations") {
		t.Errorf("expected ambiguous match error, got: %s", resAmbiguous)
	}

	// Case 3: successful unique match replacement
	argsSuccess := map[string]any{
		"file_path":   tempFile,
		"old_content": "line 3",
		"new_content": "line 3 (replaced)",
	}
	resSuccess := handleReplaceInFileTool(argsSuccess)
	if !strings.Contains(resSuccess, "success:") {
		t.Errorf("expected success message, got: %s", resSuccess)
	}

	// Verify file content post replacement
	updatedBytes, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read updated test file: %v", err)
	}
	updatedContent := string(updatedBytes)
	expectedContent := "line 1\nline 2 (target)\nline 3 (replaced)\nline 2 (target)\n"
	if updatedContent != expectedContent {
		t.Errorf("unexpected updated content:\nGot:\n%s\nExpected:\n%s", updatedContent, expectedContent)
	}
}

func TestAskUserTool(t *testing.T) {
	// Backup original inReader
	oldInReader := inReader
	defer func() {
		inReader = oldInReader
	}()

	// Case 1: user enters a numeric choice (e.g. 2 for "yes")
	inReader = strings.NewReader("2\n")
	args := map[string]any{
		"question": "Do you like Go?",
		"options":  []any{"no", "yes"},
	}

	result1 := handleAskUserTool(args)
	if result1 != "yes" {
		t.Errorf("expected 'yes' for numeric selection 2, got: '%s'", result1)
	}

	// Case 2: user enters a text choice directly (case-insensitive, e.g. "NO")
	inReader = strings.NewReader("NO\n")
	result2 := handleAskUserTool(args)
	if result2 != "no" {
		t.Errorf("expected 'no' for text selection 'NO', got: '%s'", result2)
	}

	// Case 3: invalid selection followed by a valid selection
	inReader = strings.NewReader("invalid\n3\n1\n")
	result3 := handleAskUserTool(args)
	if result3 != "no" {
		t.Errorf("expected 'no' after recovering from invalid inputs, got: '%s'", result3)
	}
}
