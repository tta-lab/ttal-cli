package daemon

import "strings"

// encodeAgentPath encodes an agent path for use in filesystem paths.
// Slashes become hyphens and dots become hyphens.
func encodeAgentPath(path string) string {
	s := strings.ReplaceAll(path, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}
