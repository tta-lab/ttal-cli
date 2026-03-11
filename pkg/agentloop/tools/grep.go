package tools

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
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
		schemaDescription(grepDescription),
		func(ctx context.Context, params GrepParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			re, err := regexp.Compile(params.Pattern)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: invalid regex pattern: %v", err)), nil
			}

			// grep accepts both files and directories; pass mustBeDir=false.
			searchDirs, err := resolveSearchPaths(params.Path, allowedPaths, false)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
			}

			var sb strings.Builder
			var skipped int
			for _, dir := range searchDirs {
				s, err := grepDir(ctx, re, dir, params.Glob, &sb)
				skipped += s
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
				}
				if len([]rune(sb.String())) >= maxContentChars {
					break
				}
			}

			if skipped > 0 {
				fmt.Fprintf(&sb, "\n# Warning: skipped %d file(s) due to read errors (check logs for details)\n", skipped)
			}

			result := sb.String()
			if result == "" {
				return fantasy.NewTextResponse("No matches found."), nil
			}
			return fantasy.NewTextResponse(truncateContent(result)), nil
		},
	)
}

// resolveSearchPaths resolves search paths for glob and grep tools.
// If path is provided, validates it's within allowedPaths.
// When mustBeDir is true, also verifies path is a directory.
// If path is empty, returns all allowedPaths.
func resolveSearchPaths(path string, allowedPaths []string, mustBeDir bool) ([]string, error) {
	if path == "" {
		return allowedPaths, nil
	}
	if !isPathAllowed(path, allowedPaths) {
		return nil, fmt.Errorf("access denied: %q is not within an allowed directory", path)
	}
	if mustBeDir {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("path error: %v", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%q is not a directory", path)
		}
	}
	return []string{path}, nil
}

// grepDir walks a path (file or directory) and writes matching lines to sb.
// Returns the count of files skipped due to errors.
func grepDir(ctx context.Context, re *regexp.Regexp, root, globPattern string, sb *strings.Builder) (skipped int, err error) { //nolint:lll
	info, err := os.Stat(root)
	if err != nil {
		return 0, fmt.Errorf("stat %q: %v", root, err)
	}

	// If root is a single file, search it directly.
	if !info.IsDir() {
		skip, err := grepFile(ctx, re, root, sb)
		return skip, err
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("grep: skipping unreadable entry", "path", path, "error", err)
			skipped++
			return nil
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

		s, err := grepFile(ctx, re, path, sb)
		skipped += s
		return err
	})
	return skipped, walkErr
}

// grepFile searches a single file and writes matching lines to sb.
// Returns 1 if the file was skipped due to an error, 0 otherwise.
func grepFile(ctx context.Context, re *regexp.Regexp, path string, sb *strings.Builder) (skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		slog.Warn("grep: skipping unreadable file", "path", path, "error", err)
		return 1, nil
	}
	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		line := scanner.Text()
		if re.MatchString(line) {
			fmt.Fprintf(sb, "%s:%d: %s\n", path, lineNum, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scanning %q: %w", path, err)
	}
	return 0, nil
}
