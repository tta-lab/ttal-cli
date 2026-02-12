package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const hookScript = `#!/bin/bash
# Taskwarrior on-modify hook — delegates to ttal.
# Installed by: ttal worker hook install
#
# Passes stdin (two JSON lines) to ttal, which detects the event type
# and handles start/complete transitions. Always exits 0.

exec ttal worker hook on-modify
`

// HookInstall installs the taskwarrior on-modify hook.
func HookInstall() error {
	// Verify ttal is in PATH
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH — install with: make install")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	hookDir := filepath.Join(home, ".task", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hookDir, "on-modify-ttal")

	// Check for existing Python hook
	pythonHook := filepath.Join(hookDir, "on-modify-worker-lifecycle")
	if _, err := os.Stat(pythonHook); err == nil {
		backupPath := pythonHook + ".bak"
		if err := os.Rename(pythonHook, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing Python hook: %w", err)
		}
		fmt.Printf("Backed up existing Python hook:\n  %s → %s\n\n", pythonHook, backupPath)
	}

	if err := os.WriteFile(hookPath, []byte(hookScript), 0o755); err != nil {
		return fmt.Errorf("failed to write hook script: %w", err)
	}

	fmt.Printf("Installed taskwarrior hook:\n  %s\n\n", hookPath)
	fmt.Printf("Using ttal binary:\n  %s\n\n", ttalBin)
	fmt.Println("Events handled:")
	fmt.Println("  Task start (+ACTIVE) → ttal worker hook on-start")
	fmt.Println("  Task complete         → ttal worker hook on-complete")

	return nil
}
