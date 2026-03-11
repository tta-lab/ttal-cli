package tools

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"charm.land/fantasy"
)

// GrepParams are the input parameters for the grep tool.
type GrepParams struct {
	Pattern string `json:"pattern" description:"Regex pattern to search for"`
	Path    string `json:"path,omitempty" description:"File or directory to search in (must be within allowed paths; empty = search all allowed paths)"` //nolint:lll
	Glob    string `json:"glob,omitempty" description:"File glob filter (e.g. '*.go')"`
}

// NewGrepTool creates a content search tool restricted to the given allowed paths.
func NewGrepTool(allowedPaths []string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		"grep",
		"Search file contents using a regex pattern within allowed project directories. Returns matching lines with file paths and line numbers. Output capped at 30,000 characters.", //nolint:lll
		func(ctx context.Context, params GrepParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			re, err := regexp.Compile(params.Pattern)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: invalid regex pattern: %v", err)), nil
			}

			searchDirs, err := resolveGrepSearchDirs(params.Path, allowedPaths)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
			}

			var sb strings.Builder
			for _, dir := range searchDirs {
				if err := grepDir(ctx, re, dir, params.Glob, &sb); err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
				}
				if len([]rune(sb.String())) >= maxContentChars {
					break
				}
			}

			result := sb.String()
			if result == "" {
				return fantasy.NewTextResponse("No matches found."), nil
			}
			return fantasy.NewTextResponse(truncateContent(result)), nil
		},
	)
}

// resolveGrepSearchDirs resolves search directories for grep.
// If path is a file, validates and returns [path] directly.
// If path is a directory, validates and returns [path].
// If path is empty, returns all allowedPaths.
func resolveGrepSearchDirs(path string, allowedPaths []string) ([]string, error) {
	if path == "" {
		return allowedPaths, nil
	}
	if !isPathAllowed(path, allowedPaths) {
		return nil, fmt.Errorf("access denied: %q is not within an allowed directory", path)
	}
	return []string{path}, nil
}

// grepDir walks a path (file or directory) and writes matching lines to sb.
func grepDir(ctx context.Context, re *regexp.Regexp, root, globPattern string, sb *strings.Builder) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat %q: %v", root, err)
	}

	// If root is a single file, search it directly.
	if !info.IsDir() {
		return grepFile(ctx, re, root, sb)
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			// Skip hidden directories.
			if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply glob filter if provided.
		if globPattern != "" {
			matched, err := filepath.Match(globPattern, d.Name())
			if err != nil || !matched {
				return nil
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if len([]rune(sb.String())) >= maxContentChars {
			return filepath.SkipAll
		}

		return grepFile(ctx, re, path, sb)
	})
}

// grepFile searches a single file and writes matching lines to sb.
func grepFile(ctx context.Context, re *regexp.Regexp, path string, sb *strings.Builder) error {
	f, err := os.Open(path)
	if err != nil {
		return nil // skip files we can't open
	}
	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		if re.MatchString(line) {
			fmt.Fprintf(sb, "%s:%d: %s\n", path, lineNum, line)
		}
	}
	return scanner.Err()
}
