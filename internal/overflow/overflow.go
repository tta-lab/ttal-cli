package overflow

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultThreshold is the message body size at which overflow kicks in.
const DefaultThreshold = 8 * 1024 // 8KB

// OverflowDir is the directory where overflow files are written.
const OverflowDir = "~/.ttal/overflow"

// Write saves body to an overflow file and returns a truncated preview
// with a reference path. Returns the original body unchanged when len(body) <= threshold.
func Write(body string, threshold int) string {
	if len(body) <= threshold {
		return body
	}

	dir := mustExpandDir(OverflowDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return truncate(body)
	}

	now := time.Now()
	path := filepath.Join(dir, fmt.Sprintf("overflow_%s.md", now.Format("20060102_150405")))
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return truncate(body)
	}

	return makePreview(body, path)
}

// makePreview builds a truncated preview with a reference path.
// The preview is at most 200 characters of the original body, followed by
// a reference line pointing to the full content.
func makePreview(body, path string) string {
	preview := truncate(body)
	return fmt.Sprintf("%s\n\n[message truncated — full content at %s]", preview, path)
}

// truncate returns the first 200 characters of body.
func truncate(body string) string {
	if len(body) <= 200 {
		return body
	}
	return body[:200]
}

// mustExpandDir expands ~ to the user's home directory.
func mustExpandDir(dir string) string {
	if len(dir) == 0 || dir[0] != '~' {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	return filepath.Join(home, dir[1:])
}
