# Task Enrichment Hook + Deterministic Spawn

> **For Claude:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task.

**Goal:** Auto-enrich new taskwarrior tasks with `project_path` and `branch` UDAs via a haiku one-shot, then deterministically spawn workers on task start — no LLM or agent messaging needed at start time.

**Architecture:** Two separate taskwarrior hooks: `on-add-ttal` enriches tasks via `claude -p --model haiku` (fire-and-forget background), `on-modify-ttal` (existing) gains start detection that reads enriched UDAs and calls `ttal worker spawn` directly (fire-and-forget background). All lifecycle events (spawn, cleanup, errors) are reported to kestrel's Telegram chat via the daemon socket.

**Tech Stack:** Go, taskwarrior hooks, `claude -p` CLI, daemon socket (Unix domain), `exec.Command` with `Setsid` for process detachment.

---

### Task 1: Add `notifyTelegram` helper to hook.go

The existing `notifyAgent` sends to an agent's zellij tab. We need a new helper that sends to an agent's Telegram chat via daemon socket `{From: agentName}`.

**Files:**
- Modify: `internal/worker/hook.go`

**Step 1: Add `notifyTelegram` function**

After the existing `notifyAgent` function, add:

```go
// notifyTelegram sends a message to an agent's Telegram chat via the daemon.
// Uses From-only routing (daemon's handleFrom → Telegram Bot API).
// Fire-and-forget: errors are logged but not propagated.
func notifyTelegram(message string) {
	agent := defaultLifecycleAgent
	if cfg, err := loadHookFallbackConfig(); err == nil && cfg.LifecycleAgent != "" {
		agent = cfg.LifecycleAgent
	}
	req := daemonSendRequest{From: agent, Message: message}
	if err := sendToDaemon(req); err != nil {
		hookLogFile(fmt.Sprintf("ERROR: telegram notify failed for %s: %v", agent, err))
	}
}
```

**Step 2: Build and verify**

Run: `go build ./...`

**Step 3: Commit**

```bash
git add internal/worker/hook.go
git commit -m "feat(hook): add notifyTelegram helper for daemon→Telegram routing"
```

---

### Task 2: Add `readHookAddInput` for on-add protocol

Taskwarrior on-add hooks receive 1 JSON line (not 2 like on-modify). Add a reader for this protocol.

**Files:**
- Modify: `internal/worker/hook.go`

**Step 1: Add hookTask accessors needed by on-add and on-start**

Re-add the accessors that were removed (needed for tag matching and enrichment):

```go
func (t hookTask) Tags() []string {
	raw, ok := t["tags"].([]any)
	if !ok {
		return nil
	}
	tags := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			tags = append(tags, s)
		}
	}
	return tags
}

func (t hookTask) ProjectPath() string {
	v, _ := t["project_path"].(string)
	return v
}

func (t hookTask) Branch() string {
	v, _ := t["branch"].(string)
	return v
}

func (t hookTask) Start() string {
	v, _ := t["start"].(string)
	return v
}
```

**Step 2: Add `readHookAddInput`**

```go
// readHookAddInput reads a single task JSON from stdin (taskwarrior on-add protocol).
func readHookAddInput() (hookTask, error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading task from stdin: %w", err)
		}
		return nil, fmt.Errorf("failed to read task from stdin")
	}

	var task hookTask
	if err := json.Unmarshal(scanner.Bytes(), &task); err != nil {
		return nil, fmt.Errorf("failed to parse task: %w", err)
	}

	return task, nil
}
```

**Step 3: Add `forkBackground` helper**

```go
// forkBackground launches a detached subprocess that runs independently of the hook process.
// Used for fire-and-forget operations that must not block taskwarrior.
func forkBackground(args ...string) error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH: %w", err)
	}

	cmd := exec.Command(ttalBin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to fork background process: %w", err)
	}

	// Detach — don't wait for child
	go cmd.Wait() //nolint:errcheck
	return nil
}
```

This needs `"os/exec"` and `"syscall"` imports added to hook.go.

**Step 4: Build and verify**

Run: `go build ./...`

**Step 5: Commit**

```bash
git add internal/worker/hook.go
git commit -m "feat(hook): add on-add input reader, forkBackground, and hookTask accessors"
```

---

### Task 3: Implement `on-add` hook handler

The on-add hook reads the new task, checks if tags already match an agent (skip enrichment), otherwise forks background enrichment.

**Files:**
- Create: `internal/worker/hook_on_add.go`

**Step 1: Write the on-add handler**

```go
package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/ent/tag"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

// HookOnAdd handles the taskwarrior on-add event.
// Reads one JSON line from stdin, outputs it back to stdout.
// If the task's tags don't match any agent, forks background enrichment.
func HookOnAdd() {
	task, err := readHookAddInput()
	if err != nil {
		hookLogFile("ERROR in on-add: " + err.Error())
		os.Exit(0)
	}
	defer passthroughTask(task)

	hookLog("ADD", task.UUID(), task.Description())

	// Skip enrichment if task tags already match an agent
	if tagsMatchAgent(task.Tags()) {
		hookLog("ADD_SKIP", task.UUID(), task.Description(), "reason", "tags_match_agent")
		return
	}

	// Fork background enrichment
	if err := forkBackground("worker", "hook", "enrich", task.UUID()); err != nil {
		hookLogFile("ERROR forking enrichment: " + err.Error())
		return
	}

	hookLog("ADD_ENRICH", task.UUID(), task.Description(), "status", "forked")
}

// tagsMatchAgent checks if any of the given tags match a registered agent's tags.
func tagsMatchAgent(taskTags []string) bool {
	if len(taskTags) == 0 {
		return false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	dbPath := filepath.Join(home, ".ttal", "ttal.db")
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}

	database, err := db.New(dbPath)
	if err != nil {
		return false
	}
	defer database.Close() //nolint:errcheck

	count, err := database.Agent.Query().
		Where(agent.HasTagsWith(tag.NameIn(taskTags...))).
		Count(context.Background())
	if err != nil {
		return false
	}

	return count > 0
}
```

**Step 2: Build and verify**

Run: `go build ./...`

**Step 3: Commit**

```bash
git add internal/worker/hook_on_add.go
git commit -m "feat(hook): add on-add handler with tag-match skip logic"
```

---

### Task 4: Implement background enrichment command

This is the detached subprocess that runs `claude -p --model haiku` to enrich a task with `project_path` and `branch` UDAs.

**Files:**
- Create: `internal/worker/hook_enrich.go`

**Step 1: Write the enrichment logic**

```go
package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

const enrichTimeout = 60 * time.Second

// HookEnrich runs background task enrichment via claude -p --model haiku.
// Called as a detached subprocess by the on-add hook.
func HookEnrich(uuid string) {
	hookLogFile(fmt.Sprintf("enrich: starting for task %s", uuid))

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR exporting task %s: %v", uuid, err))
		notifyTelegram(fmt.Sprintf("⚠ Task enrichment failed: %s\nError: %v", uuid, err))
		return
	}

	// Build context from description + annotations
	taskContext := task.Description
	for _, ann := range task.Annotations {
		taskContext += "\n" + ann.Description
	}

	prompt := buildEnrichPrompt(taskContext, task.UUID)

	ctx, cancel := context.WithTimeout(context.Background(), enrichTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", "--allowedTools", "Bash")
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		hookLogFile(fmt.Sprintf("enrich: ERROR running claude for %s: %v\nOutput: %s", uuid, err, string(out)))
		notifyTelegram(fmt.Sprintf("⚠ Task enrichment failed: %s\n%s\nError: %v", task.Description, uuid, err))
		return
	}

	hookLogFile(fmt.Sprintf("enrich: completed for task %s\nOutput: %s", uuid, string(out)))
	notifyTelegram(fmt.Sprintf("✅ Task enriched: %s\n%s", task.Description, strings.TrimSpace(string(out))))
}

func buildEnrichPrompt(taskContext, uuid string) string {
	return fmt.Sprintf(`You are a task enrichment agent. Your job is to enrich a taskwarrior task with project_path and branch UDAs so it can be automatically spawned as a worker.

TASK UUID: %s

TASK CONTEXT:
%s

INSTRUCTIONS:
1. Run: ttal project list
2. From the task context, identify which project this task belongs to
3. Run: ttal project get <alias> to get the project path
4. Derive a short, kebab-case branch name from the task description (e.g., "fix-auth-timeout", "add-user-api")
5. Run: task %s modify project_path:<path> branch:worker/<branch-name>
6. Print a one-line summary of what you set

RULES:
- Branch name should be descriptive but short (2-4 words, kebab-case)
- If you cannot determine the project, do NOT modify the task — just print "SKIP: could not determine project"
- Do not add any other modifications
- Do not start the task`, uuid, taskContext, uuid)
}
```

**Step 2: Build and verify**

Run: `go build ./...`

**Step 3: Commit**

```bash
git add internal/worker/hook_enrich.go
git commit -m "feat(hook): add haiku-powered task enrichment background command"
```

---

### Task 5: Implement deterministic on-start hook

Re-add the on-start detection in on-modify. When a task starts, read enriched UDAs and fork `ttal worker spawn` in the background.

**Files:**
- Create: `internal/worker/hook_on_start.go`
- Modify: `internal/worker/hook_on_modify.go`

**Step 1: Write the on-start handler**

```go
package worker

import (
	"fmt"
	"os"
	"strings"
)

// HookOnStart handles the task start (+ACTIVE) event.
// Reads two JSON lines from stdin, outputs modified task to stdout.
func HookOnStart() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-start: " + err.Error())
		os.Exit(0)
	}

	if original.Start() != "" || modified.Start() == "" || modified.Status() != taskStatusPending {
		passthroughTask(modified)
		return
	}

	handleOnStart(original, modified)
}

// handleOnStart forks a background spawn if the task has enriched UDAs.
func handleOnStart(_ hookTask, modified hookTask) {
	defer passthroughTask(modified)

	hookLog("START", modified.UUID(), modified.Description())

	projectPath := modified.ProjectPath()
	branch := modified.Branch()

	if projectPath == "" || branch == "" {
		hookLog("START_SKIP", modified.UUID(), modified.Description(),
			"reason", "missing_udas", "project_path", projectPath, "branch", branch)
		notifyTelegram(fmt.Sprintf("⚠ Task started but missing UDAs (not enriched?):\n%s\nproject_path=%s branch=%s",
			modified.Description(), projectPath, branch))
		return
	}

	// Derive worker name from branch (worker/fix-auth → fix-auth)
	workerName := strings.TrimPrefix(branch, "worker/")

	// Fork background spawn
	if err := forkBackground("worker", "hook", "spawn-worker",
		modified.UUID(), workerName, projectPath); err != nil {
		hookLogFile(fmt.Sprintf("ERROR forking spawn for %s: %v", modified.UUID(), err))
		notifyTelegram(fmt.Sprintf("⚠ Failed to fork worker spawn:\n%s\nError: %v",
			modified.Description(), err))
		return
	}

	hookLog("START_SPAWN", modified.UUID(), modified.Description(),
		"worker", workerName, "project", projectPath, "status", "forked")
}
```

**Step 2: Re-add start detection to on-modify**

Modify `internal/worker/hook_on_modify.go`:

```go
package worker

import "os"

// HookOnModify is the main taskwarrior on-modify hook entry point.
func HookOnModify() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-modify: " + err.Error())
		os.Exit(0)
	}

	// Detect: Task Start (pending, no start → pending, has start)
	if original.Start() == "" && modified.Start() != "" && modified.Status() == taskStatusPending {
		handleOnStart(original, modified)
		return
	}

	// Detect: Task Complete (pending → completed)
	if original.Status() == taskStatusPending && modified.Status() == taskStatusCompleted {
		handleOnComplete(original, modified)
		return
	}

	// No matching event — pass through
	passthroughTask(modified)
}
```

**Step 3: Build and verify**

Run: `go build ./...`

**Step 4: Commit**

```bash
git add internal/worker/hook_on_start.go internal/worker/hook_on_modify.go
git commit -m "feat(hook): add deterministic on-start with background spawn"
```

---

### Task 6: Implement background spawn-worker command

This is the detached subprocess that runs `ttal worker spawn` and reports results to Telegram.

**Files:**
- Create: `internal/worker/hook_spawn.go`

**Step 1: Write the spawn wrapper**

```go
package worker

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
)

// HookSpawnWorker runs worker spawn as a background process and reports to Telegram.
// Called as a detached subprocess by the on-start hook.
func HookSpawnWorker(uuid, workerName, projectPath string) {
	hookLogFile(fmt.Sprintf("spawn-worker: starting %s for task %s in %s", workerName, uuid, projectPath))

	// Load task to check for brainstorm/sonnet tags
	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		hookLogFile(fmt.Sprintf("spawn-worker: ERROR exporting task %s: %v", uuid, err))
		notifyTelegram(fmt.Sprintf("⚠ Worker spawn failed: %s\nError: could not load task: %v", workerName, err))
		return
	}

	err = Spawn(SpawnConfig{
		Name:     workerName,
		Project:  projectPath,
		TaskUUID: uuid,
		Worktree: true,
		Yolo:     true,
	})

	if err != nil {
		hookLogFile(fmt.Sprintf("spawn-worker: ERROR spawning %s: %v", workerName, err))
		notifyTelegram(fmt.Sprintf("⚠ Worker spawn failed: %s\nTask: %s\nError: %v",
			workerName, task.Description, err))
		return
	}

	hookLogFile(fmt.Sprintf("spawn-worker: successfully spawned %s", workerName))
	notifyTelegram(fmt.Sprintf("🚀 Worker spawned: %s\nTask: %s\nProject: %s",
		workerName, task.Description, projectPath))
}
```

**Step 2: Build and verify**

Run: `go build ./...`

**Step 3: Commit**

```bash
git add internal/worker/hook_spawn.go
git commit -m "feat(hook): add background spawn-worker with Telegram reporting"
```

---

### Task 7: Wire up CLI commands

Add the new subcommands: `on-add`, `enrich`, and `spawn-worker` to the hook command tree.

**Files:**
- Modify: `cmd/worker_hook.go`

**Step 1: Add the new commands**

```go
package cmd

import (
	"codeberg.org/clawteam/ttal-cli/internal/worker"
	"github.com/spf13/cobra"
)

var workerHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Taskwarrior hook handlers",
	Long:  `Commands invoked by taskwarrior hooks to handle worker lifecycle events.`,
}

var workerHookOnModifyCmd = &cobra.Command{
	Use:   "on-modify",
	Short: "Handle any on-modify event",
	Long: `Main entry point for taskwarrior on-modify hook.

Reads two JSON lines from stdin, detects the event type (start or complete),
and dispatches to the appropriate handler. For unmatched events, passes through.

This is what the installed hook shim calls. Always exits 0.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnModify()
	},
}

var workerHookOnAddCmd = &cobra.Command{
	Use:   "on-add",
	Short: "Handle task creation event",
	Long: `Handle taskwarrior on-add event.

Reads one JSON line from stdin (the new task).
Outputs the task JSON to stdout (required by taskwarrior).

If the task's tags don't match any registered agent, forks a background
enrichment process (claude -p --model haiku) to set project_path and branch UDAs.

This command always exits 0 to avoid blocking taskwarrior.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnAdd()
	},
}

var workerHookOnCompleteCmd = &cobra.Command{
	Use:   "on-complete",
	Short: "Handle task completion event",
	Long: `Handle task completion event from taskwarrior on-modify hook.

Reads two JSON lines from stdin (original and modified task).
Outputs the modified task JSON to stdout (required by taskwarrior).

For worker tasks (with session_name UDA):
- Calls worker close directly (no subprocess)
- Auto-cleans if PR merged + worktree clean
- Notifies agent if manual decision needed or on error

This command always exits 0 to avoid blocking taskwarrior.`,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookOnComplete()
	},
}

var workerHookEnrichCmd = &cobra.Command{
	Use:    "enrich <uuid>",
	Short:  "Background task enrichment via haiku",
	Long:   `Internal command — called by on-add hook as a detached subprocess. Not for direct use.`,
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookEnrich(args[0])
	},
}

var workerHookSpawnWorkerCmd = &cobra.Command{
	Use:    "spawn-worker <uuid> <worker-name> <project-path>",
	Short:  "Background worker spawn",
	Long:   `Internal command — called by on-start hook as a detached subprocess. Not for direct use.`,
	Args:   cobra.ExactArgs(3),
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		worker.HookSpawnWorker(args[0], args[1], args[2])
	},
}

func init() {
	workerHookCmd.AddCommand(workerHookOnModifyCmd)
	workerHookCmd.AddCommand(workerHookOnAddCmd)
	workerHookCmd.AddCommand(workerHookOnCompleteCmd)
	workerHookCmd.AddCommand(workerHookEnrichCmd)
	workerHookCmd.AddCommand(workerHookSpawnWorkerCmd)
}
```

**Step 2: Build and verify**

Run: `go build ./...`

**Step 3: Commit**

```bash
git add cmd/worker_hook.go
git commit -m "feat(cmd): wire up on-add, enrich, and spawn-worker hook commands"
```

---

### Task 8: Install the on-add hook shim

Update the installer to create both `on-add-ttal` and `on-modify-ttal` hook shims.

**Files:**
- Modify: `internal/worker/install.go`

**Step 1: Add on-add hook installation**

```go
package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	onModifyHookName = "on-modify-ttal"
	onAddHookName    = "on-add-ttal"
)

const onModifyHookShim = `#!/bin/bash
# Taskwarrior on-modify hook — delegates to ttal.
# Installed by: ttal worker install

exec ttal worker hook on-modify
`

const onAddHookShim = `#!/bin/bash
# Taskwarrior on-add hook — delegates to ttal.
# Installed by: ttal worker install

exec ttal worker hook on-add
`

// Install sets up the taskwarrior hooks (on-add and on-modify).
func Install() error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH — install with: make install")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	fmt.Printf("Using ttal binary: %s\n\n", ttalBin)

	if err := installHooks(home); err != nil {
		return fmt.Errorf("hook install failed: %w", err)
	}

	fmt.Println("\nNote: Worker completion polling is now handled by the ttal daemon.")
	fmt.Println("  Run: ttal daemon install")

	return nil
}

// Uninstall removes the taskwarrior hooks.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	hookDir := filepath.Join(home, ".task", "hooks")
	for _, name := range []string{onModifyHookName, onAddHookName} {
		hookPath := filepath.Join(hookDir, name)
		if _, err := os.Stat(hookPath); err == nil {
			os.Remove(hookPath) //nolint:errcheck
			fmt.Printf("Removed taskwarrior hook: %s\n", hookPath)
		} else {
			fmt.Printf("Taskwarrior hook %s: not installed\n", name)
		}
	}

	fmt.Println("\nNote: Log files remain at ~/.ttal/ and ~/.task/hooks.log")
	fmt.Println("  To also remove the daemon: ttal daemon uninstall")
	return nil
}

func installHooks(home string) error {
	hookDir := filepath.Join(home, ".task", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Backup existing Python hook if present
	pythonHook := filepath.Join(hookDir, "on-modify-worker-lifecycle")
	if _, err := os.Stat(pythonHook); err == nil {
		backupPath := pythonHook + ".bak"
		if err := os.Rename(pythonHook, backupPath); err != nil {
			return fmt.Errorf("failed to backup Python hook: %w", err)
		}
		fmt.Printf("Backed up Python hook: %s\n", backupPath)
	}

	// Install on-modify hook
	onModifyPath := filepath.Join(hookDir, onModifyHookName)
	if err := os.WriteFile(onModifyPath, []byte(onModifyHookShim), 0o755); err != nil {
		return err
	}
	fmt.Printf("Taskwarrior hook: %s\n", onModifyPath)

	// Install on-add hook
	onAddPath := filepath.Join(hookDir, onAddHookName)
	if err := os.WriteFile(onAddPath, []byte(onAddHookShim), 0o755); err != nil {
		return err
	}
	fmt.Printf("Taskwarrior hook: %s\n", onAddPath)

	return nil
}
```

**Step 2: Build and verify**

Run: `go build ./...`

**Step 3: Test install/uninstall**

Run: `./ttal worker install` — should show both hooks installed.
Run: `ls ~/.task/hooks/on-*-ttal` — should show two files.

**Step 4: Commit**

```bash
git add internal/worker/install.go
git commit -m "feat(hook): install on-add-ttal hook shim alongside on-modify-ttal"
```

---

### Task 9: Add Telegram notifications to on-complete

The existing on-complete already notifies the lifecycle agent via zellij. Add Telegram notifications for all outcomes.

**Files:**
- Modify: `internal/worker/hook_on_complete.go`

**Step 1: Add Telegram notifications to each outcome**

In `handleOnComplete`, after each existing `notifyAgent` call (or in the success path where there is none), add a `notifyTelegram` call:

For auto-cleaned success (after the hookLog at line 46):
```go
notifyTelegram(fmt.Sprintf("✅ Worker auto-cleaned: %s\nTask: %s", sessionName, modified.Description()))
```

For needs-decision (after the existing `notifyAgent` call at line 68):
```go
notifyTelegram(fmt.Sprintf("⚠ Worker needs cleanup decision: %s\nTask: %s\nStatus: %s",
    sessionName, modified.Description(), result.Status))
```

For error (after the existing `notifyAgent` call at line 94):
```go
notifyTelegram(fmt.Sprintf("❌ Worker cleanup error: %s\nTask: %s\nError: %s",
    sessionName, modified.Description(), status))
```

**Step 2: Build and verify**

Run: `go build ./...`

**Step 3: Commit**

```bash
git add internal/worker/hook_on_complete.go
git commit -m "feat(hook): add Telegram notifications for all on-complete outcomes"
```

---

### Task 10: Update CLAUDE.md and README.md

Document the new hook architecture.

**Files:**
- Modify: `CLAUDE.md` — update daemon architecture table and project structure
- Modify: `README.md` — update worker setup and task routing sections

**Step 1: Update CLAUDE.md daemon architecture table**

Add the new paths to the table:

| Path | Channel | Handler |
|---|---|---|
| JSONL watcher (fsnotify) | Telegram (outbound) | `watcher.Watcher` |
| `ttal send --to kestrel` | Zellij write-chars | `handleTo` |
| `ttal send --from yuki --to kestrel` | Zellij write-chars + attribution | `handleAgentToAgent` |
| on-add hook (task created) | Background `claude -p` enrichment | `HookOnAdd` → `HookEnrich` |
| on-modify hook (task started) | Background `ttal worker spawn` | `handleOnStart` → `HookSpawnWorker` |
| on-modify hook (task completed) | Direct worker close | `handleOnComplete` |

**Step 2: Update README.md**

Update the "Task Routing" and "Worker Setup" sections to describe the new flow:
- on-add: auto-enrichment via haiku (skipped if tags match agent)
- on-start: deterministic spawn from enriched UDAs
- on-complete: auto-cleanup (existing)

**Step 3: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: update architecture for on-add enrichment and deterministic spawn"
```

---

## Summary

| Hook | Event | Action | Blocking? |
|------|-------|--------|-----------|
| `on-add-ttal` | Task created | Enrichment via haiku (if tags don't match agent) | No — fork |
| `on-modify-ttal` | Task started | Spawn worker from enriched UDAs | No — fork |
| `on-modify-ttal` | Task completed | Close worker, cleanup | Yes (existing) |

All lifecycle events (spawn, cleanup, errors) report to kestrel's Telegram chat via daemon socket `{From: "kestrel"}`.
