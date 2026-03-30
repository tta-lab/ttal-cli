package launchcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// BuildResumeCommand builds a gatekeeper-wrapped claude --resume command.
// sessionID: the synthetic JSONL session to resume.
// trigger: prompt passed as positional arg (may contain newlines; empty = omit entirely).
// agent: CC agent identity (empty = omit --agent flag).
// Currently only supports ClaudeCode. Codex support tracked in #321.
func BuildResumeCommand(ttalBin, sessionID string, rt runtime.Runtime, agent, trigger string) (string, error) {
	switch rt {
	case runtime.ClaudeCode:
		cmd := fmt.Sprintf(
			"%s worker gatekeeper -- claude --resume %s --dangerously-skip-permissions",
			ttalBin, sessionID,
		)
		if agent != "" {
			cmd += fmt.Sprintf(" --agent %s", agent)
		}
		if trigger != "" {
			escaped := strings.ReplaceAll(trigger, "'", "'\\''")
			cmd += fmt.Sprintf(" -- '%s'", escaped)
		}
		return cmd, nil
	default:
		return "", fmt.Errorf("unsupported runtime for resume command: %q (codex support: #321)", rt)
	}
}

// BuildCCSessionCommand resolves the CC project dir, writes a synthetic JSONL session,
// and returns the gatekeeper-wrapped claude --resume command.
// sessionPath is the full path to the JSONL file — pass to os.Remove if subsequent
// steps (e.g. tmux.NewSession) fail, to avoid orphaned session files.
// agent: --agent flag value (empty = omit). trigger: positional arg (empty = omit).
func BuildCCSessionCommand(
	ttalBin, workDir string, sessCfg breathe.SessionConfig, agent, trigger string,
) (sessionPath, cmd string, err error) {
	projectDir, err := breathe.CCProjectDir(workDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve CC project dir: %w", err)
	}
	sessionID, err := breathe.WriteSyntheticSession(projectDir, sessCfg)
	if err != nil {
		return "", "", fmt.Errorf("write synthetic session: %w", err)
	}
	sessionPath = filepath.Join(projectDir, sessionID+".jsonl")
	cmd, err = BuildResumeCommand(ttalBin, sessionID, runtime.ClaudeCode, agent, trigger)
	if err != nil {
		os.Remove(sessionPath)
		return "", "", err
	}
	return sessionPath, cmd, nil
}

// BuildCodexGatekeeperCommand builds a gatekeeper-wrapped codex command
// using the legacy task-file pattern. Claude Code uses BuildResumeCommand instead.
// This will be removed when Codex supports JSONL resume (#321).
func BuildCodexGatekeeperCommand(ttalBin, taskFile string) (string, error) {
	return fmt.Sprintf(
		"%s worker gatekeeper --task-file %s -- codex --yolo --",
		ttalBin, taskFile), nil
}
