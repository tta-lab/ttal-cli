package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	hookName = "on-modify-ttal"
)

const hookShim = `#!/bin/bash
# Taskwarrior on-modify hook — delegates to ttal.
# Installed by: ttal worker install

exec ttal worker hook on-modify
`

// Install sets up the taskwarrior hook.
// Worker completion polling is now handled by the ttal daemon (ttal daemon install).
func Install() error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH — install with: make install")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	fmt.Printf("Using ttal binary: %s\n\n", ttalBin)

	if err := installHook(home); err != nil {
		return fmt.Errorf("hook install failed: %w", err)
	}

	fmt.Println("\nNote: Worker completion polling is now handled by the ttal daemon.")
	fmt.Println("  Run: ttal daemon install")

	return nil
}

// Uninstall removes the taskwarrior hook.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	hookPath := filepath.Join(home, ".task", "hooks", hookName)
	if _, err := os.Stat(hookPath); err == nil {
		os.Remove(hookPath) //nolint:errcheck
		fmt.Printf("Removed taskwarrior hook: %s\n", hookPath)
	} else {
		fmt.Println("Taskwarrior hook: not installed")
	}

	fmt.Println("\nNote: Log files remain at ~/.ttal/ and ~/.task/hooks.log")
	fmt.Println("  To also remove the daemon: ttal daemon uninstall")
	return nil
}

func installHook(home string) error {
	hookDir := filepath.Join(home, ".task", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hookDir, hookName)

	// Backup existing Python hook if present
	pythonHook := filepath.Join(hookDir, "on-modify-worker-lifecycle")
	if _, err := os.Stat(pythonHook); err == nil {
		backupPath := pythonHook + ".bak"
		if err := os.Rename(pythonHook, backupPath); err != nil {
			return fmt.Errorf("failed to backup Python hook: %w", err)
		}
		fmt.Printf("Backed up Python hook: %s\n", backupPath)
	}

	if err := os.WriteFile(hookPath, []byte(hookShim), 0o755); err != nil {
		return err
	}

	fmt.Printf("Taskwarrior hook: %s\n", hookPath)
	return nil
}
