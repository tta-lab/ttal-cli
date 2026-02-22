package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/status"
	"github.com/spf13/cobra"
)

type statuslineInput struct {
	ContextWindow struct {
		UsedPercentage      float64 `json:"used_percentage"`
		RemainingPercentage float64 `json:"remaining_percentage"`
	} `json:"context_window"`
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	SessionID string `json:"session_id"`
	Version   string `json:"version"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
}

// ANSI escape codes for ys zsh theme status line.
const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiBlue       = "\033[34m"
	ansiCyan       = "\033[36m"
	ansiGreen      = "\033[32m"
	ansiYellow     = "\033[33m"
	ansiRed        = "\033[31m"
	ansiBoldBlue   = ansiBold + ansiBlue
	ansiBoldYellow = ansiBold + ansiYellow
)

var statuslineCmd = &cobra.Command{
	Use:    "statusline",
	Short:  "CC statusline hook — prints status line and exports agent state",
	Hidden: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil // Skip root's DB init
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: runStatusline,
}

func init() {
	rootCmd.AddCommand(statuslineCmd)
}

func runStatusline(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var input statuslineInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("parse input: %w", err)
	}

	printStatusLine(input)

	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName != "" {
		s := status.AgentStatus{
			Agent:               agentName,
			ContextUsedPct:      input.ContextWindow.UsedPercentage,
			ContextRemainingPct: input.ContextWindow.RemainingPercentage,
			ModelID:             input.Model.ID,
			ModelName:           input.Model.DisplayName,
			SessionID:           input.SessionID,
			CCVersion:           input.Version,
			UpdatedAt:           time.Now().UTC(),
		}
		if err := status.WriteAgent(s); err != nil {
			// Don't fail the statusline for a write error — just skip
			fmt.Fprintf(os.Stderr, "ttal: write agent status: %v\n", err)
		}
	}

	return nil
}

func printStatusLine(input statuslineInput) {
	username := os.Getenv("USER")
	if username == "" {
		if u, err := user.Current(); err == nil {
			username = u.Username
		}
	}

	hostname, _ := os.Hostname()
	if idx := strings.IndexByte(hostname, '.'); idx != -1 {
		hostname = hostname[:idx]
	}

	cwd := input.Workspace.CurrentDir
	currentTime := time.Now().Format("15:04:05")

	// Git info
	gitInfo := ""
	if cwd != "" {
		gitInfo = getGitInfo(cwd)
	}

	ctx := fmt.Sprintf(" ctx:%.0f%%", input.ContextWindow.UsedPercentage)

	fmt.Printf("%s#%s %s%s%s @ %s%s%s in %s%s%s%s [%s]%s\n",
		ansiBoldBlue, ansiReset,
		ansiCyan, username, ansiReset,
		ansiGreen, hostname, ansiReset,
		ansiBoldYellow, cwd, ansiReset,
		gitInfo,
		currentTime,
		ctx,
	)
}

func getGitInfo(cwd string) string {
	// Check if it's a git repo
	check := exec.Command("git", "-C", cwd, "rev-parse", "--git-dir")
	check.Stderr = nil
	if err := check.Run(); err != nil {
		return ""
	}

	// Get branch name
	branch := ""
	symref := exec.Command("git", "-C", cwd, "symbolic-ref", "--short", "HEAD")
	symref.Stderr = nil
	if out, err := symref.Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	} else {
		revparse := exec.Command("git", "-C", cwd, "rev-parse", "--short", "HEAD")
		revparse.Stderr = nil
		if out, err := revparse.Output(); err == nil {
			branch = strings.TrimSpace(string(out))
		}
	}

	if branch == "" {
		return ""
	}

	// Check dirty state
	state := ansiGreen + "o" + ansiReset
	diffWork := exec.Command("git", "-C", cwd, "diff", "--quiet")
	diffWork.Stderr = nil
	diffCache := exec.Command("git", "-C", cwd, "diff", "--cached", "--quiet")
	diffCache.Stderr = nil
	if diffWork.Run() != nil || diffCache.Run() != nil {
		state = ansiRed + "x" + ansiReset
	}

	return fmt.Sprintf(" on %sgit%s:%s%s%s %s", ansiBlue, ansiReset, ansiCyan, branch, ansiReset, state)
}
