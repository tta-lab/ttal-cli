// Package onboard implements the guided first-run setup flow for ttal.
//
// It installs prerequisites (tmux, taskwarrior, zellij, ffmpeg) via Homebrew,
// applies a workspace scaffold, configures taskwarrior UDAs, installs the launchd
// daemon plist, registers taskwarrior hooks, and runs ttal doctor to verify the result.
//
// Plane: shared
package onboard
