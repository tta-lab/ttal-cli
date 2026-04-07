package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// runTtalCommand runs a ttal subcommand and returns combined output.
func runTtalCommand(args ...string) ([]byte, error) {
	return exec.Command("ttal", args...).CombinedOutput()
}

func advanceTask(uuid string) tea.Cmd {
	return func() tea.Msg {
		out, err := runTtalCommand("go", uuid)
		short := uuid
		if len(short) > 8 {
			short = short[:8]
		}
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("advance %s: %s", short, strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: strings.TrimSpace(string(out)), refresh: true}
	}
}

func openPR(uuid string) tea.Cmd {
	return func() tea.Msg {
		out, err := runTtalCommand("open", "pr", uuid)
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("open PR: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: "Opened PR in browser"}
	}
}

func openSession(t *Task, cfg *config.Config) tea.Cmd {
	// Try worker session first.
	sessionName := t.SessionName()
	if tmux.SessionExists(sessionName) {
		c := exec.Command("tmux", "attach-session", "-t", sessionName)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return execFinishedMsg{err: fmt.Errorf("attach worker session %q: %w", sessionName, err)}
			}
			return execFinishedMsg{}
		})
	}

	// Fall back to owner agent session if task has owner UDA set.
	// Worker-stage tasks have no owner written, so this branch is skipped for them.
	if t.Owner != "" && cfg != nil {
		ownerSession := config.AgentSessionName(t.Owner)
		if tmux.SessionExists(ownerSession) {
			c := exec.Command("tmux", "attach-session", "-t", ownerSession)
			return tea.ExecProcess(c, func(err error) tea.Msg {
				if err != nil {
					return execFinishedMsg{err: fmt.Errorf("attach agent session %q: %w", ownerSession, err)}
				}
				return execFinishedMsg{}
			})
		}
	}

	return func() tea.Msg {
		return actionResultMsg{err: fmt.Errorf("no worker or agent session for this task")}
	}
}

func openTerm(t *Task) tea.Cmd {
	projectPath, err := project.ResolveProjectPathOrError(t.Project)
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: err}
		}
	}
	workDir := resolveWorkDir(t, projectPath)
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
	if t.UUID == "" {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("task has no UUID")}
		}
	}
	c := taskwarrior.Command(t.UUID, "edit")
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

func deleteTask(uuid string) tea.Cmd {
	return func() tea.Msg {
		if uuid == "" {
			return actionResultMsg{err: fmt.Errorf("delete: task has no UUID")}
		}
		cmd := taskwarrior.Command("rc.confirmation:off", uuid, "delete")
		if out, err := cmd.CombinedOutput(); err != nil {
			return actionResultMsg{err: fmt.Errorf("delete: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: "Task deleted", refresh: true}
	}
}

func modifyTask(uuid, modifiers string) tea.Cmd {
	return func() tea.Msg {
		fields := strings.Fields(modifiers)
		args := make([]string, 0, 2+len(fields))
		args = append(args, uuid, "modify")
		args = append(args, fields...)
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

func toggleNext(t *Task) tea.Cmd {
	return func() tea.Msg {
		hasNext := false
		for _, tag := range t.Tags {
			if tag == "next" {
				hasNext = true
				break
			}
		}
		var modifier string
		var message string
		if hasNext {
			modifier = "-next"
			message = "Removed +next tag"
		} else {
			modifier = "+next"
			message = "Added +next tag"
		}
		cmd := taskwarrior.Command(t.UUID, "modify", modifier)
		if out, err := cmd.CombinedOutput(); err != nil {
			return actionResultMsg{err: fmt.Errorf("toggle next: %s", strings.TrimSpace(string(out)))}
		}
		return actionResultMsg{message: message, refresh: true}
	}
}

func copyTask(t *Task) tea.Cmd {
	return func() tea.Msg {
		var b strings.Builder
		fmt.Fprintf(&b, "Task: %s\n", t.Description)
		fmt.Fprintf(&b, "ID: %s\n", t.UUID)
		if t.Project != "" {
			fmt.Fprintf(&b, "Project: %s\n", t.Project)
		}
		if len(t.Tags) > 0 {
			fmt.Fprintf(&b, "Tags: %s\n", strings.Join(t.Tags, ", "))
		}
		if t.Priority != "" {
			fmt.Fprintf(&b, "Priority: %s\n", t.Priority)
		}

		if len(t.Annotations) > 0 {
			b.WriteString("\nAnnotations:\n")
			for _, ann := range t.Annotations {
				date := ""
				if ann.Entry != "" {
					date = ann.Entry[:8] + " "
				}
				fmt.Fprintf(&b, "- %s%s\n", date, ann.Description)
			}
		}

		if err := clipboardWrite(b.String()); err != nil {
			return actionResultMsg{err: fmt.Errorf("copy to clipboard: %w", err)}
		}
		return actionResultMsg{message: "Copied to clipboard"}
	}
}

func clipboardWrite(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip or xsel)")
		}
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// resolveWorkDir finds the working directory for a task (worktree or project root).
func resolveWorkDir(t *Task, projectPath string) string {
	if t.UUID != "" && t.Project != "" {
		worktreeRoot := config.EnsureWorktreeRoot()
		dir := filepath.Join(worktreeRoot, fmt.Sprintf("%s-%s", t.UUID[:8], t.Project))
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	return projectPath
}
