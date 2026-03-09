// Package agentfs discovers and reads agent metadata from the filesystem.
//
// Agents are represented as subdirectories under a team path, each containing
// a CLAUDE.md file with YAML frontmatter fields (name, role, voice, emoji, etc.).
// Provides functions to discover all agents, look up a single agent by name or
// path, filter by role, and update individual frontmatter fields in place.
//
// Plane: shared
package agentfs
