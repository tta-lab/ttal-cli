package daemon

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// k8sVolume defines a hostPath volume mount for the team pod.
type k8sVolume struct {
	Name          string
	HostPath      string
	ContainerPath string
	ReadOnly      bool
	Type          string // "File", "Directory", "Socket"
}

// k8sTeamPod manages a single team pod with multiple agent tmux sessions inside.
// One pod per team — agents are tmux sessions within the pod.
type k8sTeamPod struct {
	kubectx   string
	namespace string // default: "ttal"
	image     string
	teamName  string
}

// podName returns the pod name: ttal-<team>.
func (k *k8sTeamPod) podName() string {
	return fmt.Sprintf("ttal-%s", k.teamName)
}

// EnsureNamespace creates the ttal namespace if it doesn't exist.
// Logs the get failure reason before attempting create, since the failure could indicate
// an RBAC or context misconfiguration rather than a missing namespace.
func (k *k8sTeamPod) EnsureNamespace() error {
	out, err := k.kubectlOutput("get", "namespace", k.namespace)
	if err == nil {
		return nil
	}
	log.Printf("[k8s] namespace %s get failed (%s) — attempting create", k.namespace, strings.TrimSpace(out))
	return k.kubectl("create", "namespace", k.namespace)
}

// EnsurePod creates or reuses the team pod based on spec-hash comparison.
// If the desired spec matches the running pod's spec-hash label, the pod is reused.
// Otherwise (spec changed or pod missing), the pod is deleted and recreated.
func (k *k8sTeamPod) EnsurePod(sharedEnv []string, volumes []k8sVolume) error {
	yaml := k.generatePodYAML(sharedEnv, volumes)
	desiredHash := specHash(yaml)

	currentHash := k.getSpecHash()
	if currentHash == desiredHash {
		log.Printf("[k8s] pod %s spec unchanged — reusing", k.podName())
		return nil
	}
	if currentHash != "" {
		log.Printf("[k8s] pod %s spec changed (%s → %s) — recreating", k.podName(), currentHash, desiredHash)
		if err := k.DeleteTeamPod(); err != nil {
			return fmt.Errorf("delete pod: %w", err)
		}
	}

	yamlWithHash := k.injectSpecHash(yaml, desiredHash)
	if err := k.kubectlApply(yamlWithHash); err != nil {
		return fmt.Errorf("apply pod: %w", err)
	}
	return k.WaitForReady()
}

// WaitForReady polls until the pod reaches Running phase (timeout 120s).
// Returns immediately if the pod enters a terminal failure state.
func (k *k8sTeamPod) WaitForReady() error {
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		phase := k.getPodField("status.phase")
		if phase == "Running" {
			log.Printf("[k8s] pod %s is Running", k.podName())
			return nil
		}
		if phase == "Failed" || phase == "Succeeded" {
			reason := k.getPodField("status.reason")
			msg := k.getPodField("status.message")
			return fmt.Errorf("pod %s entered terminal state %s: %s %s", k.podName(), phase, reason, msg)
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("pod %s not ready after 120s", k.podName())
}

// SpawnAgent creates a tmux session inside the pod for an agent.
// Per-agent env vars are shell-quoted and passed via `env KEY=VAL` prefix.
// Uses `cd /workspace/<agent>` so Claude Code creates per-agent JSONL dirs.
func (k *k8sTeamPod) SpawnAgent(agentName, model string, perAgentEnv []string) error {
	ccCmd := fmt.Sprintf("cd /workspace/%s && claude --dangerously-skip-permissions", shellQuote(agentName))
	if model != "" {
		ccCmd += " --model " + shellQuote(model)
	}

	envPrefix := ""
	if len(perAgentEnv) > 0 {
		quoted := make([]string, len(perAgentEnv))
		for i, e := range perAgentEnv {
			quoted[i] = shellQuoteEnvPair(e)
		}
		envPrefix = "env " + strings.Join(quoted, " ") + " "
	}
	fullCmd := envPrefix + ccCmd

	return k.kubectlExec(k.podName(), "tmux", "new-session", "-d", "-s", agentName, fullCmd)
}

// SendKeys sends text to an agent's tmux session inside the pod.
func (k *k8sTeamPod) SendKeys(agentName, text string) error {
	return k.kubectlExec(k.podName(), "tmux", "send-keys", "-t", agentName, text, "Enter")
}

// StopAgent sends /exit to an agent's tmux session.
func (k *k8sTeamPod) StopAgent(agentName string) error {
	return k.kubectlExec(k.podName(), "tmux", "send-keys", "-t", agentName, "/exit", "Enter")
}

// SessionExists checks if an agent tmux session exists in the pod.
func (k *k8sTeamPod) SessionExists(agentName string) bool {
	err := k.kubectlExec(k.podName(), "tmux", "has-session", "-t", agentName)
	return err == nil
}

// DeleteTeamPod removes the entire team pod.
func (k *k8sTeamPod) DeleteTeamPod() error {
	return k.kubectl("delete", "pod", k.podName(), "--ignore-not-found")
}

// kubectlExec runs a command inside the pod via kubectl exec.
func (k *k8sTeamPod) kubectlExec(pod string, args ...string) error {
	allArgs := make([]string, 0, 7+len(args))
	allArgs = append(allArgs, "--context", k.kubectx, "-n", k.namespace, "exec", pod, "--")
	allArgs = append(allArgs, args...)
	cmd := exec.Command("kubectl", allArgs...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// kubectl runs a kubectl command with context and namespace set.
func (k *k8sTeamPod) kubectl(args ...string) error {
	allArgs := append([]string{"--context", k.kubectx, "-n", k.namespace}, args...)
	cmd := exec.Command("kubectl", allArgs...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// kubectlOutput runs a kubectl command and returns combined stdout+stderr output.
func (k *k8sTeamPod) kubectlOutput(args ...string) (string, error) {
	allArgs := append([]string{"--context", k.kubectx, "-n", k.namespace}, args...)
	out, err := exec.Command("kubectl", allArgs...).CombinedOutput()
	return string(out), err
}

// kubectlApply applies a YAML manifest via stdin.
func (k *k8sTeamPod) kubectlApply(yamlContent string) error {
	allArgs := []string{"--context", k.kubectx, "-n", k.namespace, "apply", "-f", "-"}
	cmd := exec.Command("kubectl", allArgs...)
	cmd.Stdin = strings.NewReader(yamlContent)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getPodField reads a simple jsonpath field from the pod spec.
func (k *k8sTeamPod) getPodField(jsonpath string) string {
	allArgs := []string{
		"--context", k.kubectx, "-n", k.namespace,
		"get", "pod", k.podName(),
		"-o", fmt.Sprintf("jsonpath={.%s}", jsonpath),
	}
	out, err := exec.Command("kubectl", allArgs...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getSpecHash retrieves the ttal.io/spec-hash label from the running pod.
// Returns "" if the pod doesn't exist or the label is unset.
// Logs a warning if the failure is not a simple "not found" (e.g., RBAC or wrong context).
func (k *k8sTeamPod) getSpecHash() string {
	checkArgs := []string{"--context", k.kubectx, "-n", k.namespace, "get", "pod", k.podName()}
	out, err := exec.Command("kubectl", checkArgs...).CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(out))
		// Log unexpected errors — plain "not found" is expected and not worth logging
		if output != "" && !strings.Contains(output, "NotFound") && !strings.Contains(output, "not found") {
			log.Printf("[k8s] warning: pod %s existence check failed: %s", k.podName(), output)
		}
		return ""
	}
	allArgs := []string{
		"--context", k.kubectx, "-n", k.namespace,
		"get", "pod", k.podName(),
		"-o", `go-template={{index .metadata.labels "ttal.io/spec-hash"}}`,
	}
	labelOut, err := exec.Command("kubectl", allArgs...).Output()
	if err != nil {
		return ""
	}
	result := strings.TrimSpace(string(labelOut))
	if result == "<no value>" {
		return ""
	}
	return result
}

// injectSpecHash inserts the ttal.io/spec-hash label into the pod YAML.
// The YAML is generated WITHOUT this label first (to compute the hash), then the label is injected.
// Logs a warning if the injection marker is not found (indicates YAML format changed).
func (k *k8sTeamPod) injectSpecHash(yaml, hash string) string {
	const marker = "    app: ttal-team\n"
	replacement := marker + "    ttal.io/spec-hash: " + hash + "\n"
	result := strings.Replace(yaml, marker, replacement, 1)
	if result == yaml {
		log.Printf("[k8s] warning: could not inject spec-hash into pod YAML for %s — hash won't be stored", k.podName())
	}
	return result
}

// specHash computes a 12-char hex SHA-256 of the YAML content.
func specHash(yaml string) string {
	h := sha256.Sum256([]byte(yaml))
	return fmt.Sprintf("%x", h[:6])
}

// generatePodYAML generates the pod manifest YAML (without the spec-hash label).
func (k *k8sTeamPod) generatePodYAML(sharedEnv []string, volumes []k8sVolume) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "apiVersion: v1\nkind: Pod\nmetadata:\n  name: %s\n  namespace: %s\n"+
		"  labels:\n    ttal.io/team: %s\n    app: ttal-team\nspec:\n  restartPolicy: Always\n"+
		"  containers:\n  - name: agent\n    image: %s\n    command: [\"sleep\", \"infinity\"]\n",
		k.podName(), k.namespace, k.teamName, k.image)

	// Env vars
	if len(sharedEnv) > 0 {
		sb.WriteString("    env:\n")
		for _, e := range sharedEnv {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) != 2 {
				continue
			}
			fmt.Fprintf(&sb, "    - name: %s\n      value: %s\n", parts[0], yamlQuote(parts[1]))
		}
	}

	// Volume mounts
	if len(volumes) > 0 {
		sb.WriteString("    volumeMounts:\n")
		for _, v := range volumes {
			readonly := "false"
			if v.ReadOnly {
				readonly = "true"
			}
			fmt.Fprintf(&sb, "    - name: %s\n      mountPath: %s\n      readOnly: %s\n",
				v.Name, v.ContainerPath, readonly)
		}
	}

	// Volumes (hostPath)
	if len(volumes) > 0 {
		sb.WriteString("  volumes:\n")
		for _, v := range volumes {
			fmt.Fprintf(&sb, "  - name: %s\n    hostPath:\n      path: %s\n      type: %s\n",
				v.Name, v.HostPath, v.Type)
		}
	}

	return sb.String()
}

// yamlQuote wraps a string in double quotes with YAML double-quoted scalar escaping.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return `"` + s + `"`
}

// shellQuote wraps a string in single quotes for POSIX shell, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellQuoteEnvPair quotes the value half of a KEY=VALUE env pair for shell interpolation.
func shellQuoteEnvPair(kv string) string {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return kv
	}
	return parts[0] + "=" + shellQuote(parts[1])
}

// bootstrapClaudeDir ensures ~/.ttal/<team>/.claude/ exists with seeded config.
// Called once before EnsurePod. Idempotent — only creates/copies if missing.
func bootstrapClaudeDir(teamName, teamPath, home string) error {
	claudeDir := filepath.Join(home, ".ttal", teamName, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	// Seed settings.json and settings.local.json from host if not present
	for _, name := range []string{"settings.json", "settings.local.json"} {
		src := filepath.Join(home, ".claude", name)
		dst := filepath.Join(claudeDir, name)
		switch _, err := os.Stat(dst); {
		case err == nil:
			// already exists, skip
		case os.IsNotExist(err):
			data, rerr := os.ReadFile(src)
			if rerr != nil {
				log.Printf("[k8s] warning: could not read %s for team %s: %v", name, teamName, rerr)
				continue
			}
			if werr := os.WriteFile(dst, data, 0o644); werr != nil {
				log.Printf("[k8s] warning: could not seed %s for team %s: %v", name, teamName, werr)
			}
		default:
			log.Printf("[k8s] warning: could not stat %s for team %s: %v", dst, teamName, err)
		}
	}

	// Ensure subdirs exist for ttal sync and watcher
	for _, sub := range []string{"skills", "agents", "rules", "projects"} {
		if err := os.MkdirAll(filepath.Join(claudeDir, sub), 0o755); err != nil {
			log.Printf("[k8s] warning: could not create subdir %s for team %s: %v", sub, teamName, err)
		}
	}

	// Discover agents once — used for .claude.json trust entries and JSONL project dirs
	agents, discErr := agentfs.DiscoverAgents(teamPath)
	if discErr != nil {
		log.Printf("[k8s] warning: could not discover agents for %s: %v", teamName, discErr)
	}

	// Seed ~/.ttal/<team>/.claude.json with onboarding + project trust
	claudeJSON := filepath.Join(home, ".ttal", teamName, ".claude.json")
	switch _, err := os.Stat(claudeJSON); {
	case os.IsNotExist(err):
		data := buildClaudeJSON(agents)
		if werr := os.WriteFile(claudeJSON, data, 0o644); werr != nil {
			log.Printf("[k8s] warning: could not seed .claude.json for team %s: %v", teamName, werr)
		}
	case err == nil:
		// File exists — ensure any new agents get trust entries added
		ensureAgentTrust(claudeJSON, agents)
	}

	// Pre-create per-agent JSONL project dirs for watcher
	projectsDir := filepath.Join(claudeDir, "projects")
	for _, name := range agents {
		encoded := watcher.EncodePath(filepath.Join("/workspace", name))
		if err := os.MkdirAll(filepath.Join(projectsDir, encoded), 0o700); err != nil {
			log.Printf("[k8s] warning: could not create JSONL project dir for %s/%s: %v", teamName, name, err)
		}
	}

	return nil
}

// buildClaudeJSON creates a minimal ~/.claude.json for k8s pods.
func buildClaudeJSON(agentNames []string) []byte {
	type projectEntry struct {
		HasTrustDialogAccepted     bool     `json:"hasTrustDialogAccepted"`
		HasCompletedProjectOnboard bool     `json:"hasCompletedProjectOnboarding"`
		AllowedTools               []string `json:"allowedTools"`
	}

	type claudeJSONStruct struct {
		HasCompletedOnboarding bool                    `json:"hasCompletedOnboarding"`
		Projects               map[string]projectEntry `json:"projects"`
	}

	cfg := claudeJSONStruct{
		HasCompletedOnboarding: true,
		Projects:               make(map[string]projectEntry),
	}

	for _, name := range agentNames {
		cfg.Projects["/workspace/"+name] = projectEntry{
			HasTrustDialogAccepted:     true,
			HasCompletedProjectOnboard: true,
			AllowedTools:               []string{},
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("[k8s] warning: could not marshal .claude.json: %v", err)
		return []byte("{}")
	}
	return data
}

// ensureAgentTrust adds trust entries for newly discovered agents to an existing .claude.json.
func ensureAgentTrust(claudeJSONPath string, agents []string) {
	keys := make([]string, len(agents))
	for i, name := range agents {
		keys[i] = "/workspace/" + name
	}
	if _, err := upsertClaudeJSONTrust(claudeJSONPath, keys); err != nil {
		log.Printf("[k8s] warning: could not update agent trust in %s: %v", claudeJSONPath, err)
	}
}

// upsertClaudeJSONTrust reads (or creates) a .claude.json file and ensures all
// given project paths have trust entries. Returns count of added entries.
func upsertClaudeJSONTrust(claudeJSONPath string, projectPaths []string) (int, error) {
	if len(projectPaths) == 0 {
		return 0, nil
	}

	var raw map[string]any
	data, rerr := os.ReadFile(claudeJSONPath)
	if rerr != nil && !os.IsNotExist(rerr) {
		return 0, fmt.Errorf("read %s: %w", claudeJSONPath, rerr)
	}
	if rerr == nil {
		if uerr := json.Unmarshal(data, &raw); uerr != nil {
			return 0, fmt.Errorf("parse %s: %w", claudeJSONPath, uerr)
		}
	}
	if raw == nil {
		raw = map[string]any{"hasCompletedOnboarding": true}
	}

	projects, _ := raw["projects"].(map[string]any)
	if projects == nil {
		projects = make(map[string]any)
		raw["projects"] = projects
	}

	added := 0
	for _, path := range projectPaths {
		if proj, exists := projects[path]; exists {
			if m, ok := proj.(map[string]any); ok && m["hasTrustDialogAccepted"] == true {
				continue
			}
		}
		projects[path] = newProjectTrustEntry()
		added++
	}

	if added == 0 {
		return 0, nil
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshal %s: %w", claudeJSONPath, err)
	}
	if err := os.WriteFile(claudeJSONPath, out, 0o644); err != nil {
		return 0, fmt.Errorf("write %s: %w", claudeJSONPath, err)
	}
	return added, nil
}

// newProjectTrustEntry returns a trust entry map for a CC project.
func newProjectTrustEntry() map[string]any {
	return map[string]any{
		"hasTrustDialogAccepted":        true,
		"hasCompletedProjectOnboarding": true,
		"allowedTools":                  []any{},
	}
}

// buildVolumes constructs the hostPath volume slice for a team pod.
// Uses the team-isolated ~/.ttal/<teamName>/.claude for the claude-config volume
// so each team's credentials and JSONL logs are fully isolated.
func buildVolumes(team *config.ResolvedTeam, teamName, home string) []k8sVolume {
	taskrc := filepath.Join(home, ".taskrc")
	if team.TaskRC != "" {
		taskrc = team.TaskRC // already expanded in ResolvedTeam
	}
	teamPath := team.TeamPath // already expanded

	return []k8sVolume{
		{
			Name: "claude-config", HostPath: filepath.Join(home, ".ttal", teamName, ".claude"),
			ContainerPath: "/home/agent/.claude", Type: "Directory",
		},
		{
			Name: "claude-json", HostPath: filepath.Join(home, ".ttal", teamName, ".claude.json"),
			ContainerPath: "/home/agent/.claude.json", Type: "File",
		},
		{
			Name: "ttal-config", HostPath: filepath.Join(home, ".config", "ttal"),
			ContainerPath: "/home/agent/.config/ttal", ReadOnly: true, Type: "Directory",
		},
		{
			Name: "ttal-home", HostPath: filepath.Join(home, ".ttal"),
			ContainerPath: "/home/agent/.ttal", Type: "Directory",
		},
		{
			Name: "ssh", HostPath: filepath.Join(home, ".ssh"),
			ContainerPath: "/home/agent/.ssh", ReadOnly: true, Type: "Directory",
		},
		{Name: "taskrc", HostPath: taskrc, ContainerPath: "/home/agent/.taskrc", ReadOnly: true, Type: "File"},
		{Name: "taskdata", HostPath: filepath.Join(home, ".task"), ContainerPath: "/home/agent/.task", Type: "Directory"},
		{Name: "team-workspace", HostPath: teamPath, ContainerPath: "/workspace", Type: "Directory"},
	}
}
