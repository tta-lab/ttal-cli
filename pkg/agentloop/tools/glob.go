package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/bmatcuk/doublestar/v4"
)

const maxGlobResults = 200

// GlobParams are the input parameters for the glob tool.
type GlobParams struct {
	Pattern string `json:"pattern" description:"Glob pattern to match files (e.g. '**/*.go', 'src/**/*.ts')"`                                    //nolint:lll
	Path    string `json:"path,omitempty" description:"Directory to search in (must be within allowed paths; empty = search all allowed paths)"` //nolint:lll
}

// NewGlobTool creates a glob tool restricted to the given allowed paths.
func NewGlobTool(allowedPaths []string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		"glob",
		schemaDescription(globDescription),
		func(ctx context.Context, params GlobParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			searchDirs, err := resolveSearchPaths(params.Path, allowedPaths, true)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
			}

			type fileEntry struct {
				path    string
				modTime int64
			}
			var entries []fileEntry

			for _, searchDir := range searchDirs {
				fsys := os.DirFS(searchDir)
				matches, err := doublestar.Glob(fsys, params.Pattern)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: invalid pattern %q: %v", params.Pattern, err)), nil
				}
				for _, match := range matches {
					absMatch := filepath.Join(searchDir, match)
					info, err := os.Stat(absMatch)
					if err != nil || info.IsDir() {
						continue
					}
					entries = append(entries, fileEntry{path: absMatch, modTime: info.ModTime().UnixNano()})
				}
			}

			// Sort by modification time, most recent first.
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].modTime > entries[j].modTime
			})

			// Cap results.
			if len(entries) > maxGlobResults {
				entries = entries[:maxGlobResults]
			}

			if len(entries) == 0 {
				return fantasy.NewTextResponse("No files matched."), nil
			}

			var sb strings.Builder
			for _, e := range entries {
				sb.WriteString(e.path)
				sb.WriteByte('\n')
			}
			return fantasy.NewTextResponse(strings.TrimRight(sb.String(), "\n")), nil
		},
	)
}
