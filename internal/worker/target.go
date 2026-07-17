package worker

import (
	"fmt"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// TmuxTarget describes where a worker or reviewer lives in tmux.
type TmuxTarget struct {
	// Session is the manager agent's tmux session, e.g. "ttal-default-astra".
	Session string
	// Window is the worker/reviewer agent name, e.g. "coder" or "pr-review-lead".
	Window string
	// WorkDir is the worktree directory path.
	WorkDir string
}

// Validate returns an error if the target has an empty session, empty window,
// or a window name containing ':', which would break "session:window" targets.
func (t *TmuxTarget) Validate() error {
	if t.Session == "" {
		return fmt.Errorf("tmux target session is empty")
	}
	if t.Window == "" {
		return fmt.Errorf("tmux target window is empty")
	}
	if strings.Contains(t.Window, ":") {
		return fmt.Errorf("tmux target window %q contains ':'", t.Window)
	}
	return nil
}

// ResolveTmuxTarget resolves the tmux target for a worker-stage task.
// The session is the owner's manager session (ttal-default-<owner>).
// The window is the pipeline worker agent name for the task's tags, falling
// back to AnyWorkerAgentName, then CoderAgentName.
// The workdir is the task's worktree path.
func ResolveTmuxTarget(task *taskwarrior.Task) (TmuxTarget, error) {
	if task == nil {
		return TmuxTarget{}, fmt.Errorf("task is nil")
	}
	if task.Owner == "" {
		return TmuxTarget{}, fmt.Errorf(
			"task %s has no owner set: cannot resolve tmux target without task owner; "+
				"run `task <uuid> modify owner:<agent>` or ensure the pipeline sets owner via ensureWorkerStageOwner",
			task.HexID())
	}

	// Resolve worker agent name: pipeline config → fallback → CoderAgentName
	agentName := resolveWorkerAgentName(task)

	workDir, err := WorktreePath(task.UUID, task.Project)
	if err != nil {
		return TmuxTarget{}, fmt.Errorf("resolve worktree path for %s: %w", task.HexID(), err)
	}

	target := TmuxTarget{
		Session: config.AgentSessionName(task.Owner),
		Window:  agentName,
		WorkDir: workDir,
	}

	if err := target.Validate(); err != nil {
		return TmuxTarget{}, fmt.Errorf("invalid tmux target for task %s: %w", task.HexID(), err)
	}

	return target, nil
}

// ResolveTmuxTargetForAgent resolves a tmux target using the provided agentName,
// bypassing pipeline config lookup. Use this when the agent name is already known
// (e.g. from SpawnConfig.AgentName or a reviewer stage).
func ResolveTmuxTargetForAgent(task *taskwarrior.Task, agentName string) (TmuxTarget, error) {
	if task == nil {
		return TmuxTarget{}, fmt.Errorf("task is nil")
	}
	if task.Owner == "" {
		return TmuxTarget{}, fmt.Errorf(
			"task %s has no owner set: cannot resolve tmux target without task owner",
			task.HexID())
	}
	if agentName == "" {
		return TmuxTarget{}, fmt.Errorf("agent name is empty")
	}

	workDir, err := WorktreePath(task.UUID, task.Project)
	if err != nil {
		return TmuxTarget{}, fmt.Errorf("resolve worktree path for %s: %w", task.HexID(), err)
	}

	target := TmuxTarget{
		Session: config.AgentSessionName(task.Owner),
		Window:  agentName,
		WorkDir: workDir,
	}

	if err := target.Validate(); err != nil {
		return TmuxTarget{}, fmt.Errorf("invalid tmux target for task %s: %w", task.HexID(), err)
	}

	return target, nil
}

// ResolveWorkerAgentName determines the worker agent name from the task's tags
// via pipeline config, falling back to AnyWorkerAgentName, then CoderAgentName.
func ResolveWorkerAgentName(task *taskwarrior.Task) string {
	if pc, err := pipeline.Load(config.DefaultConfigDir()); err == nil {
		if name := pc.WorkerAgentName(task.Tags); name != "" {
			return name
		}
		if name := pc.AnyWorkerAgentName(); name != "" {
			return name
		}
	}
	return CoderAgentName
}

// resolveWorkerAgentName is the package-level seam used by ResolveTmuxTarget.
// Set to ResolveWorkerAgentName by default; overridable in tests.
var resolveWorkerAgentName = ResolveWorkerAgentName
