// Package onboard implements the guided first-run setup flow for ttal.
//
// It installs prerequisites (tmux, flicktask, zellij, ffmpeg) via Homebrew,
// applies a workspace scaffold, installs the launchd daemon plist, registers
// flicktask hooks, and runs ttal doctor to verify the result.
//
// Plane: shared
package onboard
