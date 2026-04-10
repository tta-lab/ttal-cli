package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/statusline"
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
	RunE:   runStatusline,
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
		team := defaultTeamName
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
		if err := status.WriteAgent(team, s); err != nil {
			// Don't fail the statusline for a write error — just skip
			fmt.Fprintf(os.Stderr, "ttal: write agent status: %v\n", err)
		}
	}

	return nil
}

func printStatusLine(input statuslineInput) {
	cwd := input.Workspace.CurrentDir
	jobID := os.Getenv("TTAL_JOB_ID")
	agentName := os.Getenv("TTAL_AGENT_NAME")

	compactCwd := statusline.CompactPath(cwd, jobID)
	currentTime := time.Now().Format("15:04:05")

	// Git info
	gitInfo := ""
	if cwd != "" {
		gitInfo = getGitInfo(cwd)
	}

	ctx := ""
	pct := input.ContextWindow.UsedPercentage
	switch {
	case pct >= 75:
		ctx = fmt.Sprintf(" %sctx:%.0f%%%s", ansiYellow, pct, ansiReset)
	case pct >= 65:
		ctx = fmt.Sprintf(" ctx:%.0f%%", pct)
	}

	agentPrefix := ""
	if agentName != "" {
		agentPrefix = fmt.Sprintf("%s[%s]%s ", ansiBoldBlue, agentName, ansiReset)
	}

	fmt.Printf("%s%s%s%s%s [%s]%s\n",
		agentPrefix,
		ansiBoldYellow, compactCwd, ansiReset,
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
