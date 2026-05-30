package reviewer

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DiffLine represents a single line in a diff context.
type DiffLine struct {
	Type string `json:"type"` // "context", "add", "del"
	Line int    `json:"line"` // line number in new file (0 for del lines)
	Text string `json:"text"`
}

// ExtractDiffContext parses a unified diff and extracts context around the target line.
// Returns a JSON array of DiffLine, or empty string if parsing fails.
func ExtractDiffContext(diff string, targetLine int, contextLines int) string {
	if diff == "" || targetLine <= 0 {
		return ""
	}

	lines := parseDiffLines(diff)
	if len(lines) == 0 {
		return ""
	}

	// Find the range around targetLine
	start := targetLine - contextLines
	if start < 1 {
		start = 1
	}
	end := targetLine + contextLines

	var result []DiffLine
	for _, dl := range lines {
		if dl.Line == 0 {
			// deleted lines: include if adjacent to target range
			continue
		}
		if dl.Line >= start && dl.Line <= end {
			result = append(result, dl)
		}
	}

	if len(result) == 0 {
		return ""
	}

	data, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	return string(data)
}

// parseDiffLines parses unified diff into a list of DiffLine with new file line numbers.
func parseDiffLines(diff string) []DiffLine {
	var result []DiffLine
	lines := strings.Split(diff, "\n")

	newLine := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			// Parse hunk header: @@ -old,count +new,count @@
			newLine = parseHunkNewStart(line)
			continue
		}

		if newLine == 0 {
			continue
		}

		if strings.HasPrefix(line, "+") {
			result = append(result, DiffLine{
				Type: "add",
				Line: newLine,
				Text: line[1:],
			})
			newLine++
		} else if strings.HasPrefix(line, "-") {
			result = append(result, DiffLine{
				Type: "del",
				Line: 0, // deleted lines don't have a new file line number
				Text: line[1:],
			})
		} else if len(line) > 0 {
			// context line (starts with space or is just content)
			text := line
			if line[0] == ' ' {
				text = line[1:]
			}
			result = append(result, DiffLine{
				Type: "context",
				Line: newLine,
				Text: text,
			})
			newLine++
		}
	}

	return result
}

// parseHunkNewStart extracts the new file start line from a @@ header.
func parseHunkNewStart(hunk string) int {
	// Format: @@ -old_start,old_count +new_start,new_count @@
	idx := strings.Index(hunk, "+")
	if idx < 0 {
		return 0
	}
	rest := hunk[idx+1:]
	// Find the comma or space after the number
	end := 0
	for i, c := range rest {
		if c == ',' || c == ' ' || c == '@' {
			end = i
			break
		}
	}
	if end == 0 {
		end = len(rest)
	}
	n := 0
	for _, c := range rest[:end] {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n == 0 {
		return 1
	}
	return n
}

// FormatDiffContextForDisplay returns a human-readable string of the diff context.
func FormatDiffContextForDisplay(contextJSON string) string {
	if contextJSON == "" {
		return ""
	}
	var lines []DiffLine
	if err := json.Unmarshal([]byte(contextJSON), &lines); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, dl := range lines {
		switch dl.Type {
		case "add":
			fmt.Fprintf(&sb, "+%4d | %s\n", dl.Line, dl.Text)
		case "del":
			fmt.Fprintf(&sb, "-     | %s\n", dl.Text)
		default:
			fmt.Fprintf(&sb, " %4d | %s\n", dl.Line, dl.Text)
		}
	}
	return sb.String()
}
