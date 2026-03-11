// Package tools provides reusable agent tool implementations.
//
// Includes bash (sandboxed shell execution), web_fetch (HTML-to-markdown with
// pluggable WebFetchBackend), and web_search (DuckDuckGo Lite). Tools are
// constructed with NewBashTool, NewWebFetchTool, and NewWebSearchTool, or
// bundled via NewDefaultToolSet.
//
// Plane: shared
package tools
