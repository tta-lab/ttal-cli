package config

import (
	"strings"
	"testing"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
)

func TestAgentSessionName(t *testing.T) {
	tests := []struct {
		team  string
		agent string
		want  string
	}{
		{"default", "kestrel", "ttal-default-kestrel"},
		{"default", "yuki", "ttal-default-yuki"},
		{"guion", "mira", "ttal-guion-mira"},
		{"sven", "athena", "ttal-sven-athena"},
	}
	for _, tt := range tests {
		if got := AgentSessionName(tt.team, tt.agent); got != tt.want {
			t.Errorf("AgentSessionName(%q, %q) = %q, want %q", tt.team, tt.agent, got, tt.want)
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

func TestAgentRuntime(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want runtime.Runtime
	}{
		{"unset defaults to claude-code", &Config{}, runtime.ClaudeCode},
		{"explicit opencode", &Config{resolvedAgentRuntime: "opencode"}, runtime.OpenCode},
		{"explicit openclaw", &Config{resolvedAgentRuntime: "openclaw"}, runtime.OpenClaw},
		{"explicit claude-code", &Config{resolvedAgentRuntime: "claude-code"}, runtime.ClaudeCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.AgentRuntime(); got != tt.want {
				t.Errorf("AgentRuntime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkerRuntime(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want runtime.Runtime
	}{
		{"unset defaults to claude-code", &Config{}, runtime.ClaudeCode},
		{"explicit opencode", &Config{resolvedWorkerRuntime: "opencode"}, runtime.OpenCode},
		{"explicit claude-code", &Config{resolvedWorkerRuntime: "claude-code"}, runtime.ClaudeCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.WorkerRuntime(); got != tt.want {
				t.Errorf("WorkerRuntime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkerRuntimeRejectsOpenClaw(t *testing.T) {
	cfg := &Config{
		DefaultTeam: "test",
		Teams: map[string]TeamConfig{
			"test": {
				TeamPath:       "/tmp/test",
				WorkerRuntime:  "openclaw",
				ChatID:         "x",
				LifecycleAgent: "k",
				Agents:         map[string]AgentConfig{"k": {}},
			},
		},
	}
	if err := cfg.resolve(); err == nil {
		t.Fatal("resolve() should reject openclaw as worker_runtime")
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
	t.Setenv("TTAL_TEAM", "")
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
						TeamPath:       "/tmp/test",
						MergeMode:      "manual",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
						TeamPath:       "/tmp/test",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
						TeamPath:       "/tmp/test",
						MergeMode:      "manaul",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
						TeamPath:       "/tmp/test",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
	t.Setenv("TTAL_TEAM", "")
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
						TeamPath:       "/tmp/agents",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
						TeamPath:       "/tmp/agents",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
						TeamPath:       "/tmp/agents",
						DataDir:        "/tmp/custom",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
						TeamPath:       "/tmp/agents",
						TaskRC:         "/tmp/my-taskrc",
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
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
	t.Setenv("TTAL_TEAM", "")
	cfg := &Config{
		DefaultTeam: "test",
		Teams: map[string]TeamConfig{
			"test": {
				TeamPath:       "~/agents",
				ChatID:         "x",
				LifecycleAgent: "k",
				Agents:         map[string]AgentConfig{"k": {}},
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
