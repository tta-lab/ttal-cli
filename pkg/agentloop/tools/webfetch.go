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
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

const (
	maxBodyBytes    = 1024 * 1024 // 1MB
	maxContentChars = 30_000
	webFetchAgent   = "ttal-agentloop/1.0"
)

// WebFetchParams are the input parameters for the web_fetch tool.
type WebFetchParams struct {
	URL string `json:"url" description:"The URL to fetch content from"`
}

// WebFetchBackend controls how HTML is fetched and converted to markdown.
type WebFetchBackend interface {
	Fetch(ctx context.Context, url string) (content string, err error)
}

// NewWebFetchTool creates a web fetch tool using the provided backend.
// backend controls how HTML is fetched and converted:
//   - BrowserGatewayBackend: POST to browser-gateway /api/extract (for server use)
//   - DefuddleCLIBackend: shell out to `defuddle parse <url> --markdown` (for local/CLI use)
//   - DirectFetchBackend: plain HTTP + html-to-markdown (fallback)
func NewWebFetchTool(backend WebFetchBackend) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		"web_fetch",
		"Fetch a URL and return its content as text. HTML is converted to markdown.",
		func(ctx context.Context, params WebFetchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			content, err := backend.Fetch(ctx, params.URL)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("fetch error: %v", err)), nil
			}
			return fantasy.NewTextResponse(content), nil
		},
	)
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
// Falls back to direct HTTP fetch on any gateway error.
func NewBrowserGatewayBackend(gatewayURL string, client *http.Client) WebFetchBackend {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &browserGatewayBackend{gatewayURL: gatewayURL, client: client}
}

func (b *browserGatewayBackend) Fetch(ctx context.Context, url string) (string, error) {
	body, err := json.Marshal(map[string]string{"url": url})
	if err != nil {
		slog.Warn("browser-gateway marshal failed, falling back to direct fetch", "url", url, "error", err)
		return fallbackDirect(ctx, b.client, url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.gatewayURL+"/api/extract", bytes.NewReader(body))
	if err != nil {
		slog.Warn("browser-gateway request build failed, falling back to direct fetch", "url", url, "error", err)
		return fallbackDirect(ctx, b.client, url)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", webFetchAgent)

	resp, err := b.client.Do(req)
	if err != nil {
		slog.Warn("browser-gateway fetch failed, falling back to direct fetch", "url", url, "error", err)
		return fallbackDirect(ctx, b.client, url)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		slog.Warn("browser-gateway returned error, falling back to direct fetch", "url", url, "status", resp.StatusCode)
		return fallbackDirect(ctx, b.client, url)
	}

	var extracted extractResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&extracted); err != nil {
		slog.Warn("browser-gateway decode failed, falling back to direct fetch", "url", url, "error", err)
		return fallbackDirect(ctx, b.client, url)
	}

	if extracted.Content == "" {
		slog.Warn("browser-gateway returned empty content, falling back to direct fetch", "url", url)
		return fallbackDirect(ctx, b.client, url)
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

// fallbackDirect checks ctx before delegating to directFetch so cancelled
// contexts don't trigger a second network call.
func fallbackDirect(ctx context.Context, client *http.Client, url string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return directFetch(ctx, client, url)
}

// --- DefuddleCLIBackend ---

// defuddleCLIBackend shells out to `defuddle parse <url> --markdown`.
type defuddleCLIBackend struct{}

// NewDefuddleCLIBackend creates a backend that shells out to the defuddle CLI.
// Requires defuddle to be installed and on PATH.
func NewDefuddleCLIBackend() WebFetchBackend {
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

// --- DirectFetchBackend ---

// directFetchBackend fetches via plain HTTP and converts HTML to markdown.
type directFetchBackend struct {
	client *http.Client
}

// NewDirectFetchBackend creates a backend that fetches via plain HTTP.
// HTML responses are converted to markdown using html-to-markdown.
func NewDirectFetchBackend(client *http.Client) WebFetchBackend {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &directFetchBackend{client: client}
}

func (b *directFetchBackend) Fetch(ctx context.Context, url string) (string, error) {
	return directFetch(ctx, b.client, url)
}

// directFetch is the shared plain-HTTP implementation used by both
// directFetchBackend and browserGatewayBackend's fallback path.
func directFetch(ctx context.Context, client *http.Client, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", webFetchAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch error: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		markdown, err := htmltomarkdown.ConvertString(string(body))
		if err != nil {
			// Return content with a warning prepended so the LLM knows it received raw HTML.
			slog.Warn("html-to-markdown conversion failed, returning raw HTML", "url", targetURL, "error", err)
			return truncateContent("[html-to-markdown conversion failed; raw HTML follows]\n\n" + string(body)), nil
		}
		return truncateContent(markdown), nil
	}

	return truncateContent(string(body)), nil
}

func truncateContent(s string) string {
	if utf8.RuneCountInString(s) <= maxContentChars {
		return s
	}
	return string([]rune(s)[:maxContentChars]) + "\n[content truncated at 30,000 characters]"
}
