package tui

import (
	"strings"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func wrapText(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = wrapLine(line, limit)
	}
	return strings.Join(lines, "\n")
}

func wrapLine(line string, limit int) string {
	if len(line) <= limit {
		return line
	}

	var sb strings.Builder
	runes := []rune(line)
	start := 0

	for start < len(runes) {
		if len(runes)-start <= limit {
			sb.WriteString(string(runes[start:]))
			break
		}

		end := start + limit
		spaceIdx := -1
		for i := end; i > start; i-- {
			if runes[i] == ' ' || runes[i] == '\t' {
				spaceIdx = i
				break
			}
		}

		if spaceIdx != -1 {
			sb.WriteString(string(runes[start:spaceIdx]))
			sb.WriteString("\n")
			start = spaceIdx + 1
		} else {
			sb.WriteString(string(runes[start:end]))
			sb.WriteString("\n")
			start = end
		}
	}
	return sb.String()
}