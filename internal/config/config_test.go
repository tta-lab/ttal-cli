package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestAgentSessionName(t *testing.T) {
	tests := []struct {
		team  string
		agent string
		want  string
	}{
		{"default", "kestrel", "ttal-default-kestrel"},
		{"default", "yuki", "ttal-default-yuki"},
		{"guion", "mira", "ttal-default-mira"},
		{"sven", "athena", "ttal-default-athena"},
	}
	for _, tt := range tests {
		if got := AgentSessionName(tt.agent); got != tt.want {
			t.Errorf("AgentSessionName(%q) = %q, want %q", tt.agent, got, tt.want)
		}
	}
}

func TestGetShell(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"empty config defaults to zsh", &Config{}, "zsh"},
		{"fish override", &Config{Shell: "fish"}, "fish"},
		{"zsh explicit", &Config{Shell: "zsh"}, "zsh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetShell(); got != tt.want {
				t.Errorf("GetShell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShellCommand(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		cmd     string
		wantCmd string
	}{
		{"zsh default", &Config{}, "echo hello", "zsh -c 'echo hello'"},
		{"fish shell", &Config{Shell: "fish"}, "echo hello", "fish -C 'echo hello'"},
		{"zsh explicit", &Config{Shell: "zsh"}, "echo hello", "zsh -c 'echo hello'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ShellCommand(tt.cmd); got != tt.wantCmd {
				t.Errorf("ShellCommand() = %q, want %q", got, tt.wantCmd)
			}
		})
	}
}

func TestDefaultRuntime(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want runtime.Runtime
	}{
		{"unset defaults to claude-code", &Config{}, runtime.ClaudeCode},
		{"explicit claude-code", &Config{resolvedDefaultRuntime: "claude-code"}, runtime.ClaudeCode},
		{"explicit codex", &Config{resolvedDefaultRuntime: "codex"}, runtime.Codex},
		{"explicit lenos", &Config{resolvedDefaultRuntime: "lenos"}, runtime.Lenos},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.DefaultRuntime(); got != tt.want {
				t.Errorf("DefaultRuntime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultRuntimeRoundTrip(t *testing.T) {
	cfg := &Config{
		DefaultTeam: "default",
		Teams: map[string]TeamConfig{
			"default": {
				TeamPath:       "/tmp/test",
				DefaultRuntime: "lenos",
				ChatID:         "x",
			},
		},
	}
	if err := cfg.resolve(); err != nil {
		t.Fatalf("resolve() failed: %v", err)
	}
	if got := cfg.DefaultRuntime(); got != runtime.Lenos {
		t.Errorf("DefaultRuntime() = %q, want %q", got, runtime.Lenos)
	}
}

func TestGetMergeMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"unset defaults to auto", &Config{}, MergeModeAuto},
		{"explicit auto", &Config{resolvedMergeMode: "auto"}, MergeModeAuto},
		{"explicit manual", &Config{resolvedMergeMode: "manual"}, MergeModeManual},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetMergeMode(); got != tt.want {
				t.Errorf("GetMergeMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeModeResolution(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		want    string
		wantErr bool
	}{
		{
			name: "team sets merge mode",
			cfg: &Config{
				DefaultTeam: "test",
				Teams: map[string]TeamConfig{
					"test": {
						TeamPath:  "/tmp/test",
						MergeMode: "manual",
						ChatID:    "x",
						Agents:    map[string]AgentConfig{"k": {}},
					},
				},
			},
			want: MergeModeManual,
		},
		{
			name: "team empty defaults to auto",
			cfg: &Config{
				DefaultTeam: "test",
				Teams: map[string]TeamConfig{
					"test": {
						TeamPath: "/tmp/test",
						ChatID:   "x",
						Agents:   map[string]AgentConfig{"k": {}},
					},
				},
			},
			want: MergeModeAuto,
		},
		{
			name: "invalid merge_mode rejected",
			cfg: &Config{
				DefaultTeam: "test",
				Teams: map[string]TeamConfig{
					"test": {
						TeamPath:  "/tmp/test",
						MergeMode: "manaul",
						ChatID:    "x",
						Agents:    map[string]AgentConfig{"k": {}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "both empty defaults to auto",
			cfg: &Config{
				DefaultTeam: "test",
				Teams: map[string]TeamConfig{
					"test": {
						TeamPath: "/tmp/test",
						ChatID:   "x",
						Agents:   map[string]AgentConfig{"k": {}},
					},
				},
			},
			want: MergeModeAuto,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.resolve()
			if tt.wantErr {
				if err == nil {
					t.Fatal("resolve() should have returned an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve() error: %v", err)
			}
			if got := tt.cfg.GetMergeMode(); got != tt.want {
				t.Errorf("GetMergeMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlatConfigRejected(t *testing.T) {
	cfg := &Config{Shell: "zsh"}
	err := cfg.resolve()
	if err == nil {
		t.Fatal("resolve() should reject flat config (no teams)")
	}
	if !strings.Contains(err.Error(), "flat config no longer supported") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConventionBasedPaths(t *testing.T) {
	defDataDir := defaultDataDir()
	defTaskRC := defaultTaskRC()

	tests := []struct {
		name         string
		cfg          *Config
		wantDataDir  string
		wantTaskRC   string
		wantTaskData string
	}{
		{
			name: "default team uses traditional paths",
			cfg: &Config{
				DefaultTeam: DefaultTeamName,
				Teams: map[string]TeamConfig{
					DefaultTeamName: {
						TeamPath: "/tmp/agents",
						ChatID:   "x",
						Agents:   map[string]AgentConfig{"k": {}},
					},
				},
			},
			wantDataDir:  defDataDir,
			wantTaskRC:   defTaskRC,
			wantTaskData: defDataDir + "/tasks",
		},
		{
			name: "non-default team uses convention paths",
			cfg: &Config{
				DefaultTeam: "guion",
				Teams: map[string]TeamConfig{
					"guion": {
						TeamPath: "/tmp/agents",
						ChatID:   "x",
						Agents:   map[string]AgentConfig{"k": {}},
					},
				},
			},
			wantDataDir:  defDataDir + "/guion",
			wantTaskRC:   defDataDir + "/guion/taskrc",
			wantTaskData: defDataDir + "/guion/tasks",
		},
		{
			name: "explicit data_dir overrides convention",
			cfg: &Config{
				DefaultTeam: "guion",
				Teams: map[string]TeamConfig{
					"guion": {
						TeamPath: "/tmp/agents",
						DataDir:  "/tmp/custom",
						ChatID:   "x",
						Agents:   map[string]AgentConfig{"k": {}},
					},
				},
			},
			wantDataDir:  "/tmp/custom",
			wantTaskRC:   "/tmp/custom/taskrc",
			wantTaskData: "/tmp/custom/tasks",
		},
		{
			name: "explicit taskrc overrides convention",
			cfg: &Config{
				DefaultTeam: "guion",
				Teams: map[string]TeamConfig{
					"guion": {
						TeamPath: "/tmp/agents",
						TaskRC:   "/tmp/my-taskrc",
						ChatID:   "x",
						Agents:   map[string]AgentConfig{"k": {}},
					},
				},
			},
			wantDataDir:  defDataDir + "/guion",
			wantTaskRC:   "/tmp/my-taskrc",
			wantTaskData: defDataDir + "/guion/tasks",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.resolve(); err != nil {
				t.Fatalf("resolve() error: %v", err)
			}
			if got := tt.cfg.DataDir(); got != tt.wantDataDir {
				t.Errorf("DataDir() = %q, want %q", got, tt.wantDataDir)
			}
			if got := tt.cfg.TaskRC(); got != tt.wantTaskRC {
				t.Errorf("TaskRC() = %q, want %q", got, tt.wantTaskRC)
			}
			if got := tt.cfg.TaskData(); got != tt.wantTaskData {
				t.Errorf("TaskData() = %q, want %q", got, tt.wantTaskData)
			}
		})
	}
}

func TestAgentPath(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		agent    string
		wantPath string
	}{
		{
			"empty team_path returns empty",
			&Config{},
			"kestrel",
			"",
		},
		{
			"normal path joins correctly",
			&Config{resolvedTeamPath: "/home/user/agents"},
			"kestrel",
			"/home/user/agents/kestrel",
		},
		{
			"different agent name",
			&Config{resolvedTeamPath: "/opt/teams/default"},
			"athena",
			"/opt/teams/default/athena",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.AgentPath(tt.agent); got != tt.wantPath {
				t.Errorf("AgentPath(%q) = %q, want %q", tt.agent, got, tt.wantPath)
			}
		})
	}
}

func TestAgentPathFromResolve(t *testing.T) {
	cfg := &Config{
		DefaultTeam: "test",
		Teams: map[string]TeamConfig{
			"test": {
				TeamPath: "~/agents",
				ChatID:   "x",
				Agents:   map[string]AgentConfig{"k": {}},
			},
		},
	}
	if err := cfg.resolve(); err != nil {
		t.Fatalf("resolve() error: %v", err)
	}

	// After resolve, team_path with ~ should be expanded
	got := cfg.AgentPath("kestrel")
	if got == "" {
		t.Fatal("AgentPath() returned empty after resolve with team_path set")
	}
	if strings.Contains(got, "~") {
		t.Errorf("AgentPath() = %q, should not contain tilde after resolve", got)
	}
	if !strings.HasSuffix(got, "/kestrel") {
		t.Errorf("AgentPath() = %q, should end with /kestrel", got)
	}
}

func TestBuildEnvShellCommand(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *Config
		envParts     []string
		cmd          string
		wantContains []string
	}{
		{
			"zsh with env",
			&Config{},
			[]string{"FOO=bar", "BAZ=qux"},
			"echo hello",
			[]string{"env FOO=bar BAZ=qux", "zsh -c", "echo hello"},
		},
		{
			"fish with env",
			&Config{Shell: "fish"},
			[]string{"FOO=bar"},
			"echo hello",
			[]string{"env FOO=bar", "fish -C", "echo hello"},
		},
		{
			"no env parts",
			&Config{},
			nil,
			"echo hello",
			[]string{"zsh -c", "echo hello"},
		},
		{
			"empty env parts",
			&Config{Shell: "fish"},
			[]string{},
			"echo hello",
			[]string{"fish -C", "echo hello"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.BuildEnvShellCommand(tt.envParts, tt.cmd)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildEnvShellCommand() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

func TestRolesConfigHeartbeatPrompt(t *testing.T) {
	data := map[string]interface{}{
		"yuki": map[string]interface{}{
			"prompt":           "You are Yuki.",
			"heartbeat_prompt": "Check in: what are you working on?",
		},
		"inke": map[string]interface{}{
			"prompt": "You are Inke.",
		},
	}
	var r RolesConfig
	if err := r.UnmarshalTOML(data); err != nil {
		t.Fatalf("UnmarshalTOML error: %v", err)
	}
	if got := r.HeartbeatPrompts["yuki"]; got != "Check in: what are you working on?" {
		t.Errorf("HeartbeatPrompts[yuki] = %q, want %q", got, "Check in: what are you working on?")
	}
	if got := r.HeartbeatPrompts["inke"]; got != "" {
		t.Errorf("HeartbeatPrompts[inke] = %q, want empty", got)
	}
}

func TestPromptPlanReview(t *testing.T) {
	cfg := &Config{
		Prompts: PromptsConfig{
			PlanReview:   "review plan {{task-id}}",
			PlanReReview: "re-review plan {{task-id}}",
			PlanTriage:   "triage plan",
		},
	}
	if got := cfg.Prompt("plan_review"); got != "review plan {{task-id}}" {
		t.Errorf("Prompt(plan_review) = %q, want %q", got, "review plan {{task-id}}")
	}
	if got := cfg.Prompt("plan_re_review"); got != "re-review plan {{task-id}}" {
		t.Errorf("Prompt(plan_re_review) = %q, want %q", got, "re-review plan {{task-id}}")
	}
	if got := cfg.Prompt("plan_triage"); got != "triage plan" {
		t.Errorf("Prompt(plan_triage) = %q, want %q", got, "triage plan")
	}
	if !cfg.hasAnyPromptConfigured() {
		t.Error("hasAnyPromptConfigured() = false when PlanReview is set, want true")
	}
	cfgPlanOnly := &Config{Prompts: PromptsConfig{PlanReview: "x"}}
	if !cfgPlanOnly.hasAnyPromptConfigured() {
		t.Error("hasAnyPromptConfigured() = false when only PlanReview is set, want true")
	}
	cfgTriageOnly := &Config{Prompts: PromptsConfig{PlanTriage: "x"}}
	if !cfgTriageOnly.hasAnyPromptConfigured() {
		t.Error("hasAnyPromptConfigured() = false when only PlanTriage is set, want true")
	}
}

func TestConfigHeartbeatPromptNilGuard(t *testing.T) {
	// resolvedRoles == nil (e.g. no roles.toml)
	cfg := &Config{}
	if got := cfg.HeartbeatPrompt("yuki"); got != "" {
		t.Errorf("HeartbeatPrompt with nil resolvedRoles = %q, want empty", got)
	}

	// resolvedRoles set but HeartbeatPrompts is nil (should not panic)
	cfg.resolvedRoles = &RolesConfig{Roles: map[string]string{"yuki": "prompt"}}
	if got := cfg.HeartbeatPrompt("yuki"); got != "" {
		t.Errorf("HeartbeatPrompt with nil HeartbeatPrompts = %q, want empty", got)
	}

	// resolvedRoles and HeartbeatPrompts set, agent present
	cfg.resolvedRoles = &RolesConfig{
		HeartbeatPrompts: map[string]string{"yuki": "heartbeat msg"},
	}
	if got := cfg.HeartbeatPrompt("yuki"); got != "heartbeat msg" {
		t.Errorf("HeartbeatPrompt = %q, want %q", got, "heartbeat msg")
	}

	// agent not present
	if got := cfg.HeartbeatPrompt("inke"); got != "" {
		t.Errorf("HeartbeatPrompt for unknown agent = %q, want empty", got)
	}
}

func TestCommentSyncResolution(t *testing.T) {
	tests := []struct {
		name        string
		commentSync string
		want        string // expected value in ResolvedTeam
	}{
		{
			name:        "empty propagates as empty (daemon defaults to pr)",
			commentSync: "",
			want:        "",
		},
		{
			name:        "none propagates as none",
			commentSync: "none",
			want:        "none",
		},
		{
			name:        "pr propagates as pr",
			commentSync: "pr",
			want:        "pr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := TeamConfig{
				TeamPath:    "/tmp/test",
				CommentSync: tt.commentSync,
			}
			rt, err := resolveTeam("test", team, nil)
			if err != nil {
				t.Fatalf("resolveTeam() error: %v", err)
			}
			if rt.CommentSync != tt.want {
				t.Errorf("CommentSync = %q, want %q", rt.CommentSync, tt.want)
			}
		})
	}
}

// TestPromptContext_ReturnsContextField verifies that Prompt("context") returns the
// Context field from prompts.toml and does not inherit from roles.toml[default].
func TestPromptContext_ReturnsContextField(t *testing.T) {
	contextPrompt := "$ echo hello\nsome text"
	cfg := &Config{
		Prompts: PromptsConfig{Context: contextPrompt},
	}
	got := cfg.Prompt("context")
	if got != contextPrompt {
		t.Errorf("Prompt(\"context\") = %q, want %q", got, contextPrompt)
	}
}

// TestPromptContext_NotInheritedFromDefault verifies "context" is in workerPromptKeys
// so it does not fall back to roles.toml[default].
func TestPromptContext_NotInheritedFromDefault(t *testing.T) {
	if !workerPromptKeys["context"] {
		t.Error("\"context\" should be in workerPromptKeys to prevent roles.toml[default] fallback")
	}
}

// TestPromptContext_HasAnyPromptConfigured verifies hasAnyPromptConfigured returns true
// when only Context is set.
func TestPromptContext_HasAnyPromptConfigured(t *testing.T) {
	cfg := &Config{
		Prompts: PromptsConfig{Context: "$ echo hi"},
	}
	if !cfg.hasAnyPromptConfigured() {
		t.Error("hasAnyPromptConfigured() = false when Context is set, want true")
	}
}

func TestRuntimeForAgent(t *testing.T) {
	// Create temp agent dirs with AGENTS.md for testing per-agent override
	dir := t.TempDir()

	// Agent with codex runtime override
	os.MkdirAll(filepath.Join(dir, "codex-agent"), 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "codex-agent", "AGENTS.md"),
		[]byte("---\nname: codex-agent\ndefault_runtime: codex\n---\n# CodeX Agent"), 0o644) //nolint:errcheck
	// Agent with no runtime override
	os.MkdirAll(filepath.Join(dir, "cc-agent"), 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "cc-agent", "AGENTS.md"),
		[]byte("---\nname: cc-agent\n---\n# CC Agent"), 0o644) //nolint:errcheck

	tests := []struct {
		name      string
		cfg       *DaemonConfig
		teamPath  string
		agentName string
		want      runtime.Runtime
	}{
		{
			"per-agent runtime override",
			&DaemonConfig{Teams: map[string]*ResolvedTeam{"team": {DefaultRuntime: "claude-code"}}},
			dir, "codex-agent", runtime.Codex,
		},
		{
			"team fallback when no per-agent override",
			&DaemonConfig{Teams: map[string]*ResolvedTeam{"team": {DefaultRuntime: "codex"}}},
			dir, "cc-agent", runtime.Codex,
		},
		{
			"ClaudeCode default when no team runtime",
			&DaemonConfig{Teams: map[string]*ResolvedTeam{"team": {}}},
			dir, "cc-agent", runtime.ClaudeCode,
		},
		{
			"ClaudeCode default when unknown team",
			&DaemonConfig{Teams: map[string]*ResolvedTeam{}},
			dir, "cc-agent", runtime.ClaudeCode,
		},
		{
			"teamPath empty string falls back to team runtime",
			&DaemonConfig{Teams: map[string]*ResolvedTeam{"team": {DefaultRuntime: "codex"}}},
			"", "cc-agent", runtime.Codex,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.RuntimeForAgent("team", tt.teamPath, tt.agentName)
			if got != tt.want {
				t.Errorf("RuntimeForAgent() = %v, want %v", got, tt.want)
			}
		})
	}
}
