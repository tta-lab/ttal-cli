// Package agentfs discovers and reads agent metadata from the filesystem.
//
// Agents are represented as flat .md files under a team path (e.g., yuki.md)
// with YAML frontmatter fields (name, role, voice, emoji, etc.).
// Provides functions to discover all agents, look up a single agent by name or
// path, filter by role, and update individual frontmatter fields in place.
//
// Plane: shared
package agentfs
