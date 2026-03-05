package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func executeTask(uuid string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("ttal", "task", "execute", uuid)
		out, err := cmd.CombinedOutput()
		short := uuid
		if len(short) > 8 {
			short = short[:8]
		}
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("execute %s: %s", short, strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Worker spawned for %s", short), refresh: true}
	}
}

func routeTask(uuid, agentName string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("ttal", "task", "route", uuid, "--to", agentName)
		out, err := cmd.CombinedOutput()
		short := uuid
		if len(short) > 8 {
			short = short[:8]
		}
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("route %s to %s: %s", short, agentName, strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Routed %s to %s", short, agentName)}
	}
}

func openPR(uuid string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("ttal", "open", "pr", uuid)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("open PR: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: "Opened PR in browser"}
	}
}

func openSession(t *Task) tea.Cmd {
	if t.Branch == "" {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("no worker session for this task")}
		}
	}
	sessionName := t.SessionName()
	c := exec.Command("tmux", "attach-session", "-t", sessionName)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execFinishedMsg{err: err}
	})
}

func openTerm(t *Task) tea.Cmd {
	if t.ProjectPath == "" {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("no project path for this task")}
		}
	}
	workDir := resolveWorkDir(t)
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	c := exec.Command(shell)
	c.Dir = workDir
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execFinishedMsg{err: err}
	})
}

func openEditor(t *Task) tea.Cmd {
	if t.ProjectPath == "" {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("no project path for this task")}
		}
	}
	workDir := resolveWorkDir(t)
	editor := os.Getenv("TT_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, ".")
	c.Dir = workDir
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execFinishedMsg{err: err}
	})
}

func addToToday(uuid string) tea.Cmd {
	return func() tea.Msg {
		cmd := taskwarrior.Command(uuid, "modify", "scheduled:today")
		if out, err := cmd.CombinedOutput(); err != nil {
			return actionResultMsg{err: fmt.Errorf("add to today: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: "Added to today", refresh: true}
	}
}

func removeFromToday(uuid string) tea.Cmd {
	return func() tea.Msg {
		cmd := taskwarrior.Command(uuid, "modify", "scheduled:")
		if out, err := cmd.CombinedOutput(); err != nil {
			return actionResultMsg{err: fmt.Errorf("remove from today: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: "Removed from today", refresh: true}
	}
}

func doneTask(uuid string) tea.Cmd {
	return func() tea.Msg {
		cmd := taskwarrior.Command(uuid, "done")
		if out, err := cmd.CombinedOutput(); err != nil {
			return actionResultMsg{err: fmt.Errorf("done: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: "Task marked done", refresh: true}
	}
}

func modifyTask(uuid, modifiers string) tea.Cmd {
	return func() tea.Msg {
		args := []string{uuid, "modify"}
		args = append(args, strings.Fields(modifiers)...)
		cmd := taskwarrior.Command(args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return actionResultMsg{err: fmt.Errorf("modify: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: "Task modified", refresh: true}
	}
}

func annotateTask(uuid, text string) tea.Cmd {
	return func() tea.Msg {
		if err := taskwarrior.AnnotateTask(uuid, text); err != nil {
			return actionResultMsg{err: fmt.Errorf("annotate: %w", err)}
		}
		return actionResultMsg{message: "Annotation added", refresh: true}
	}
}

// resolveWorkDir finds the working directory for a task (worktree or project root).
func resolveWorkDir(t *Task) string {
	if t.Branch != "" {
		name := strings.TrimPrefix(t.Branch, "worker/")
		dir := filepath.Join(t.ProjectPath, ".worktrees", name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return t.ProjectPath
}
