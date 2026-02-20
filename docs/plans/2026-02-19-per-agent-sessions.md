# Per-Agent Zellij Sessions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task.

**Goal:** Convert ttal from one shared Zellij session (all agents as tabs) to separate Zellij sessions per agent, so message delivery doesn't flip tabs.

**Architecture:** Each agent gets its own Zellij session named `session-<agent>` (e.g. `session-kestrel`, `session-yuki`). A new `AgentSessionName()` helper derives this by convention — no config storage needed. `ttal team start` launches N sessions instead of 1. The daemon resolves each agent's session name when delivering messages.

**Tech Stack:** Go, Cobra CLI, ent ORM, Zellij (PTY), TOML config

---

### Task 1: Add `AgentSessionName` helper to config package

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the test**

```go
// internal/config/config_test.go
package config

import "testing"

func TestAgentSessionName(t *testing.T) {
	tests := []struct {
		agent string
		want  string
	}{
		{"kestrel", "session-kestrel"},
		{"yuki", "session-yuki"},
		{"athena", "session-athena"},
	}
	for _, tt := range tests {
		if got := AgentSessionName(tt.agent); got != tt.want {
			t.Errorf("AgentSessionName(%q) = %q, want %q", tt.agent, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestAgentSessionName -v`
Expected: FAIL — `AgentSessionName` not defined

**Step 3: Write minimal implementation**

Add to `internal/config/config.go`:

```go
// AgentSessionName returns the zellij session name for an agent.
// Convention: "session-<agent-name>". Derived, not stored.
func AgentSessionName(agent string) string {
	return "session-" + agent
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestAgentSessionName -v`
Expected: PASS

**Step 5: Make `zellij_session` optional in config validation**

The global `zellij_session` field is no longer required — sessions are per-agent now. Update `Load()` to remove the validation:

```go
// Remove this block from Load():
// if cfg.ZellijSession == "" {
//     return nil, fmt.Errorf("config missing 'zellij_session'")
// }
```

Also update the `Config` struct comment:

```go
// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// ZellijSession is deprecated — sessions are now per-agent (session-<name>).
// Kept for backward compatibility but no longer required.
```

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add AgentSessionName helper, make zellij_session optional"
```

---

### Task 2: Update `team.Start` to create per-agent sessions

**Files:**
- Modify: `internal/team/start.go`

**Step 1: Rewrite `Start` to loop per agent**

Replace the single-session logic with a loop that creates one session per agent. Each session has the agent's tab + a term tab.

Key changes:
- Remove: single `createTeamLayout` with all tabs
- Add: loop over agents, create `createAgentLayout` for each, launch each session
- Session naming: `config.AgentSessionName(agentName)`
- **Without `--force`**: skip already-running sessions (only start missing ones). This is a key benefit of per-agent sessions — you can add a new agent and run `ttal team start` without disrupting existing agents.
- **With `--force`**: kill and recreate all sessions

```go
func Start(database *ent.Client, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	var started []AgentTab
	var skipped []string

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

		tab := AgentTab{Name: agentName, Path: ag.Path, Model: string(ag.Model)}
		sessionName := config.AgentSessionName(agentName)

		// Handle existing session
		if zellij.SessionExists(sessionName) {
			if !force {
				// Without --force: skip running sessions, only start missing ones
				skipped = append(skipped, agentName)
				continue
			}
			// With --force: kill and recreate
			fmt.Printf("Removing existing session %q...\n", sessionName)
			if err := zellij.KillSession(sessionName); err != nil {
				if err := zellij.DeleteSession(sessionName); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to remove session %q: %v\n", sessionName, err)
					continue
				}
			} else {
				for range 15 {
					if !zellij.SessionExists(sessionName) {
						break
					}
					time.Sleep(200 * time.Millisecond)
				}
				_ = zellij.DeleteSession(sessionName)
			}
		}

		layoutPath, err := createAgentLayout(tab)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to create layout for %s: %v\n", agentName, err)
			continue
		}

		handle, err := zellij.LaunchSession(sessionName, layoutPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to launch session for %s: %v\n", agentName, err)
			continue
		}

		if err := zellij.WaitForSession(sessionName, handle, 30*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "warning: session for %s failed to start: %v\n", agentName, err)
			continue
		}

		started = append(started, tab)
	}

	if len(started) == 0 && len(skipped) == 0 {
		return fmt.Errorf("no agent sessions started — register agents with: ttal agent add <name> --path <path>")
	}

	if len(started) > 0 {
		fmt.Printf("Started %d agent sessions:\n", len(started))
		for _, t := range started {
			fmt.Printf("  %s → %s (session: %s)\n", t.Name, t.Path, config.AgentSessionName(t.Name))
		}
	}
	if len(skipped) > 0 {
		fmt.Printf("Skipped %d already running: %s\n", len(skipped), strings.Join(skipped, ", "))
	}
	fmt.Printf("\nAttach with: ttal team attach <agent-name>\n")

	return nil
}
```

**Step 2: Replace `createTeamLayout` with `createAgentLayout`**

Each agent gets its own layout with one CC tab + one term tab:

```go
func createAgentLayout(tab AgentTab) (string, error) {
	claudeCmd := "claude --dangerously-skip-permissions"
	if tab.Model != "" {
		claudeCmd += " --model " + tab.Model
	}
	if hasConversation(tab.Path) {
		claudeCmd += " --continue"
	}

	layoutContent := fmt.Sprintf(`layout {
    tab name="%s" focus=true {
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
    }

    tab name="term" {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }

        pane {
            cwd "%s"
            command "fish"
        }

        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, tab.Name, tab.Path, claudeCmd, tab.Path)

	layoutFile, err := os.CreateTemp("", "ttal-agent-layout-*.kdl")
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
```

Delete the old `createTeamLayout` function.

**Step 3: Run build to verify**

Run: `make build`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/team/start.go
git commit -m "feat(team): create per-agent zellij sessions instead of shared session"
```

---

### Task 3: Add `ttal team attach` command

**Files:**
- Create: `internal/team/attach.go`
- Modify: `cmd/team.go`

**Step 1: Write `Attach` function**

```go
// internal/team/attach.go
package team

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// Attach attaches the current terminal to an agent's zellij session.
// Uses exec (replaces current process) so the user's terminal becomes the session.
func Attach(agentName string) error {
	sessionName := config.AgentSessionName(agentName)

	if !zellij.SessionExists(sessionName) {
		return fmt.Errorf("session %q not found — start with: ttal team start", sessionName)
	}

	zellijBin, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij not found in PATH")
	}

	// Use exec to replace current process — gives clean terminal experience
	args := []string{"zellij", "--data-dir", zellij.DataDir(), "attach", sessionName}
	return syscall.Exec(zellijBin, args, os.Environ())
}
```

**Step 2: Add cobra command**

Add to `cmd/team.go`:

```go
var teamAttachCmd = &cobra.Command{
	Use:   "attach <agent-name>",
	Short: "Attach to an agent's zellij session",
	Long: `Attach the current terminal to an agent's zellij session.

Equivalent to: zellij --data-dir <datadir> attach session-<agent-name>

Example:
  ttal team attach kestrel
  ttal team attach yuki`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return team.Attach(args[0])
	},
}
```

Add to `init()`:

```go
teamCmd.AddCommand(teamAttachCmd)
```

**Step 3: Run build to verify**

Run: `make build`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/team/attach.go cmd/team.go
git commit -m "feat(team): add 'ttal team attach <agent>' command"
```

---

### Task 4: Update daemon delivery to use per-agent sessions

**Files:**
- Modify: `internal/daemon/deliver.go`
- Modify: `internal/daemon/daemon.go`

**Step 1: Update `deliverToZellij` to accept only agent name**

The session is now derived from the agent name:

```go
// internal/daemon/deliver.go

// deliverToZellij sends text to an agent's zellij session via write-chars + Enter.
// Session name = "session-<agentName>" (convention). Tab name = agent name.
func deliverToZellij(agentName, text string) error {
	session := config.AgentSessionName(agentName)
	return zellij.WriteChars(session, agentName, "", text)
}
```

Add import for `codeberg.org/clawteam/ttal-cli/internal/config`.

**Step 2: Update all callers in `daemon.go`**

In `Run()` — Telegram poller callback:

```go
// Before:
deliverToZellij(cfg.ZellijSession, name, text)
// After:
deliverToZellij(name, text)
```

In `handleTo()`:

```go
// Before:
return deliverToZellij(cfg.ZellijSession, req.To, req.Message)
// After:
return deliverToZellij(req.To, req.Message)
```

In `handleAgentToAgent()`:

```go
// Before:
return deliverToZellij(cfg.ZellijSession, req.To, msg)
// After:
return deliverToZellij(req.To, msg)
```

**Step 3: Run build to verify**

Run: `make build`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/daemon/deliver.go internal/daemon/daemon.go
git commit -m "feat(daemon): deliver messages to per-agent sessions"
```

---

### Task 5: Update hook fallback to use per-agent sessions

**Files:**
- Modify: `internal/worker/hook.go`

**Step 1: Update `notifyAgentWith` fallback path**

The fallback (when daemon is down) currently reads `cfg.ZellijSession` from `daemon.json`. Update to use the convention:

```go
// In notifyAgentWith(), replace the fallback block:

// Before:
// cfg, err := loadHookFallbackConfig()
// ...
// zellij.WriteChars(cfg.ZellijSession, agent, "", message)

// After:
session := "session-" + agent  // convention — no config needed for fallback
if err := zellij.WriteChars(session, agent, "", message); err != nil {
	hookLogFile("ERROR: failed to deliver to zellij: " + err.Error())
	return
}
hookLogFile(fmt.Sprintf("Delivered via zellij fallback: session=%s tab=%s", session, agent))
```

Note: We hardcode the convention string here instead of importing `config` to avoid adding a TOML dependency to the hook path (hooks must be fast and lightweight). The convention `session-<name>` is stable.

**Step 2: Clean up `hookFallbackConfig`**

The `ZellijSession` field in `hookFallbackConfig` is no longer needed for delivery. However, keep the struct and `loadHookFallbackConfig()` since `LifecycleAgent` is still read from `daemon.json`. Just remove the `ZellijSession` validation:

```go
type hookFallbackConfig struct {
	LifecycleAgent string `json:"lifecycle_agent"`
}

func loadHookFallbackConfig() (*hookFallbackConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".ttal", "daemon.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("daemon config not found: %s", path)
	}

	var cfg hookFallbackConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid daemon config: %w", err)
	}

	return &cfg, nil
}
```

Remove the `ZellijSession` empty check.

**Step 3: Run build and tests**

Run: `make build && make test`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/worker/hook.go
git commit -m "feat(hook): use per-agent session convention for zellij fallback delivery"
```

---

### Task 6: Add `ttal team list` command

**Files:**
- Create: `internal/team/list.go`
- Modify: `cmd/team.go`

**Step 1: Write `List` function**

Shows all agent sessions and their status (running/stopped):

```go
// internal/team/list.go
package team

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// List shows all agent sessions and their status.
func List() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Agents) == 0 {
		fmt.Println("No agents configured.")
		return nil
	}

	fmt.Printf("%-15s %-25s %s\n", "AGENT", "SESSION", "STATUS")
	for agentName := range cfg.Agents {
		sessionName := config.AgentSessionName(agentName)
		status := "stopped"
		if zellij.SessionExists(sessionName) {
			status = "running"
		}
		fmt.Printf("%-15s %-25s %s\n", agentName, sessionName, status)
	}

	return nil
}
```

**Step 2: Add cobra command**

Add to `cmd/team.go`:

```go
var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent sessions and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return team.List()
	},
}
```

Add to `init()`:

```go
teamCmd.AddCommand(teamListCmd)
```

**Step 3: Run build to verify**

Run: `make build`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/team/list.go cmd/team.go
git commit -m "feat(team): add 'ttal team list' to show agent session status"
```

---

### Task 7: Final verification and cleanup

**Step 1: Run full CI checks**

Run: `make ci`
Expected: All checks pass (fmt, generate, vet, lint, test, build)

**Step 2: Manual smoke test**

```bash
# Build and install
make install

# List sessions (should show all agents as stopped)
ttal team list

# Start all agent sessions
ttal team start

# List again (should show running)
ttal team list

# Attach to one
ttal team attach kestrel

# (Ctrl+Q to detach from zellij)

# Test message delivery (daemon must be running)
ttal send --to kestrel "test message"

# Force restart
ttal team start --force
```

**Step 3: Commit any remaining fixes**

If any issues found during smoke test, fix and commit individually.

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `AgentSessionName()`, make `zellij_session` optional |
| `internal/config/config_test.go` | New — test for `AgentSessionName` |
| `internal/team/start.go` | Create per-agent sessions instead of shared session |
| `internal/team/attach.go` | New — `Attach()` function for `ttal team attach` |
| `internal/team/list.go` | New — `List()` function for `ttal team list` |
| `cmd/team.go` | Add `attach` and `list` subcommands |
| `internal/daemon/deliver.go` | Derive session from agent name, drop session param |
| `internal/daemon/daemon.go` | Update all `deliverToZellij` calls (remove session arg) |
| `internal/worker/hook.go` | Use `session-<name>` convention in fallback, remove `ZellijSession` from fallback config |
