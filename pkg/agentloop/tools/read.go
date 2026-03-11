package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
)

const (
	maxFileBytes     = 5 * 1024 * 1024 // 5MB
	defaultReadLines = 2000
	maxLineChars     = 2000
)

// ReadParams are the input parameters for the read tool.
type ReadParams struct {
	FilePath string `json:"file_path" description:"Absolute path to the file to read"`
	Offset   int    `json:"offset,omitempty" description:"Line number to start reading from (0-based)"`
	Limit    int    `json:"limit,omitempty" description:"Number of lines to read (default 2000)"`
}

// NewReadTool creates a file-reading tool restricted to the given allowed paths.
// Access to files outside allowedPaths is denied.
func NewReadTool(allowedPaths []string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		"read",
		schemaDescription(readDescription),
		func(ctx context.Context, params ReadParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if !isPathAllowed(params.FilePath, allowedPaths) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: access denied: %q is not within an allowed directory", params.FilePath)), nil //nolint:lll
			}

			info, err := os.Stat(params.FilePath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
			}
			if info.IsDir() {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %q is a directory, not a file", params.FilePath)), nil
			}
			if info.Size() > maxFileBytes {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: file too large (%d bytes, max %d bytes)", info.Size(), maxFileBytes)), nil //nolint:lll
			}

			limit := params.Limit
			if limit <= 0 {
				limit = defaultReadLines
			}

			content, err := readTextFile(params.FilePath, params.Offset, limit)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
			}

			return fantasy.NewTextResponse(addLineNumbers(content, params.Offset+1)), nil
		},
	)
}

// isPathAllowed checks whether filePath is within any of the allowed directories.
// Resolves symlinks and prevents traversal attacks.
func isPathAllowed(filePath string, allowedPaths []string) bool {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	// EvalSymlinks may fail if the file doesn't exist yet; fall back to absPath.
	evalPath := absPath
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		evalPath = resolved
	}

	for _, allowed := range allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		evalAllowed := absAllowed
		if resolved, err := filepath.EvalSymlinks(absAllowed); err == nil {
			evalAllowed = resolved
		}
		rel, err := filepath.Rel(evalAllowed, evalPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

// readTextFile reads lines [offset, offset+limit) from the file.
// offset is 0-based. Returns the raw lines (without line numbers).
func readTextFile(path string, offset, limit int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var lines []string
	lineNum := 0
	for scanner.Scan() {
		if lineNum >= offset && len(lines) < limit {
			line := scanner.Text()
			// Truncate extremely long lines.
			if len([]rune(line)) > maxLineChars {
				line = string([]rune(line)[:maxLineChars]) + " [line truncated]"
			}
			lines = append(lines, line)
		}
		lineNum++
		if len(lines) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		if err == bufio.ErrTooLong {
			return lines, fmt.Errorf("file contains a line exceeding 64KB; use the bash tool with awk/cut to read it")
		}
		return lines, err
	}
	return lines, nil
}

// addLineNumbers formats lines with 1-based line numbers starting at startLine.
// Format: "     1\tline content"
func addLineNumbers(lines []string, startLine int) string {
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%6d\t%s\n", startLine+i, line)
	}
	return sb.String()
}
