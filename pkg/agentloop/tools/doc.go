// Package tools provides reusable agent tool implementations.
//
// Includes bash (sandboxed shell execution), read_url (HTML-to-markdown with
// pluggable ReadURLBackend, tree/section/full modes), search_web (DuckDuckGo Lite),
// read (file reader with line numbers), read_md (markdown-aware reader with
// tree/section/full modes), glob (file pattern matching), and grep (content search).
// Tools are constructed with their respective New* constructors or bundled via
// NewDefaultToolSet. Filesystem tools (read, read_md, glob, grep) require
// allowedPaths to be configured.
//
// Plane: shared
package tools
