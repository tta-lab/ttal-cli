package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
)

const (
	maxBodyBytes    = 1024 * 1024 // 1MB
	maxContentChars = 30_000
	webFetchAgent   = "ttal-agentloop/1.0"
)

// ReadURLParams are the input parameters for the read_url tool.
type ReadURLParams struct {
	URL     string `json:"url" description:"The URL to fetch content from"`
	Tree    bool   `json:"tree,omitempty" description:"Force tree view regardless of content size"`
	Section string `json:"section,omitempty" description:"Section ID to extract (use tree view first to see IDs)"`
	Full    bool   `json:"full,omitempty" description:"Force full content (truncated at 30k chars)"`
}

// ReadURLBackend controls how HTML is fetched and converted to markdown.
type ReadURLBackend interface {
	Fetch(ctx context.Context, url string) (content string, err error)
}

// cachedPage holds a parsed page with its markdown and headings.
// The cache has no TTL — pages are cached for the lifetime of the agent loop.
type cachedPage struct {
	markdown string
	headings []mdHeading
}

// pageCache is an in-memory cache of fetched pages.
type pageCache struct {
	mu    sync.RWMutex
	pages map[string]*cachedPage
}

func (c *pageCache) get(url string) (*cachedPage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.pages[url]
	return p, ok
}

func (c *pageCache) set(url string, page *cachedPage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pages[url] = page
}

// NewReadURLTool creates a URL fetch tool using the provided backend.
// backend controls how HTML is fetched and converted:
//   - BrowserGatewayBackend: POST to browser-gateway /api/extract (for server use)
//   - DefuddleCLIBackend: shell out to `defuddle parse <url> --markdown` (for local/CLI use)
func NewReadURLTool(backend ReadURLBackend, treeThreshold int) fantasy.AgentTool {
	if treeThreshold <= 0 {
		treeThreshold = 5000
	}
	cache := &pageCache{pages: make(map[string]*cachedPage)}

	return fantasy.NewParallelAgentTool(
		"read_url",
		schemaDescription(readURLDescription),
		func(ctx context.Context, params ReadURLParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			page, err := fetchOrCachePage(ctx, params.URL, backend, cache)
			if err != nil {
				slog.Warn("read_url: fetch failed", "url", params.URL, "error", err)
				return fantasy.NewTextErrorResponse(fmt.Sprintf("fetch error: %v", err)), nil
			}

			source := []byte(page.markdown)

			// Section extraction mode.
			if params.Section != "" {
				section, err := extractSection(source, page.headings, params.Section)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
				}
				return fantasy.NewTextResponse(section), nil
			}

			charCount := utf8.RuneCountInString(page.markdown)

			// Tree mode (explicit or auto for large content).
			if params.Tree || (!params.Full && charCount > treeThreshold) {
				if len(page.headings) == 0 {
					// No headings — return full content.
					slog.Warn("read_url: no headings found, returning full content", "url", params.URL)
					return fantasy.NewTextResponse(truncateContent(page.markdown)), nil
				}
				return fantasy.NewTextResponse(renderTree(page.headings, source)), nil
			}

			// Full mode (explicit or auto for small content).
			return fantasy.NewTextResponse(truncateContent(page.markdown)), nil
		},
	)
}

// fetchOrCachePage fetches a URL and caches the result.
func fetchOrCachePage(ctx context.Context, url string, backend ReadURLBackend, cache *pageCache) (*cachedPage, error) {
	if page, ok := cache.get(url); ok {
		return page, nil
	}
	markdown, err := backend.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	source := []byte(markdown)
	headings := parseHeadings(source)
	assignIDs(headings)
	page := &cachedPage{
		markdown: markdown,
		headings: headings,
	}
	cache.set(url, page)
	return page, nil
}

// --- BrowserGatewayBackend ---

type extractResponse struct {
	Content     string `json:"content"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Description string `json:"description"`
	WordCount   int    `json:"wordCount"`
}

// browserGatewayBackend fetches via a browser-gateway /api/extract endpoint.
type browserGatewayBackend struct {
	gatewayURL string
	client     *http.Client
}

// NewBrowserGatewayBackend creates a backend that fetches via browser-gateway.
func NewBrowserGatewayBackend(gatewayURL string, client *http.Client) ReadURLBackend {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &browserGatewayBackend{gatewayURL: gatewayURL, client: client}
}

func (b *browserGatewayBackend) Fetch(ctx context.Context, url string) (string, error) {
	body, err := json.Marshal(map[string]string{"url": url})
	if err != nil {
		return "", fmt.Errorf("browser-gateway: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.gatewayURL+"/api/extract", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("browser-gateway: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", webFetchAgent)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("browser-gateway: fetch: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		slog.Warn("browser-gateway returned error status", "url", url, "status", resp.StatusCode)
		return "", fmt.Errorf("browser-gateway: HTTP %d", resp.StatusCode)
	}

	var extracted extractResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&extracted); err != nil {
		return "", fmt.Errorf("browser-gateway: decode response: %w", err)
	}

	if extracted.Content == "" {
		return "", fmt.Errorf("browser-gateway: empty content for %s", url)
	}

	var sb strings.Builder
	if extracted.Title != "" {
		sb.WriteString("# ")
		sb.WriteString(extracted.Title)
		sb.WriteString("\n\n")
	}
	if extracted.Author != "" {
		sb.WriteString("*By ")
		sb.WriteString(extracted.Author)
		sb.WriteString("*\n\n")
	}
	sb.WriteString(extracted.Content)

	return truncateContent(sb.String()), nil
}

// --- DefuddleCLIBackend ---

// defuddleCLIBackend shells out to `defuddle parse <url> --markdown`.
type defuddleCLIBackend struct{}

// NewDefuddleCLIBackend creates a backend that shells out to the defuddle CLI.
// Requires defuddle to be installed and on PATH.
func NewDefuddleCLIBackend() ReadURLBackend {
	return &defuddleCLIBackend{}
}

func (b *defuddleCLIBackend) Fetch(ctx context.Context, url string) (string, error) {
	cmd := exec.CommandContext(ctx, "defuddle", "parse", url, "--markdown")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("defuddle parse failed: %w\noutput: %s", err, strings.TrimSpace(string(out)))
	}
	return truncateContent(string(out)), nil
}

func truncateContent(s string) string {
	if utf8.RuneCountInString(s) <= maxContentChars {
		return s
	}
	return string([]rune(s)[:maxContentChars]) + "\n[content truncated at 30,000 characters]"
}
