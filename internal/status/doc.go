// Package status reads and writes live agent session status files.
//
// Status files are JSON documents stored under ~/.ttal/status/ with a
// team-prefixed filename ({team}-{agent}.json). Each file captures context
// usage percentage, model ID, session ID, and a timestamp so that the
// manager plane and CLI commands can display current agent health at a glance.
//
// Plane: shared
package status
