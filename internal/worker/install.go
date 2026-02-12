package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	plistName = "io.guion.ttal.poll-completion"
	hookName  = "on-modify-ttal"
)

const hookShim = `#!/bin/bash
# Taskwarrior on-modify hook — delegates to ttal.
# Installed by: ttal worker install

exec ttal worker hook on-modify
`

// Install sets up both the taskwarrior hook and the launchd poll service.
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

	// 1. Install taskwarrior hook
	if err := installHook(home); err != nil {
		return fmt.Errorf("hook install failed: %w", err)
	}

	// 2. Install poll service (macOS only)
	if runtime.GOOS == "darwin" {
		if err := installPollService(home, ttalBin); err != nil {
			return fmt.Errorf("poll service install failed: %w", err)
		}
	} else {
		fmt.Println("Poll service: skipped (launchd is macOS-only)")
		fmt.Println("  Set up a cron job manually: */1 * * * * ttal worker poll")
	}

	return nil
}

// Uninstall removes the taskwarrior hook and the launchd poll service.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// 1. Remove taskwarrior hook
	hookPath := filepath.Join(home, ".task", "hooks", hookName)
	if _, err := os.Stat(hookPath); err == nil {
		os.Remove(hookPath)
		fmt.Printf("Removed taskwarrior hook: %s\n", hookPath)
	} else {
		fmt.Println("Taskwarrior hook: not installed")
	}

	// 2. Remove poll service
	if runtime.GOOS == "darwin" {
		uninstallPollService(home)
	}

	fmt.Println("\nNote: Log files remain at ~/.ttal/ and ~/.task/hooks.log")
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

func installPollService(home, ttalBin string) error {
	logDir := filepath.Join(home, ".ttal")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName+".plist")

	// Bootout existing service
	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, plistName))
	cmd.Run() // ignore error (service may not exist)

	// Read env vars to bake into plist
	forgejoURL := os.Getenv("FORGEJO_URL")
	forgejoToken := os.Getenv("FORGEJO_TOKEN")
	if forgejoToken == "" {
		forgejoToken = os.Getenv("FORGEJO_ACCESS_TOKEN")
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>

    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>worker</string>
        <string>poll</string>
    </array>

    <key>StartInterval</key>
    <integer>60</integer>

    <key>RunAtLoad</key>
    <true/>

    <key>StandardOutPath</key>
    <string>%s/poll_completion_stdout.log</string>

    <key>StandardErrorPath</key>
    <string>%s/poll_completion_stderr.log</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin:%s/.local/bin:%s/go/bin</string>
        <key>FORGEJO_URL</key>
        <string>%s</string>
        <key>FORGEJO_TOKEN</key>
        <string>%s</string>
    </dict>
</dict>
</plist>
`, plistName, ttalBin, logDir, logDir, home, home, forgejoURL, forgejoToken)

	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return err
	}

	// Warn if env vars missing
	var warnings []string
	if forgejoURL == "" {
		warnings = append(warnings, "FORGEJO_URL is not set")
	}
	if forgejoToken == "" {
		warnings = append(warnings, "FORGEJO_TOKEN/FORGEJO_ACCESS_TOKEN is not set")
	}
	if len(warnings) > 0 {
		fmt.Printf("  Warning: %s (poll won't check PR status)\n", strings.Join(warnings, ", "))
	}

	// Bootstrap service
	cmd = exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	fmt.Printf("Poll service: %s (every 60s)\n", plistPath)
	return nil
}

func uninstallPollService(home string) {
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName+".plist")

	if _, err := os.Stat(plistPath); err != nil {
		fmt.Println("Poll service: not installed")
		return
	}

	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, plistName))
	cmd.Run() // ignore error

	os.Remove(plistPath)
	fmt.Printf("Removed poll service: %s\n", plistPath)
}
