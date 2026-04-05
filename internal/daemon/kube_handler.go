package daemon

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/project"
)

// HandleKubeLog returns a handler that fetches pod logs via kubectl.
func HandleKubeLog(store *project.Store, kubeCtx string, allowedNS []string) func(KubeLogRequest) KubeLogResponse {
	return func(req KubeLogRequest) KubeLogResponse {
		// Resolve project
		proj, err := store.Get(req.Alias)
		if err != nil {
			return KubeLogResponse{OK: false, Error: fmt.Sprintf("failed to get project: %v", err)}
		}
		if proj == nil {
			return KubeLogResponse{OK: false, Error: fmt.Sprintf("project %q not found", req.Alias)}
		}

		// Guard: both k8s fields must be configured
		if proj.K8sApp == "" || proj.K8sNamespace == "" {
			return KubeLogResponse{
				OK: false,
				Error: fmt.Sprintf(
					`project %q has incomplete k8s config (k8s_app=%q, k8s_namespace=%q) — both required`,
					req.Alias, proj.K8sApp, proj.K8sNamespace),
			}
		}

		// Guard: allowlist must be configured
		if len(allowedNS) == 0 {
			return KubeLogResponse{
				OK:    false,
				Error: "no namespaces configured in kubernetes.allowed_namespaces — add to config.toml",
			}
		}

		// Namespace validation
		if !isNamespaceAllowed(proj.K8sNamespace, allowedNS) {
			return KubeLogResponse{
				OK:    false,
				Error: fmt.Sprintf("namespace %q is not allowed (allowed: %s)", proj.K8sNamespace, strings.Join(allowedNS, ", ")),
			}
		}

		// Build kubectl command
		args := buildKubectlArgs(proj.K8sApp, proj.K8sNamespace, kubeCtx, req.Tail, req.Since)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "kubectl", args...)
		out, err := cmd.CombinedOutput()

		if ctx.Err() == context.DeadlineExceeded {
			return KubeLogResponse{OK: false, Error: "kubectl logs timed out after 30s"}
		}
		if err != nil {
			return KubeLogResponse{OK: false, Error: fmt.Sprintf("kubectl logs failed: %s", string(out))}
		}

		return KubeLogResponse{OK: true, Logs: string(out)}
	}
}

// isNamespaceAllowed checks if namespace is in the allowlist.
func isNamespaceAllowed(ns string, allowlist []string) bool {
	for _, allowed := range allowlist {
		if ns == allowed {
			return true
		}
	}
	return false
}

// buildKubectlArgs constructs the kubectl logs arguments.
func buildKubectlArgs(app, namespace, kubeCtx string, tail int, since string) []string {
	if tail <= 0 {
		tail = 100
	}
	args := []string{
		"logs",
		"-l", fmt.Sprintf("app.kubernetes.io/name=%s", app),
		"-n", namespace,
		"--context", kubeCtx,
		"--tail", fmt.Sprintf("%d", tail),
	}
	if since != "" {
		args = append(args, "--since="+since)
	}
	return args
}
