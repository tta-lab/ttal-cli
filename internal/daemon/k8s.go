package daemon

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
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
func (k *k8sTeamPod) EnsureNamespace() error {
	if err := k.kubectl("get", "namespace", k.namespace); err == nil {
		return nil
	}
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
func (k *k8sTeamPod) WaitForReady() error {
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		phase := k.getPodField("status.phase")
		if phase == "Running" {
			log.Printf("[k8s] pod %s is Running", k.podName())
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("pod %s not ready after 120s", k.podName())
}

// SpawnAgent creates a tmux session inside the pod for an agent.
// Per-agent env vars are set via `env KEY=VAL` prefix so the claude process sees them immediately.
func (k *k8sTeamPod) SpawnAgent(agentName, model string, perAgentEnv []string) error {
	ccCmd := "claude --dangerously-skip-permissions"
	if model != "" {
		ccCmd += " --model " + model
	}

	envPrefix := ""
	if len(perAgentEnv) > 0 {
		envPrefix = "env " + strings.Join(perAgentEnv, " ") + " "
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
	allArgs := []string{"--context", k.kubectx, "-n", k.namespace, "exec", pod, "--"}
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
func (k *k8sTeamPod) getSpecHash() string {
	// Check if pod exists first
	checkArgs := []string{"--context", k.kubectx, "-n", k.namespace, "get", "pod", k.podName()}
	if err := exec.Command("kubectl", checkArgs...).Run(); err != nil {
		return ""
	}
	allArgs := []string{
		"--context", k.kubectx, "-n", k.namespace,
		"get", "pod", k.podName(),
		"-o", `go-template={{index .metadata.labels "ttal.io/spec-hash"}}`,
	}
	out, err := exec.Command("kubectl", allArgs...).Output()
	if err != nil {
		return ""
	}
	result := strings.TrimSpace(string(out))
	if result == "<no value>" {
		return ""
	}
	return result
}

// injectSpecHash inserts the ttal.io/spec-hash label into the pod YAML.
// The YAML is generated WITHOUT this label first (to compute the hash), then the label is injected.
func (k *k8sTeamPod) injectSpecHash(yaml, hash string) string {
	return strings.Replace(yaml,
		"    app: ttal-team",
		fmt.Sprintf("    app: ttal-team\n    ttal.io/spec-hash: %s", hash),
		1)
}

// specHash computes a 12-char hex SHA-256 of the YAML content.
func specHash(yaml string) string {
	h := sha256.Sum256([]byte(yaml))
	return fmt.Sprintf("%x", h[:6])
}

// generatePodYAML generates the pod manifest YAML (without the spec-hash label).
func (k *k8sTeamPod) generatePodYAML(sharedEnv []string, volumes []k8sVolume) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    ttal.io/team: %s
    app: ttal-team
spec:
  restartPolicy: Always
  containers:
  - name: agent
    image: %s
    command: ["sleep", "infinity"]
`, k.podName(), k.namespace, k.teamName, k.image))

	// Env vars
	if len(sharedEnv) > 0 {
		sb.WriteString("    env:\n")
		for _, e := range sharedEnv {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) != 2 {
				continue
			}
			sb.WriteString(fmt.Sprintf("    - name: %s\n      value: %s\n", parts[0], yamlQuote(parts[1])))
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
			sb.WriteString(fmt.Sprintf("    - name: %s\n      mountPath: %s\n      readOnly: %s\n",
				v.Name, v.ContainerPath, readonly))
		}
	}

	// Volumes (hostPath)
	if len(volumes) > 0 {
		sb.WriteString("  volumes:\n")
		for _, v := range volumes {
			sb.WriteString(fmt.Sprintf("  - name: %s\n    hostPath:\n      path: %s\n      type: %s\n",
				v.Name, v.HostPath, v.Type))
		}
	}

	return sb.String()
}

// yamlQuote wraps a string in double quotes with minimal YAML escaping.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// buildVolumes constructs the hostPath volume slice for a team pod.
func buildVolumes(team *config.ResolvedTeam, home string) []k8sVolume {
	taskrc := filepath.Join(home, ".taskrc")
	if team.TaskRC != "" {
		taskrc = team.TaskRC // already expanded in ResolvedTeam
	}
	teamPath := team.TeamPath // already expanded

	vols := []k8sVolume{
		{Name: "claude-config", HostPath: filepath.Join(home, ".claude"), ContainerPath: "/home/node/.claude", Type: "Directory"},
		{Name: "ttal-config", HostPath: filepath.Join(home, ".config", "ttal"), ContainerPath: "/home/node/.config/ttal", ReadOnly: true, Type: "Directory"},
		{Name: "daemon-sock", HostPath: filepath.Join(home, ".ttal", "daemon.sock"), ContainerPath: "/home/node/.ttal/daemon.sock", Type: "Socket"},
		{Name: "ssh", HostPath: filepath.Join(home, ".ssh"), ContainerPath: "/home/node/.ssh", ReadOnly: true, Type: "Directory"},
		{Name: "taskrc", HostPath: taskrc, ContainerPath: "/home/node/.taskrc", ReadOnly: true, Type: "File"},
		{Name: "taskdata", HostPath: filepath.Join(home, ".task"), ContainerPath: "/home/node/.task", Type: "Directory"},
		{Name: "team-workspace", HostPath: teamPath, ContainerPath: "/workspace", Type: "Directory"},
	}

	// Mount CLI binaries — skip if not installed
	for _, bin := range []struct{ name, mountPath string }{
		{"ttal", "/usr/local/bin/ttal"},
		{"flicknote", "/usr/local/bin/flicknote"},
		{"diary", "/usr/local/bin/diary"},
	} {
		path := whichBin(bin.name)
		if path == "" {
			continue
		}
		vols = append(vols, k8sVolume{
			Name:          bin.name + "-bin",
			HostPath:      path,
			ContainerPath: bin.mountPath,
			ReadOnly:      true,
			Type:          "File",
		})
	}

	return vols
}

// whichBin returns the full path to a binary, or "" if not found.
func whichBin(name string) string {
	path, _ := exec.LookPath(name)
	return path
}
