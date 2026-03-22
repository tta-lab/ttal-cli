package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/claudeconfig"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// spawnTaskScopedAgent creates a new CC session for a task-scoped agent.
// Used only for the FIRST task when no prior session exists.
// Subsequent tasks use breathe rotation via handleBreathe.
func spawnTaskScopedAgent(
	mcfg *config.DaemonConfig,
	task *taskwarrior.Task,
	agent *agentfs.AgentInfo,
	rolePrompt string,
	projectPath string,
	team string,
) error {
	sessionName := task.TaskScopedSessionName(agent.Name)
	windowName := agent.Name

	ensureTaskScopedTrust(projectPath)

	model := mcfg.AgentModelForTeam(team, agent.Name)

	// Load diary for initial memory (first spawn has no handoff — diary is the memory)
	handoff := diaryReadToday(agent.Name, rolePrompt)

	projectDir, err := breathe.CCProjectDir(projectPath)
	if err != nil {
		return fmt.Errorf("resolve CC project dir: %w", err)
	}
	sessionID, err := breathe.WriteSyntheticSession(projectDir, breathe.SessionConfig{
		CWD:     projectPath,
		Handoff: handoff,
	})
	if err != nil {
		return fmt.Errorf("write synthetic session: %w", err)
	}

	ccCmd := fmt.Sprintf(
		"claude --resume %s --model %s --dangerously-skip-permissions --agent %s",
		sessionID, model, agent.Name,
	)

	taskRC := ""
	if mcfg != nil {
		if t, ok := mcfg.Teams[team]; ok {
			taskRC = t.TaskRC
		}
	}
	env := buildTaskScopedEnv(agent.Name, team, task.SessionID(), sessionName, taskRC)
	envStr := ""
	if len(env) > 0 {
		envStr = fmt.Sprintf("env %s ", strings.Join(env, " "))
	}

	shell := mcfg.Global.GetShell()
	var shellCmd string
	switch shell {
	case "fish":
		shellCmd = fmt.Sprintf("%sfish -C '%s'", envStr, ccCmd)
	default:
		shellCmd = fmt.Sprintf("%szsh -c '%s'", envStr, ccCmd)
	}

	log.Printf("[taskscoped] first spawn: %s for task %s in %s", agent.Name, task.UUID[:8], projectPath)

	if err := tmux.NewSession(sessionName, windowName, projectPath, shellCmd); err != nil {
		os.Remove(filepath.Join(projectDir, sessionID+".jsonl"))
		return fmt.Errorf("create tmux session: %w", err)
	}

	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			if err := tmux.SetEnv(sessionName, parts[0], parts[1]); err != nil {
				log.Printf("[taskscoped] warning: SetEnv %s failed for session %s: %v", parts[0], sessionName, err)
			}
		}
	}

	log.Printf("[taskscoped] %s spawned in session %s", agent.Name, sessionName)
	return nil
}

// buildTaskScopedEnv returns env vars for a task-scoped agent session.
// TTAL_JOB_ID is consumed by resolveCurrentTask() for ttal task get, ttal go, ttal comment.
// taskRC is the optional path to the team's taskrc file (empty string = omit TASKRC).
func buildTaskScopedEnv(agentName, team, jobID, sessionName, taskRC string) []string {
	env := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
		fmt.Sprintf("TTAL_TEAM=%s", team),
		fmt.Sprintf("TTAL_JOB_ID=%s", jobID),
		fmt.Sprintf("TTAL_SESSION_NAME=%s", sessionName),
		"TTAL_SESSION_MODE=task-scoped",
	}
	if taskRC != "" {
		env = append(env, fmt.Sprintf("TASKRC=%s", taskRC))
	}
	env = append(env, config.DotEnvParts()...)
	return env
}

// ensureTaskScopedTrust adds the project path to ~/.claude.json trust entries.
func ensureTaskScopedTrust(projectPath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[taskscoped] warning: cannot resolve home dir for trust: %v", err)
		return
	}
	claudeJSONPath := filepath.Join(home, ".claude.json")
	n, err := claudeconfig.UpsertTrust(claudeJSONPath, []string{projectPath})
	if err != nil {
		log.Printf("[taskscoped] warning: failed to add trust entry: %v", err)
	} else if n > 0 {
		log.Printf("[taskscoped] trust entry added for %s", projectPath)
	}
}
