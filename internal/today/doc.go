// Package today manages the daily focus task list via flicktask scheduled dates.
//
// It queries flicktask for pending tasks with a scheduled date of today or earlier,
// renders them in a lipgloss table sorted by scheduled date, and provides Add/Remove
// helpers that set or clear the scheduled field. Also exposes a Completed view for
// tasks finished today.
//
// Plane: shared
package today
