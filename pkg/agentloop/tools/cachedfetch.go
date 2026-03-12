package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CachedFetchBackend wraps a ReadURLBackend with a file-based daily cache.
// Cache key: sanitized URL + today's date. Cache dir: ~/.ttal/scrapes/.
// Cache TTL: date-in-filename (today's date = hit, anything else = miss).
type CachedFetchBackend struct {
	cacheDir string
	fallback ReadURLBackend
}

// NewCachedFetchBackend creates a CachedFetchBackend wrapping the given fallback.
// cacheDir is created if it doesn't exist (best-effort).
func NewCachedFetchBackend(cacheDir string, fallback ReadURLBackend) *CachedFetchBackend {
	_ = os.MkdirAll(cacheDir, 0o755) // best-effort
	return &CachedFetchBackend{cacheDir: cacheDir, fallback: fallback}
}

// Fetch returns cached content if available for today, otherwise delegates to fallback and caches.
func (b *CachedFetchBackend) Fetch(ctx context.Context, rawURL string) (string, error) {
	if cached, ok := b.readCache(rawURL); ok {
		return cached, nil
	}
	content, err := b.fallback.Fetch(ctx, rawURL)
	if err != nil {
		return "", err
	}
	_ = b.writeCache(rawURL, content) // best-effort
	return content, nil
}

func (b *CachedFetchBackend) readCache(rawURL string) (string, bool) {
	path := b.cachePath(rawURL)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (b *CachedFetchBackend) writeCache(rawURL, content string) error {
	return os.WriteFile(b.cachePath(rawURL), []byte(content), 0o644)
}

func (b *CachedFetchBackend) cachePath(rawURL string) string {
	sanitized := sanitizeURL(rawURL)
	date := time.Now().Format("2006-01-02")
	return filepath.Join(b.cacheDir, sanitized+"__"+date+".md")
}

// sanitizeURL converts a URL into a safe filename segment.
// Replaces "://" → "___" and "/" → "_".
// If query params are present, appends "_q" + first 8 chars of SHA256 of the query string.
func sanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Fallback: just replace unsafe chars
		r := strings.NewReplacer("://", "___", "/", "_", "?", "_", "=", "_", "&", "_")
		return r.Replace(rawURL)
	}

	base := strings.ReplaceAll(rawURL, "://", "___")
	// Strip query from base before replacing /
	if u.RawQuery != "" {
		withoutQuery := strings.SplitN(base, "?", 2)[0]
		base = withoutQuery
	}
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.TrimSuffix(base, "_") // strip trailing underscore

	if u.RawQuery != "" {
		h := sha256.Sum256([]byte(u.RawQuery))
		base += "_q" + fmt.Sprintf("%x", h[:4]) // first 8 hex chars (4 bytes)
	}
	return base
}
