// Package taskwarrior provides shared helpers for interacting with taskwarrior.
//
// It defines the Task struct with worker UDAs (branch, project_path, pr_id,
// spawner), UUID validation and slug generation for session naming, task export
// and query helpers, flicknote annotation resolution, and UDA verification.
// Used by CLI commands, the daemon, and worker hooks throughout both planes.
//
// Plane: shared
package taskwarrior
