package config

import (
	"strings"
	"testing"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
)

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
		{"explicit opencode", &Config{resolvedDefRuntime: "opencode"}, runtime.OpenCode},
		{"explicit claude-code", &Config{resolvedDefRuntime: "claude-code"}, runtime.ClaudeCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.DefaultRuntime(); got != tt.want {
				t.Errorf("DefaultRuntime() = %q, want %q", got, tt.want)
			}
		})
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
			name: "legacy flat with merge_mode",
			cfg:  &Config{MergeMode: "manual"},
			want: MergeModeManual,
		},
		{
			name: "team overrides global",
			cfg: &Config{
				MergeMode:   "auto",
				DefaultTeam: "test",
				Teams: map[string]TeamConfig{
					"test": {
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
			name: "team empty falls back to global",
			cfg: &Config{
				MergeMode:   "manual",
				DefaultTeam: "test",
				Teams: map[string]TeamConfig{
					"test": {
						ChatID:         "x",
						LifecycleAgent: "k",
						Agents:         map[string]AgentConfig{"k": {}},
					},
				},
			},
			want: MergeModeManual,
		},
		{
			name:    "invalid merge_mode rejected",
			cfg:     &Config{MergeMode: "manaul"},
			wantErr: true,
		},
		{
			name: "both empty defaults to auto",
			cfg: &Config{
				DefaultTeam: "test",
				Teams: map[string]TeamConfig{
					"test": {
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
