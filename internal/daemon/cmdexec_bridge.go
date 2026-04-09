package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/cmdexec"
	"github.com/tta-lab/ttal-cli/internal/config"
	envpkg "github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// maxPayloadBytes is the tmux send-keys safety margin. Payloads larger than this
// are head-truncated — the last 64KB (tail) are kept so the model still sees
// exit codes and error messages that live at the end of the output.
const maxPayloadBytes = 64 * 1024

// recursionGuard rejects ttal go commands at the bridge layer to prevent feedback loops.
var recursionGuard = regexp.MustCompile(`(?i)^\s*ttal\s+go\b`)

// cmdexecBridge holds state for the cmdexec dispatcher.
type cmdexecBridge struct {
	cfg          *config.DaemonConfig
	runner       logos.CommandRunner
	projectStore *project.Store
	agentMutexes sync.Map // map[agentName]*sync.Mutex
}

// getMutex returns the mutex for a given agent name, creating it lazily.
func (b *cmdexecBridge) getMutex(agentName string) *sync.Mutex {
	mu, _ := b.agentMutexes.LoadOrStore(agentName, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// startCmdExec assembles the cmdexec dispatcher for CC sessions that have
// a registered workspace path. Returns nil if the temenos client cannot be
// constructed — the rest of the daemon continues without cmdexec.
func startCmdExec(mcfg *config.DaemonConfig) watcher.CmdFunc {
	runner, err := logos.NewClient("")
	if err != nil {
		log.Printf("[cmdexec] temenos client unavailable: %v — cmdexec disabled", err)
		return nil
	}

	// Probe: run a harmless command to check sandbox health.
	probe, probeErr := runner.Run(context.Background(), logos.RunRequest{Command: "echo __ttal_sandbox_probe__"})
	if probeErr != nil {
		log.Printf("[cmdexec] temenos probe failed: %v — cmdexec disabled", probeErr)
		return nil
	}
	if strings.Contains(probe.Stdout, "__ttal_sandbox_probe__") {
		log.Print("[cmdexec] temenos connected, sandbox active")
	} else {
		log.Printf("[cmdexec] WARNING: temenos responded but sandbox appears inactive — proceeding anyway")
	}

	// Load project store.
	store := project.NewStore(filepath.Join(config.DefaultConfigDir(), "projects.toml"))

	bridge := &cmdexecBridge{
		cfg:          mcfg,
		runner:       runner,
		projectStore: store,
		agentMutexes: sync.Map{},
	}

	return bridge.dispatch
}

// dispatch is the watcher.CmdFunc implementation.
func (b *cmdexecBridge) dispatch(teamName, agentName string, cmds []string) {
	if b.cfg == nil {
		log.Printf("[cmdexec] dispatch called with nil config — skipping")
		return
	}

	// Resolve agent workspace from config.
	agentCwd := b.cfg.Global.AgentPath(agentName)
	if agentCwd == "" {
		log.Printf("[cmdexec] no workspace for agent %s — skipping dispatch", agentName)
		return
	}

	// Compute sandbox policy.
	policy, ok := cmdexec.PolicyForAgent(b.projectStore, agentCwd)
	if !ok {
		log.Printf("[cmdexec] no policy for %s (cwd=%s) — skipping", agentName, agentCwd)
		return
	}

	// Serialize dispatches per agent (case-normalized to prevent key splits).
	mu := b.getMutex(strings.ToLower(agentName))
	mu.Lock()
	defer mu.Unlock()

	ctx := context.Background() // 10 min timeout applied per-cmd by temenos daemon

	// Execute all cmds.
	results := b.executeCmds(ctx, agentName, policy, cmds)

	// Format and truncate.
	payload := formatResults(results)
	if len(payload) > maxPayloadBytes {
		truncated := len(payload) - maxPayloadBytes
		marker := fmt.Sprintf("[truncated %d bytes]\n", truncated)
		// Guard: if marker itself exceeds the limit, keep just the payload tail.
		if len(marker) > maxPayloadBytes {
			payload = payload[len(payload)-maxPayloadBytes:]
		} else {
			payload = marker + payload[len(payload)-maxPayloadBytes+len(marker):]
		}
	}

	// Deliver via tmux send-keys.
	session := config.AgentSessionName(agentName)
	if err := tmux.SendKeys(session, agentName, payload); err != nil {
		log.Printf("[cmdexec] SendKeys failed for %s: %v", agentName, err)
	}
}

// buildSandboxEnv constructs the env map forwarded to the temenos sandbox.
func (b *cmdexecBridge) buildSandboxEnv(agentName string) map[string]string {
	var envParts []string
	if b.cfg != nil {
		envParts = buildManagerAgentEnv(agentName, b.cfg)
	}
	if home := os.Getenv("HOME"); home != "" {
		envParts = append(envParts, "HOME="+home)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		envParts = append(envParts, "XDG_CONFIG_HOME="+xdg)
	}
	return envpkg.EnvSliceToMap(envParts)
}

// executeCmds runs each command via the logos runner and returns formatted results.
// Commands matching the recursion guard return a synthetic error result.
func (b *cmdexecBridge) executeCmds(
	ctx context.Context,
	agentName string,
	policy []logos.AllowedPath,
	cmds []string,
) []string {
	results := make([]string, 0, len(cmds))
	sandboxEnv := b.buildSandboxEnv(agentName)
	for _, cmd := range cmds {
		if recursionGuard.MatchString(cmd) {
			results = append(results, formatOneResult(cmd, "", "ttal go forbidden in cmd block — route via persist channel", -1))
			continue
		}

		resp, err := b.runner.Run(ctx, logos.RunRequest{
			Command:      cmd,
			Env:          sandboxEnv,
			AllowedPaths: policy,
		})
		if err != nil {
			results = append(results, formatOneResult(cmd, "", fmt.Sprintf("execution error: %v", err), -1))
			continue
		}

		output := resp.Stdout
		if resp.Stderr != "" {
			output = output + "\nSTDERR:\n" + resp.Stderr
		}
		if output == "" {
			output = "(no output)"
		}
		results = append(results, formatOneResult(cmd, output, "", resp.ExitCode))
	}
	return results
}

// formatOneResult formats a single command result in logos format:
// <cmd-verbatim>\n<output>
// (exit code: N) if exit != 0 && != -1
func formatOneResult(cmd, output, errMsg string, exitCode int) string {
	var b strings.Builder
	b.WriteString(cmd)
	if errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errMsg)
	} else if output != "" {
		b.WriteString("\n")
		b.WriteString(output)
		if exitCode != 0 && exitCode != -1 {
			fmt.Fprintf(&b, "\n(exit code: %d)", exitCode)
		}
	} else {
		// No output — exit code on its own line if non-zero.
		if exitCode != 0 && exitCode != -1 {
			fmt.Fprintf(&b, "\n(exit code: %d)", exitCode)
		}
	}
	return b.String()
}

// formatResults wraps the results slice in a single <result> block.
func formatResults(results []string) string {
	if len(results) == 0 {
		return ""
	}
	return "<result>\n" + strings.Join(results, "\n") + "\n</result>"
}
