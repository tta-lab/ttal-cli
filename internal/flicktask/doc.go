// Package flicktask provides shared helpers for interacting with flicktask.
//
// It defines the Task struct with worker UDAs (branch, pr_id, spawner),
// ID validation and slug generation for session naming, task export
// and query helpers, and flicknote annotation resolution.
// Used by CLI commands, the daemon, and worker hooks throughout both planes.
//
// Plane: shared
package flicktask
