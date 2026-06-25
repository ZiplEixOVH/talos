package tools

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

func TestListToolWithGitIgnore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "talos_test_list_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = os.WriteFile(tempDir+"/allowed.txt", []byte("ok"), 0644)
	if err != nil {
		t.Fatalf("failed to write allowed file: %v", err)
	}

	err = os.WriteFile(tempDir+"/ignored.log", []byte("ignored"), 0644)
	if err != nil {
		t.Fatalf("failed to write ignored file: %v", err)
	}

	err = os.WriteFile(tempDir+"/.gitignore", []byte("ignored.log\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write gitignore: %v", err)
	}

	args := map[string]any{
		"directory": tempDir,
	}

	result := handleListTool(args)
	if strings.HasPrefix(result, "error:") {
		t.Fatalf("handleListTool returned error: %s", result)
	}

	var entries []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(result), &entries)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v, raw: %s", err, result)
	}

	foundAllowed := false
	foundIgnored := false

	for _, entry := range entries {
		if entry.Name == "allowed.txt" {
			foundAllowed = true
		}
		if entry.Name == "ignored.log" {
			foundIgnored = true
		}
	}

	if !foundAllowed {
		t.Errorf("expected allowed.txt to be listed")
	}
	if foundIgnored {
		t.Errorf("expected ignored.log to be excluded by gitignore, but it was listed")
	}
}

