// Package agentfs discovers and reads agent metadata from the filesystem.
//
// Agents are represented as workspace subdirectories containing an AGENTS.md file
// under a team path (e.g., templates/ttal/yuki/AGENTS.md). The directory name is
// the agent name. Metadata is parsed from YAML frontmatter in AGENTS.md (name, role,
// voice, emoji, color, etc.). Provides functions to discover all agents, look up a
// single agent by name or path, filter by role, and update individual frontmatter
// fields in place.
//
// Plane: shared
package agentfs
