package ask

import "github.com/tta-lab/logos"

// Command documentation for Organon CLI tools exposed in logos agent loops.
// These are passed to logos.BuildSystemPrompt via PromptData.Commands.

// FetchCommandDoc documents the web fetch command (fetch a web page as markdown).
var FetchCommandDoc = logos.CommandDoc{
	Name:    "web fetch",
	Summary: "Fetch and read a web page as markdown",
	Help: `Flags:
  --tree           Show heading tree with section IDs
  -s ID            Read section by ID (2-char base62)
  --full           Full content, skip auto-tree
  --tree-threshold Auto-tree above this char count (default: 5000)

Long pages (>5000 chars) auto-show a heading tree. Use -s to read specific sections.

Examples:
  web fetch https://docs.example.com/api
  web fetch https://docs.example.com/api -s cD
  web fetch https://example.com --full`,
}

// WebCommandDoc documents the web search command (search the web).
var WebCommandDoc = logos.CommandDoc{
	Name:    "web search",
	Summary: "Search the web",
	Help: `Flags:
  -n N / --max N   Maximum results (default 10, max 20)

Uses Brave Search API when BRAVE_API_KEY is set, falls back to DuckDuckGo.

Examples:
  web search "golang context timeout patterns"
  web search "RFC 7231 HTTP semantics" -n 5`,
}

// RGCommandDoc documents the rg command (search file contents).
var RGCommandDoc = logos.CommandDoc{
	Name:    "rg",
	Summary: "Search file contents (ripgrep)",
	Help: `Common flags:
  --glob "*.go"   Filter by file pattern
  -C 3            Show 3 lines of context
  -i              Case insensitive
  --type go       Filter by language
  -l              List matching files only

List files:
  rg --files [path] --glob "*.ts" --sort modified

rg respects .gitignore by default. Fast, recursive.`,
}

// SrcCommandDoc documents the src command (symbol-aware source reader).
var SrcCommandDoc = logos.CommandDoc{
	Name:    "src",
	Summary: "Symbol-aware source reader and editor",
	Help: `Usage:
  src <file>              Symbol tree (depth 2)
  src <file> --depth 3    Deeper tree
  src <file> -s <id>      Read symbol by ID (from tree output)
  src <file> --tree       Force tree view

Markdown files (.md, .markdown, .mdx) use heading-based sections.

Prefer src over cat/sed for reading code — it shows structure first,
then lets you zoom into specific symbols without reading the whole file.

Examples:
  src internal/router/router.go
  src internal/router/router.go -s aB
  src docs/architecture.md --tree`,
}

// NetworkCommands returns CommandDocs for network-only modes (--url, --web).
func NetworkCommands() []logos.CommandDoc {
	return []logos.CommandDoc{FetchCommandDoc, WebCommandDoc}
}

// AllCommands returns CommandDocs for modes with both filesystem and network access.
func AllCommands() []logos.CommandDoc {
	return append(NetworkCommands(), RGCommandDoc, SrcCommandDoc)
}
