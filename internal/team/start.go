package team

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/zellij"

	entagent "codeberg.org/clawteam/ttal-cli/ent/agent"
)

// AgentTab holds the info needed to create a tab for one agent.
type AgentTab struct {
	Name  string
	Path  string
	Model string
}

// Start creates per-agent zellij sessions (one session per agent).
// Without --force: skips already-running sessions, only starts missing ones.
// With --force: kills and recreates all sessions.
func Start(database *ent.Client, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	started := make([]AgentTab, 0, len(cfg.Agents))
	skipped := make([]string, 0, len(cfg.Agents))

	for agentName := range cfg.Agents {
		ag, err := database.Agent.Query().
			Where(entagent.Name(agentName)).
			Only(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: agent %q not found in ttal DB, skipping\n", agentName)
			continue
		}
		if ag.Path == "" {
			fmt.Fprintf(os.Stderr, "warning: agent %q has no path, skipping\n", agentName)
			continue
		}

		tab := AgentTab{Name: agentName, Path: ag.Path, Model: string(ag.Model)}
		sessionName := config.AgentSessionName(agentName)

		if zellij.SessionExists(sessionName) {
			if !force {
				skipped = append(skipped, agentName)
				continue
			}
			if err := removeSession(sessionName); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to remove session %q: %v\n", sessionName, err)
				continue
			}
		}

		if err := launchAgentSession(sessionName, tab); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", agentName, err)
			continue
		}

		started = append(started, tab)
	}

	if len(started) == 0 && len(skipped) == 0 {
		return fmt.Errorf("no agent sessions started — register agents with: ttal agent add <name> --path <path>")
	}

	if len(started) > 0 {
		fmt.Printf("Started %d agent sessions:\n", len(started))
		for _, t := range started {
			fmt.Printf("  %s → %s (session: %s)\n", t.Name, t.Path, config.AgentSessionName(t.Name))
		}
	}
	if len(skipped) > 0 {
		fmt.Printf("Skipped %d already running: %s\n", len(skipped), strings.Join(skipped, ", "))
	}
	fmt.Printf("\nAttach with: ttal team attach <agent-name>\n")

	return nil
}

// launchAgentSession creates a layout and starts a zellij session for one agent.
func launchAgentSession(sessionName string, tab AgentTab) error {
	layoutPath, err := createAgentLayout(tab)
	if err != nil {
		return fmt.Errorf("failed to create layout: %w", err)
	}
	defer os.Remove(layoutPath) //nolint:errcheck

	handle, err := zellij.LaunchSession(sessionName, layoutPath)
	if err != nil {
		return fmt.Errorf("failed to launch session: %w", err)
	}

	if err := zellij.WaitForSession(sessionName, handle, 30*time.Second); err != nil {
		return fmt.Errorf("session failed to start: %w", err)
	}

	return nil
}

// removeSession kills and deletes an existing zellij session.
func removeSession(sessionName string) error {
	fmt.Printf("Removing existing session %q...\n", sessionName)
	if err := zellij.KillSession(sessionName); err != nil {
		return zellij.DeleteSession(sessionName)
	}
	for range 15 {
		if !zellij.SessionExists(sessionName) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if zellij.SessionExists(sessionName) {
		return zellij.DeleteSession(sessionName)
	}
	return nil
}

// buildAgentLayoutKDL generates the KDL layout content for an agent session.
// Pure function — no I/O, fully testable.
func buildAgentLayoutKDL(tab AgentTab, hasContinue bool) string {
	claudeCmd := "claude --dangerously-skip-permissions"
	if tab.Model != "" {
		claudeCmd += " --model " + tab.Model
	}
	if hasContinue {
		claudeCmd += " --continue"
	}

	return fmt.Sprintf(`layout {
    tab name="%s" focus=true {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }

        pane {
            cwd "%s"
            command "fish"
            args "-C" "%s"
        }

        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }

    tab name="term" {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }

        pane {
            cwd "%s"
            command "fish"
        }

        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, tab.Name, tab.Path, claudeCmd, tab.Path)
}

func createAgentLayout(tab AgentTab) (string, error) {
	hasContinue := hasConversation(tab.Path)
	layoutContent := buildAgentLayoutKDL(tab, hasContinue)

	layoutFile, err := os.CreateTemp("", "ttal-agent-layout-*.kdl")
	if err != nil {
		return "", fmt.Errorf("failed to create layout file: %w", err)
	}
	if _, err := layoutFile.WriteString(layoutContent); err != nil {
		_ = layoutFile.Close()
		return "", fmt.Errorf("failed to write layout file: %w", err)
	}
	_ = layoutFile.Close()

	return layoutFile.Name(), nil
}

// hasConversation checks if Claude Code has a previous conversation for the given path.
// Claude stores conversations as .jsonl files in ~/.claude/projects/<sanitized-path>/.
func hasConversation(workDir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Claude sanitizes paths: / and . → -
	sanitized := strings.ReplaceAll(workDir, string(filepath.Separator), "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	projectDir := filepath.Join(home, ".claude", "projects", sanitized)

	matches, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if err != nil {
		return false
	}
	return len(matches) > 0
}
