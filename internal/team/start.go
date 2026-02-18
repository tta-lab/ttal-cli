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
	Name string
	Path string
}

// Start creates the team zellij session with a tab per agent.
func Start(database *ent.Client, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	sessionName := cfg.ZellijSession
	if zellij.SessionExists(sessionName) {
		if !force {
			return fmt.Errorf(
				"session %q already exists — use --force to recreate, or attach with: zellij --data-dir %s attach %s",
				sessionName, zellij.DataDir(), sessionName,
			)
		}
		fmt.Printf("Killing existing session %q...\n", sessionName)
		if err := zellij.KillSession(sessionName); err != nil {
			return fmt.Errorf("failed to kill session: %w", err)
		}
		// Wait for session to exit before deleting
		for range 15 {
			if !zellij.SessionExists(sessionName) {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	// Delete exited session if present (killed or crashed)
	_ = zellij.DeleteSession(sessionName)

	// Look up agents from config in ttal DB to get their paths
	ctx := context.Background()
	tabs := make([]AgentTab, 0, len(cfg.Agents))

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
		tabs = append(tabs, AgentTab{Name: agentName, Path: ag.Path})
	}

	if len(tabs) == 0 {
		return fmt.Errorf("no agents with paths found — register agents with: ttal agent add <name> --path <path>")
	}

	// Build layout KDL
	layoutPath, err := createTeamLayout(tabs)
	if err != nil {
		return err
	}

	handle, err := zellij.LaunchSession(sessionName, layoutPath)
	if err != nil {
		return err
	}

	if err := zellij.WaitForSession(sessionName, handle, 30e9); err != nil {
		return fmt.Errorf("session failed to start: %w", err)
	}

	fmt.Printf("Team session %q started with %d agents\n", sessionName, len(tabs))
	for _, t := range tabs {
		fmt.Printf("  %s → %s\n", t.Name, t.Path)
	}
	fmt.Printf("\nAttach with: zellij --data-dir %s attach %s\n", zellij.DataDir(), sessionName)

	return nil
}

func createTeamLayout(tabs []AgentTab) (string, error) {
	tabBlocks := make([]string, 0, len(tabs))

	for i, tab := range tabs {
		focus := ""
		if i == 0 {
			focus = " focus=true"
		}

		claudeCmd := "claude --dangerously-skip-permissions"
		if hasConversation(tab.Path) {
			claudeCmd += " --continue"
		}

		block := fmt.Sprintf(`    tab name="%s"%s {
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
    }`, tab.Name, focus, tab.Path, claudeCmd)

		tabBlocks = append(tabBlocks, block)
	}

	// Add a plain terminal tab at the end
	tabBlocks = append(tabBlocks, `    tab name="term" {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }

        pane {
            command "fish"
        }

        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }`)

	layoutContent := "layout {\n" + strings.Join(tabBlocks, "\n\n") + "\n}\n"

	layoutFile, err := os.CreateTemp("", "ttal-team-layout-*.kdl")
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
