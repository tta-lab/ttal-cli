package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

const (
	onModifyHookName = "on-modify-ttal"
	onAddHookName    = "on-add-ttal"
)

const onModifyHookShim = `#!/bin/bash
# Taskwarrior on-modify hook — delegates to ttal.
# Installed by: ttal worker install

exec ttal worker hook on-modify
`

const onAddHookShim = `#!/bin/bash
# Taskwarrior on-add hook — delegates to ttal.
# Installed by: ttal worker install

exec ttal worker hook on-add
`

// Install sets up the taskwarrior hooks (on-add and on-modify).
func Install() error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH — install with: make install")
	}

	hookDir, err := taskHookDir()
	if err != nil {
		return err
	}

	fmt.Printf("Using ttal binary: %s\n\n", ttalBin)

	if err := installHooks(hookDir); err != nil {
		return fmt.Errorf("hook install failed: %w", err)
	}

	return nil
}

// Uninstall removes the taskwarrior hooks.
func Uninstall() error {
	hookDir, err := taskHookDir()
	if err != nil {
		return err
	}

	for _, name := range []string{onModifyHookName, onAddHookName} {
		hookPath := filepath.Join(hookDir, name)
		if _, err := os.Stat(hookPath); err == nil {
			os.Remove(hookPath)
			fmt.Printf("Removed taskwarrior hook: %s\n", hookPath)
		} else {
			fmt.Printf("Taskwarrior hook %s: not installed\n", name)
		}
	}

	fmt.Println("\nNote: Log files remain at data dir and hooks.log")
	fmt.Println("  To also remove the daemon: ttal daemon uninstall")
	return nil
}

func installHooks(hookDir string) error {
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Backup existing Python hook if present
	pythonHook := filepath.Join(hookDir, "on-modify-worker-lifecycle")
	if _, err := os.Stat(pythonHook); err == nil {
		backupPath := pythonHook + ".bak"
		if err := os.Rename(pythonHook, backupPath); err != nil {
			return fmt.Errorf("failed to backup Python hook: %w", err)
		}
		fmt.Printf("Backed up Python hook: %s\n", backupPath)
	}

	// Install on-modify hook
	onModifyPath := filepath.Join(hookDir, onModifyHookName)
	if err := os.WriteFile(onModifyPath, []byte(onModifyHookShim), 0o755); err != nil {
		return err
	}
	fmt.Printf("Taskwarrior hook: %s\n", onModifyPath)

	// Install on-add hook
	onAddPath := filepath.Join(hookDir, onAddHookName)
	if err := os.WriteFile(onAddPath, []byte(onAddHookShim), 0o755); err != nil {
		return err
	}
	fmt.Printf("Taskwarrior hook: %s\n", onAddPath)

	return nil
}

// taskHookDir resolves the taskwarrior hooks directory from config.
func taskHookDir() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		// Fallback: default taskwarrior location
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, ".task", "hooks"), nil
	}
	return filepath.Join(cfg.TaskData(), "hooks"), nil
}
