package daemon

import (
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/project"
)

func testKubeStore(t *testing.T) *project.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.toml")
	return project.NewStore(path)
}

func TestKubeHandlerIncompleteK8sConfig(t *testing.T) {
	store := testKubeStore(t)
	if err := store.Add("proj", "Project", "/path"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	handler := HandleKubeLog(store, "do-sgp1", []string{"apps-dev", "supa-dev"})

	// No k8s fields set
	resp := handler(KubeLogRequest{Alias: "proj", Tail: 100})
	if resp.OK {
		t.Error("handler should return error for project without k8s fields")
	}
	if resp.Error != `project "proj" has incomplete k8s config (k8s_app="", k8s_namespace="") — both required` {
		t.Errorf("unexpected error: %q", resp.Error)
	}

	// Only k8s_app set
	if err := store.Modify("proj", map[string]string{"k8s_app": "my-api"}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}
	resp = handler(KubeLogRequest{Alias: "proj", Tail: 100})
	if resp.OK {
		t.Error("handler should return error for project without namespace")
	}
}

func TestKubeHandlerEmptyNamespace(t *testing.T) {
	store := testKubeStore(t)
	if err := store.Add("proj", "Project", "/path"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := store.Modify("proj", map[string]string{"k8s_app": "my-api", "k8s_namespace": ""}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	handler := HandleKubeLog(store, "do-sgp1", []string{"apps-dev", "supa-dev"})
	resp := handler(KubeLogRequest{Alias: "proj", Tail: 100})

	if resp.OK {
		t.Error("handler should return error for missing namespace")
	}
}

func TestKubeHandlerEmptyAllowlist(t *testing.T) {
	store := testKubeStore(t)
	if err := store.Add("proj", "Project", "/path"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := store.Modify("proj", map[string]string{"k8s_app": "my-api", "k8s_namespace": "apps-dev"}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	handler := HandleKubeLog(store, "do-sgp1", []string{})
	resp := handler(KubeLogRequest{Alias: "proj", Tail: 100})

	if resp.OK {
		t.Error("handler should return error for empty allowlist")
	}
	if resp.Error != "no namespaces configured in kubernetes.allowed_namespaces — add to config.toml" {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

func TestKubeHandlerProjectNotFound(t *testing.T) {
	store := testKubeStore(t)

	handler := HandleKubeLog(store, "do-sgp1", []string{"apps-dev", "supa-dev"})
	resp := handler(KubeLogRequest{Alias: "nonexistent", Tail: 100})

	if resp.OK {
		t.Error("handler should return error for nonexistent project")
	}
	if resp.Error != `project "nonexistent" not found` {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

func TestKubeHandlerNamespaceNotAllowed(t *testing.T) {
	store := testKubeStore(t)
	if err := store.Add("proj", "Project", "/path"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := store.Modify("proj", map[string]string{"k8s_app": "my-api", "k8s_namespace": "production"}); err != nil {
		t.Fatalf("Modify() error: %v", err)
	}

	handler := HandleKubeLog(store, "do-sgp1", []string{"apps-dev", "supa-dev"})
	resp := handler(KubeLogRequest{Alias: "proj", Tail: 100})

	if resp.OK {
		t.Error("handler should return error for disallowed namespace")
	}
	if resp.Error != `namespace "production" is not allowed (allowed: apps-dev, supa-dev)` {
		t.Errorf("unexpected error: %q", resp.Error)
	}
}

func TestNamespaceAllowlistValidation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		allowlist []string
		want      bool
	}{
		{
			name:      "exact match allowed",
			namespace: "apps-dev",
			allowlist: []string{"apps-dev", "supa-dev"},
			want:      true,
		},
		{
			name:      "second namespace allowed",
			namespace: "supa-dev",
			allowlist: []string{"apps-dev", "supa-dev"},
			want:      true,
		},
		{
			name:      "non-match rejected",
			namespace: "production",
			allowlist: []string{"apps-dev", "supa-dev"},
			want:      false,
		},
		{
			name:      "empty namespace rejected",
			namespace: "",
			allowlist: []string{"apps-dev", "supa-dev"},
			want:      false,
		},
		{
			name:      "empty allowlist rejects all",
			namespace: "apps-dev",
			allowlist: []string{},
			want:      false,
		},
		{
			name:      "case-sensitive match",
			namespace: "Apps-Dev",
			allowlist: []string{"apps-dev"},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNamespaceAllowed(tt.namespace, tt.allowlist)
			if got != tt.want {
				t.Errorf("isNamespaceAllowed(%q, %v) = %v, want %v",
					tt.namespace, tt.allowlist, got, tt.want)
			}
		})
	}
}

func TestKubectlCommandConstruction(t *testing.T) {
	tests := []struct {
		name      string
		app       string
		namespace string
		context   string
		tail      int
		since     string
		want      []string
	}{
		{
			name:      "default tail",
			app:       "my-api",
			namespace: "apps-dev",
			context:   "do-sgp1",
			tail:      0,
			since:     "",
			want: []string{
				"logs", "-l", "app.kubernetes.io/name=my-api",
				"-n", "apps-dev", "--context", "do-sgp1", "--tail", "100",
			},
		},
		{
			name:      "custom tail",
			app:       "my-api",
			namespace: "apps-dev",
			context:   "do-sgp1",
			tail:      500,
			since:     "",
			want: []string{
				"logs", "-l", "app.kubernetes.io/name=my-api",
				"-n", "apps-dev", "--context", "do-sgp1", "--tail", "500",
			},
		},
		{
			name:      "with since",
			app:       "my-api",
			namespace: "apps-dev",
			context:   "do-sgp1",
			tail:      100,
			since:     "5m",
			want: []string{
				"logs", "-l", "app.kubernetes.io/name=my-api",
				"-n", "apps-dev", "--context", "do-sgp1",
				"--tail", "100", "--since=5m",
			},
		},
		{
			name:      "with since no tail flag default",
			app:       "my-api",
			namespace: "supa-dev",
			context:   "do-sgp1",
			tail:      0,
			since:     "10m",
			want: []string{
				"logs", "-l", "app.kubernetes.io/name=my-api",
				"-n", "supa-dev", "--context", "do-sgp1",
				"--tail", "100", "--since=10m",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKubectlArgs(tt.app, tt.namespace, tt.context, tt.tail, tt.since)
			if len(got) != len(tt.want) {
				t.Errorf("buildKubectlArgs() len = %d, want %d\n  got:  %v\n  want: %v",
					len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildKubectlArgs()[%d] = %q, want %q\n  got:  %v\n  want: %v",
						i, got[i], tt.want[i], got, tt.want)
					return
				}
			}
		})
	}
}
