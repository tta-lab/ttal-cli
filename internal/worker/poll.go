package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"errors"

	"codeberg.org/clawteam/ttal-cli/internal/forgejo"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

var (
	logDir  string
	logFile string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	logDir = filepath.Join(home, ".ttal")
	logFile = filepath.Join(logDir, "poll_completion.log")
}

// Poll checks active worker tasks for merged PRs and auto-completes them.
// Also cleans up stale temp files. Intended to be run periodically (e.g., via launchd).
func Poll() error {
	// Cleanup old task files
	cleanupOldTaskFiles()

	tasks, err := taskwarrior.GetActiveWorkerTasks()
	if err != nil {
		pollLog("ERROR", "Failed to query tasks", "error", err.Error())
		return err
	}

	if len(tasks) == 0 {
		return nil
	}

	for _, task := range tasks {
		if task.SessionName == "" || task.ProjectPath == "" {
			continue
		}

		owner, repo, err := forgejo.ParseRepoInfo(task.ProjectPath)
		if err != nil {
			pollLog("ERROR", "Could not detect repo info",
				"uuid", task.UUID,
				"session", task.SessionName,
				"path", task.ProjectPath)
			continue
		}

		if task.PRID == "" {
			pollLog("WAITING", "No pr_id UDA set",
				"uuid", task.UUID,
				"session", task.SessionName)
			continue
		}

		prID, err := strconv.ParseInt(task.PRID, 10, 64)
		if err != nil {
			pollLog("ERROR", "Invalid pr_id",
				"uuid", task.UUID,
				"pr_id", task.PRID)
			continue
		}

		merged, err := forgejo.IsPRMerged(owner, repo, prID)
		if err != nil {
			pollLog("ERROR", "Failed to fetch PR info",
				"uuid", task.UUID,
				"session", task.SessionName,
				"pr_id", task.PRID,
				"owner", owner,
				"repo", repo,
				"error", err.Error())
			continue
		}

		if !merged {
			pollLog("WAITING", "PR not merged",
				"uuid", task.UUID,
				"session", task.SessionName,
				"pr", "#"+task.PRID)
			continue
		}

		// PR is merged — close worker (session + worktree), then mark task done
		result, closeErr := Close(task.SessionName, false)

		if closeErr != nil && errors.Is(closeErr, ErrNeedsDecision) && result != nil {
			pollLog("NEEDS_DECISION", result.Status,
				"uuid", task.UUID,
				"session", task.SessionName,
				"pr", "#"+task.PRID)
			notifyTelegram(fmt.Sprintf("⚠ Worker needs cleanup decision: %s\nTask: %s\nStatus: %s",
				task.SessionName, task.Description, result.Status))
			continue
		}

		if closeErr != nil {
			status := "unknown error"
			if result != nil {
				status = result.Status
			}
			pollLog("ERROR", "Worker cleanup failed",
				"uuid", task.UUID,
				"session", task.SessionName,
				"pr", "#"+task.PRID,
				"error", status)
			notifyTelegram(fmt.Sprintf("❌ Worker cleanup error: %s\nTask: %s\nError: %s",
				task.SessionName, task.Description, status))
			continue
		}

		// Cleanup succeeded — mark task done
		if err := taskwarrior.MarkDone(task.UUID); err != nil {
			pollLog("ERROR", "Failed to mark task done",
				"uuid", task.UUID,
				"session", task.SessionName,
				"pr", "#"+task.PRID,
				"error", err.Error())
			notifyTelegram(fmt.Sprintf("❌ Failed to mark task done: %s\nTask: %s\nError: %v",
				task.SessionName, task.Description, err))
			continue
		}

		pollLog("SUCCESS", "Worker cleaned up and task completed",
			"uuid", task.UUID,
			"session", task.SessionName,
			"pr", "#"+task.PRID)
	}

	return nil
}

func cleanupOldTaskFiles() {
	tmpDir := os.TempDir()
	now := time.Now()
	ageThreshold := 24 * time.Hour
	cleaned := 0

	patterns := []string{"claude-task-*.txt", "zellij-layout-*.kdl"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(tmpDir, pattern))
		if err != nil {
			continue
		}
		for _, path := range matches {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if now.Sub(info.ModTime()) > ageThreshold {
				if err := os.Remove(path); err == nil {
					cleaned++
				}
			}
		}
	}

	if cleaned > 0 {
		pollLog("CLEANUP", fmt.Sprintf("Removed %d old task files from %s", cleaned, tmpDir))
	}
}

func pollLog(level, message string, kvs ...string) {
	_ = os.MkdirAll(logDir, 0o755)

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("%s [%s] %s", timestamp, level, message)

	// Append key-value pairs
	for i := 0; i+1 < len(kvs); i += 2 {
		line += fmt.Sprintf(" %s=%s", kvs[i], kvs[i+1])
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck

	_, _ = fmt.Fprintln(f, line)
}
