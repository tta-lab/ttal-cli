package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
)

const (
	onModifyHookName = "on-modify-ttal"
	onAddHookName    = "on-add-ttal"
)

// flicktaskHookDir returns the flicktask hooks directory (~/.config/flicktask/hooks).
func flicktaskHookDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "flicktask", "hooks"), nil
}

// hookShim generates a bash hook script that delegates to ttal.
func hookShim(hookCmd, teamName string) string {
	var envLine string
	if teamName != "" {
		envLine = fmt.Sprintf("\nexport TTAL_TEAM=%q", teamName)
	}
	return fmt.Sprintf(`#!/bin/bash
# flicktask hook — delegates to ttal.
# Installed by: ttal doctor --fix
%s
exec ttal worker hook %s
`, envLine, hookCmd)
}

// Install sets up the flicktask hooks (on-add and on-modify).
func Install() error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH — install with: make install")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	hookDir, err := flicktaskHookDir()
	if err != nil {
		return err
	}
	teamName := cfg.TeamName()

	fmt.Printf("Using ttal binary: %s\n", ttalBin)
	if teamName != config.DefaultTeamName {
		fmt.Printf("Team: %s\n", teamName)
	}
	fmt.Println()

	if err := InstallHooks(hookDir, teamName); err != nil {
		return fmt.Errorf("hook install failed: %w", err)
	}

	return nil
}

// Uninstall removes the flicktask hooks.
func Uninstall() error {
	hookDir, err := flicktaskHookDir()
	if err != nil {
		return err
	}

	for _, name := range []string{onModifyHookName, onAddHookName} {
		hookPath := filepath.Join(hookDir, name)
		if _, err := os.Stat(hookPath); err == nil {
			if err := os.Remove(hookPath); err != nil {
				return fmt.Errorf("failed to remove %s: %w", name, err)
			}
			fmt.Printf("Removed flicktask hook: %s\n", hookPath)
		} else {
			fmt.Printf("flicktask hook %s: not installed\n", name)
		}
	}

	fmt.Println("\nNote: Log files remain at data dir and hooks.log")
	fmt.Println("  To also remove the daemon: ttal daemon uninstall")
	return nil
}

// InstallHooks writes flicktask hook scripts to the given directory.
func InstallHooks(hookDir, teamName string) error {
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Install on-modify hook
	onModifyPath := filepath.Join(hookDir, onModifyHookName)
	if err := os.WriteFile(onModifyPath, []byte(hookShim("on-modify", teamName)), 0o755); err != nil {
		return err
	}
	fmt.Printf("flicktask hook: %s\n", onModifyPath)

	// Install on-add hook
	onAddPath := filepath.Join(hookDir, onAddHookName)
	if err := os.WriteFile(onAddPath, []byte(hookShim("on-add", teamName)), 0o755); err != nil {
		return err
	}
	fmt.Printf("flicktask hook: %s\n", onAddPath)

	return nil
}
