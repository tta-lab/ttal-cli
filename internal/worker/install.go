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

// hookShim generates a bash hook script that delegates to ttal.
// For non-default teams, it bakes in "export TTAL_TEAM=<team>" so hooks
// work correctly even when triggered from tools without TTAL_TEAM in env
// (e.g. taskwarrior-tui).
func hookShim(hookCmd, teamName string) string {
	var envLine string
	if teamName != "" && teamName != config.DefaultTeamName {
		envLine = fmt.Sprintf("\nexport TTAL_TEAM=%q", teamName)
	}
	return fmt.Sprintf(`#!/bin/bash
# Taskwarrior hook — delegates to ttal.
# Installed by: ttal worker install
%s
exec ttal worker hook %s
`, envLine, hookCmd)
}

// Install sets up the taskwarrior hooks (on-add and on-modify).
func Install() error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH — install with: make install")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	hookDir := filepath.Join(cfg.TaskData(), "hooks")
	teamName := cfg.TeamName()

	fmt.Printf("Using ttal binary: %s\n", ttalBin)
	if teamName != config.DefaultTeamName {
		fmt.Printf("Team: %s\n", teamName)
	}
	fmt.Println()

	if err := installHooks(hookDir, teamName); err != nil {
		return fmt.Errorf("hook install failed: %w", err)
	}

	return nil
}

// Uninstall removes the taskwarrior hooks.
func Uninstall() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	hookDir := filepath.Join(cfg.TaskData(), "hooks")

	for _, name := range []string{onModifyHookName, onAddHookName} {
		hookPath := filepath.Join(hookDir, name)
		if _, err := os.Stat(hookPath); err == nil {
			if err := os.Remove(hookPath); err != nil {
				return fmt.Errorf("failed to remove %s: %w", name, err)
			}
			fmt.Printf("Removed taskwarrior hook: %s\n", hookPath)
		} else {
			fmt.Printf("Taskwarrior hook %s: not installed\n", name)
		}
	}

	fmt.Println("\nNote: Log files remain at data dir and hooks.log")
	fmt.Println("  To also remove the daemon: ttal daemon uninstall")
	return nil
}

func installHooks(hookDir, teamName string) error {
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
	if err := os.WriteFile(onModifyPath, []byte(hookShim("on-modify", teamName)), 0o755); err != nil {
		return err
	}
	fmt.Printf("Taskwarrior hook: %s\n", onModifyPath)

	// Install on-add hook
	onAddPath := filepath.Join(hookDir, onAddHookName)
	if err := os.WriteFile(onAddPath, []byte(hookShim("on-add", teamName)), 0o755); err != nil {
		return err
	}
	fmt.Printf("Taskwarrior hook: %s\n", onAddPath)

	return nil
}
