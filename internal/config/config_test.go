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
		{"fish override", &Config{Shell_: "fish"}, "fish"},
		{"zsh explicit", &Config{Shell_: "zsh"}, "zsh"},
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
		{"fish shell", &Config{Shell_: "fish"}, "echo hello", "fish -C 'echo hello'"},
		{"zsh explicit", &Config{Shell_: "zsh"}, "echo hello", "zsh -c 'echo hello'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ShellCommand(tt.cmd); got != tt.wantCmd {
				t.Errorf("ShellCommand() = %q, want %q", got, tt.wantCmd)
			}
		})
	}
}

func TestLegacyDefaultRuntime(t *testing.T) {
	// Tests the legacy multi-team resolution path.
	tests := []struct {
		name string
		cfg  *LegacyConfig
		want runtime.Runtime
	}{
		{"unset defaults to claude-code", &LegacyConfig{}, runtime.ClaudeCode},
		{"explicit claude-code", &LegacyConfig{legacyResolvedDefaultRuntime: "claude-code"}, runtime.ClaudeCode},
		{"explicit codex", &LegacyConfig{legacyResolvedDefaultRuntime: "codex"}, runtime.Codex},
		{"explicit lenos", &LegacyConfig{legacyResolvedDefaultRuntime: "lenos"}, runtime.Lenos},
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
	cfg := &LegacyConfig{
		DefaultTeam: "default",
		Teams: map[string]TeamConfig{
			"default": {
				TeamPath:       "/tmp/test",
				DefaultRuntime: "lenos",
				ChatID:         "x",
			},
		},
	}
	if err := cfg.legacyResolve(); err != nil {
		t.Fatalf("legacyResolve() failed: %v", err)
	}
	if got := cfg.DefaultRuntime(); got != runtime.Lenos {
		t.Errorf("DefaultRuntime() = %q, want %q", got, runtime.Lenos)
	}
}

func TestLegacyMergeMode(t *testing.T) {
	// Tests the legacy multi-team merge mode resolution.
	tests := []struct {
		name string
		cfg  *LegacyConfig
		want string
	}{
		{"unset defaults to auto", &LegacyConfig{}, MergeModeAuto},
		{"explicit auto", &LegacyConfig{legacyResolvedMergeMode: "auto"}, MergeModeAuto},
		{"explicit manual", &LegacyConfig{legacyResolvedMergeMode: "manual"}, MergeModeManual},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetMergeMode(); got != tt.want {
				t.Errorf("GetMergeMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLegacyMergeModeResolution(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *LegacyConfig
		want    string
		wantErr bool
	}{
		{
			name: "team sets merge mode",
			cfg: &LegacyConfig{
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
			cfg: &LegacyConfig{
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
			cfg: &LegacyConfig{
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
			cfg: &LegacyConfig{
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
			err := tt.cfg.legacyResolve()
			if tt.wantErr {
				if err == nil {
					t.Fatal("legacyResolve() should have returned an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("legacyResolve() error: %v", err)
			}
			if got := tt.cfg.GetMergeMode(); got != tt.want {
				t.Errorf("GetMergeMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLegacyConventionBasedPaths(t *testing.T) {
	defDataDir := defaultDataDir()
	defTaskRC := defaultTaskRC()

	tests := []struct {
		name         string
		cfg          *LegacyConfig
		wantDataDir  string
		wantTaskRC   string
		wantTaskData string
	}{
		{
			name: "default team uses traditional paths",
			cfg: &LegacyConfig{
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
			cfg: &LegacyConfig{
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
			cfg: &LegacyConfig{
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
			cfg: &LegacyConfig{
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
			if err := tt.cfg.legacyResolve(); err != nil {
				t.Fatalf("legacyResolve() error: %v", err)
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
	// Tests the flat Config AgentPath method.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.AgentPath(tt.agent); got != tt.wantPath {
				t.Errorf("AgentPath(%q) = %q, want %q", tt.agent, got, tt.wantPath)
			}
		})
	}
}

func TestLegacyAgentPath(t *testing.T) {
	// Tests the legacy multi-team AgentPath resolution.
	tests := []struct {
		name     string
		cfg      *LegacyConfig
		agent    string
		wantPath string
	}{
		{
			"normal path joins correctly",
			&LegacyConfig{legacyResolvedTeamPath: "/home/user/agents"},
			"kestrel",
			"/home/user/agents/kestrel",
		},
		{
			"different agent name",
			&LegacyConfig{legacyResolvedTeamPath: "/opt/teams/default"},
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

func TestLegacyAgentPathFromResolve(t *testing.T) {
	cfg := &LegacyConfig{
		DefaultTeam: "test",
		Teams: map[string]TeamConfig{
			"test": {
				TeamPath: "~/agents",
				ChatID:   "x",
				Agents:   map[string]AgentConfig{"k": {}},
			},
		},
	}
	if err := cfg.legacyResolve(); err != nil {
		t.Fatalf("legacyResolve() error: %v", err)
	}

	// After legacyResolve, team_path with ~ should be expanded
	got := cfg.AgentPath("kestrel")
	if got == "" {
		t.Fatal("AgentPath() returned empty after legacyResolve with team_path set")
	}
	if strings.Contains(got, "~") {
		t.Errorf("AgentPath() = %q, should not contain tilde after legacyResolve", got)
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
			&Config{Shell_: "fish"},
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
			&Config{Shell_: "fish"},
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
		Prompts_: PromptsConfig{
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
	if !cfg.hasAnyPromptConfigured_() {
		t.Error("hasAnyPromptConfigured_() = false when PlanReview is set, want true")
	}
	cfgPlanOnly := &Config{Prompts_: PromptsConfig{PlanReview: "x"}}
	if !cfgPlanOnly.hasAnyPromptConfigured_() {
		t.Error("hasAnyPromptConfigured_() = false when only PlanReview is set, want true")
	}
	cfgTriageOnly := &Config{Prompts_: PromptsConfig{PlanTriage: "x"}}
	if !cfgTriageOnly.hasAnyPromptConfigured_() {
		t.Error("hasAnyPromptConfigured_() = false when only PlanTriage is set, want true")
	}
}

func TestLegacyHeartbeatPromptNilGuard(t *testing.T) {
	// Tests LegacyConfig HeartbeatPrompt with nil/partial RolesConfig.
	cfg := &LegacyConfig{}
	if got := cfg.HeartbeatPrompt("yuki"); got != "" {
		t.Errorf("HeartbeatPrompt with nil resolvedRoles = %q, want empty", got)
	}

	// resolvedRoles set but HeartbeatPrompts is nil (should not panic)
	cfg.legacyResolvedRoles = &RolesConfig{Roles: map[string]string{"yuki": "prompt"}}
	if got := cfg.HeartbeatPrompt("yuki"); got != "" {
		t.Errorf("HeartbeatPrompt with nil HeartbeatPrompts = %q, want empty", got)
	}

	// resolvedRoles and HeartbeatPrompts set, agent present
	cfg.legacyResolvedRoles = &RolesConfig{
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
		Prompts_: PromptsConfig{Context: contextPrompt},
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
		Prompts_: PromptsConfig{Context: "$ echo hi"},
	}
	if !cfg.hasAnyPromptConfigured_() {
		t.Error("hasAnyPromptConfigured_() = false when Context is set, want true")
	}
}

// TestLoad_MissingTeamsDefault verifies that Load() returns an actionable error
// when config.toml is missing [teams.default].
func TestLoad_MissingTeamsDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// config.toml with no teams section
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil {
		t.Fatal("Load() should have returned an error for missing [teams.default]")
	}
	if !strings.Contains(err.Error(), "[teams.default]") {
		t.Errorf("error should mention [teams.default], got: %v", err)
	}
}

// TestLoad_MissingTeamPath verifies that Load() returns an actionable error
// when [teams.default] has an empty team_path.
func TestLoad_MissingTeamPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte("[teams.default]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil {
		t.Fatal("Load() should have returned an error for missing team_path")
	}
	if !strings.Contains(err.Error(), "team_path") {
		t.Errorf("error should mention team_path, got: %v", err)
	}
}

// TestLoad_InvalidMergeMode verifies that Load() rejects invalid merge_mode values
// with a helpful error message.
func TestLoad_InvalidMergeMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "[teams.default]\nteam_path = \"/tmp/test\"\nmerge_mode = \"invalid\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil {
		t.Fatal("Load() should have returned an error for invalid merge_mode")
	}
	if !strings.Contains(err.Error(), "invalid merge_mode") {
		t.Errorf("error should mention 'invalid merge_mode', got: %v", err)
	}
}

// TestLoad_InvalidDefaultRuntime verifies that Load() rejects invalid default_runtime values.
func TestLoad_InvalidDefaultRuntime(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "[teams.default]\nteam_path = \"/tmp/test\"\ndefault_runtime = \"not-a-runtime\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil {
		t.Fatal("Load() should have returned an error for invalid default_runtime")
	}
	if !strings.Contains(err.Error(), "default_runtime") {
		t.Errorf("error should mention 'default_runtime', got: %v", err)
	}
}

// TestLegacyConfigAsConfig_MapsFieldsCorrectly verifies that AsConfig() maps
// all fields correctly so that Telegram routing and breathe routing work.
func TestLegacyConfigAsConfig_MapsFieldsCorrectly(t *testing.T) {
	emoji := true
	threshold := 50.0
	cfg := &LegacyConfig{
		Teams: map[string]TeamConfig{
			"default": {
				TeamPath:         "/tmp/team",
				ChatID:           "123456",
				Frontend:         "telegram",
				DefaultRuntime:   "claude-code",
				MergeMode:        "manual",
				EmojiReactions:   &emoji,
				BreatheThreshold: &threshold,
			},
		},
	}
	if err := cfg.legacyResolve(); err != nil {
		t.Fatalf("legacyResolve: %v", err)
	}
	flat := cfg.AsConfig()
	if flat.TeamPath() != "/tmp/team" {
		t.Errorf("TeamPath = %q, want %q", flat.TeamPath(), "/tmp/team")
	}
	if flat.ChatID_ != "123456" {
		t.Errorf("ChatID_ = %q, want %q", flat.ChatID_, "123456")
	}
	if flat.Frontend_ != "telegram" {
		t.Errorf("Frontend_ = %q, want %q", flat.Frontend_, "telegram")
	}
	if flat.DefaultRuntime() != runtime.ClaudeCode {
		t.Errorf("DefaultRuntime = %v, want %v", flat.DefaultRuntime(), runtime.ClaudeCode)
	}
	if flat.GetMergeMode() != "manual" {
		t.Errorf("MergeMode = %q, want %q", flat.GetMergeMode(), "manual")
	}
	if !flat.EmojiReactions_ {
		t.Error("EmojiReactions_ = false, want true")
	}
	if flat.BreatheThreshold_ != 50.0 {
		t.Errorf("BreatheThreshold_ = %v, want %v", flat.BreatheThreshold_, 50.0)
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

// TestLoadAll_GlobalAgentPathResolved verifies that LoadAll populates
// the resolved fields so that AgentPath / TeamPath return correct
// values via mcfg.Global. Regression test for the cmdexec bridge "no workspace" skip.
func TestLoadAll_GlobalAgentPathResolved(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	teamPath := filepath.Join(tmp, "team")
	toml := "[teams.default]\nteam_path = \"" + teamPath + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	mcfg, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	want := filepath.Join(teamPath, "yuki")
	if got := mcfg.Global.AgentPath("yuki"); got != want {
		t.Errorf("Global.AgentPath = %q, want %q", got, want)
	}
	if mcfg.Global.TeamPath() == "" {
		t.Error("Global.TeamPath() empty after LoadAll")
	}
}
