// Package skill provides disk-based skill access for agents.
//
// Skills are markdown files deployed to ~/.agents/skills/ by `ttal sync`.
// Each file has YAML frontmatter (name, description, category) and a body.
// Agents fetch skill content via `ttal skill get <name>`.
//
// Plane: shared
package skill
