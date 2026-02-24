package daemon

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

const (
	daemonPlistBase = "io.guion.ttal.daemon"
	oldPollPlist    = "io.guion.ttal.poll-completion"
)

// daemonPlistName returns the launchd label for the active team's daemon.
// Single-team (default): "io.guion.ttal.daemon"
// Multi-team: "io.guion.ttal.daemon.<team>"
func daemonPlistName() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	if len(cfg.Teams) == 0 {
		return daemonPlistBase, nil
	}
	return daemonPlistBase + "." + cfg.TeamName(), nil
}

// Install installs the launchd plist and creates a config template if needed.
func Install() error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH — install with: make install")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dataDir := config.ResolveDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	// Create config template if not present
	if cfgPath, err := config.Path(); err == nil {
		if _, statErr := os.Stat(cfgPath); os.IsNotExist(statErr) {
			if err := config.WriteTemplate(); err != nil {
				return fmt.Errorf("failed to write config template: %w", err)
			}
			fmt.Printf("Created config template: %s\n", cfgPath)
			fmt.Println("  Edit it to configure your agents before starting the daemon.")
		}
	}

	// Remove old poll-completion plist if present
	removeOldPollPlist(home)

	// Install daemon plist
	if err := installDaemonPlist(home, ttalBin, dataDir); err != nil {
		return err
	}

	return nil
}

// Uninstall removes the launchd plist and cleans up daemon files.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	label, err := daemonPlistName()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", label+".plist")

	if _, err := os.Stat(plistPath); err != nil {
		fmt.Println("Daemon plist: not installed")
	} else {
		uid := os.Getuid()
		cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, label))
		cmd.Run()

		os.Remove(plistPath)
		fmt.Printf("Removed daemon plist: %s\n", plistPath)
	}

	// Remove socket and pid
	sockPath, _ := SocketPath()
	os.Remove(sockPath)

	dataDir := config.ResolveDataDir()
	pidPath := filepath.Join(dataDir, pidFileName)
	os.Remove(pidPath)

	fmt.Println("Daemon uninstalled. Config and logs preserved.")
	return nil
}

func installDaemonPlist(home, ttalBin, dataDir string) error {
	label, err := daemonPlistName()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", label+".plist")

	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, label))
	cmd.Run()

	// Bake env vars into plist
	forgejoURL := os.Getenv("FORGEJO_URL")
	forgejoToken := os.Getenv("FORGEJO_TOKEN")
	if forgejoToken == "" {
		forgejoToken = os.Getenv("FORGEJO_ACCESS_TOKEN")
	}

	// Resolve TTAL_TEAM for multi-team setups
	ttalTeam := os.Getenv("TTAL_TEAM")

	var warnings []string
	if forgejoURL == "" {
		warnings = append(warnings, "FORGEJO_URL is not set")
	}
	if forgejoToken == "" {
		warnings = append(warnings, "FORGEJO_TOKEN/FORGEJO_ACCESS_TOKEN is not set")
	}

	// Build optional TTAL_TEAM env entry
	ttalTeamEntry := ""
	if ttalTeam != "" {
		ttalTeamEntry = fmt.Sprintf(`
        <key>TTAL_TEAM</key>
        <string>%s</string>`, xmlEscape(ttalTeam))
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
        <string>daemon</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>%s/daemon.log</string>

    <key>StandardErrorPath</key>
    <string>%s/daemon.log</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin:%s/.local/bin:%s/go/bin</string>
        <key>FORGEJO_URL</key>
        <string>%s</string>
        <key>FORGEJO_TOKEN</key>
        <string>%s</string>%s
    </dict>
</dict>
</plist>
`, label, ttalBin, dataDir, dataDir, home, home,
		xmlEscape(forgejoURL), xmlEscape(forgejoToken), ttalTeamEntry)

	if err := os.WriteFile(plistPath, []byte(plist), 0o600); err != nil {
		return err
	}

	if len(warnings) > 0 {
		fmt.Printf("  Warning: %s (worker cleanup won't function)\n", strings.Join(warnings, ", "))
	}

	cmd = exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	fmt.Printf("Daemon plist installed: %s\n", plistPath)
	fmt.Printf("Logs: %s/daemon.log\n", dataDir)
	return nil
}

// xmlEscape escapes a string for safe embedding in XML/plist content.
func xmlEscape(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		// EscapeText only fails on write errors to the builder, which can't happen.
		return s
	}
	return b.String()
}

// Start boots the daemon launchd service.
func Start() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	label, err := daemonPlistName()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", label+".plist")
	if _, err := os.Stat(plistPath); err != nil {
		return fmt.Errorf("daemon not installed (run: ttal daemon install)")
	}

	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(outStr, "already bootstrapped") || strings.Contains(outStr, "36:") {
			fmt.Println("Daemon already running")
			return nil
		}
		return fmt.Errorf("launchctl bootstrap failed: %w: %s", err, outStr)
	}

	fmt.Println("Daemon started")
	return nil
}

// Stop stops the daemon launchd service.
func Stop() error {
	label, err := daemonPlistName()
	if err != nil {
		return err
	}
	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, label))
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := strings.TrimSpace(string(out))
		if strings.Contains(outStr, "No such process") || strings.Contains(outStr, "3:") {
			fmt.Println("Daemon not running")
			return nil
		}
		return fmt.Errorf("launchctl bootout failed: %w: %s", err, outStr)
	}

	fmt.Println("Daemon stopped")
	return nil
}

// Restart stops then starts the daemon.
func Restart() error {
	if err := Stop(); err != nil {
		return err
	}
	return Start()
}

func removeOldPollPlist(home string) {
	plistPath := filepath.Join(home, "Library", "LaunchAgents", oldPollPlist+".plist")
	if _, err := os.Stat(plistPath); err != nil {
		return
	}

	uid := os.Getuid()
	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, oldPollPlist))
	cmd.Run()

	os.Remove(plistPath)
	fmt.Printf("Removed old poll-completion plist: %s\n", plistPath)
}
