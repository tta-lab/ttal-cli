// Package skill provides a runtime-agnostic skill registry backed by flicknote.
//
// Skills are markdown documents (commands, methodologies, reference sheets)
// stored in flicknote and accessed by agents via `ttal skill get <name>`.
// The registry maps human-readable names to flicknote hex IDs and provides
// per-agent filtering.
//
// Plane: shared
package skill
