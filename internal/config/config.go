package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"github.com/BurntSushi/toml"
)

// AgentSessionName returns the tmux session name for an agent.
// Convention: "session-<agent-name>" (e.g. "session-athena"). Derived, not stored.
//
// This is distinct from worker sessions which use "w-<uuid[:8]>-<slug>"
// (e.g. "w-e9d4b7c1-fix-auth"). See taskwarrior.Task.SessionName().
func AgentSessionName(agent string) string {
	return "session-" + agent
}

// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// Supports two layouts:
//   - Flat (legacy): chat_id, lifecycle_agent, agents, voice at top level
//   - Team-aware: default_team + [teams.<name>] sections
//
// After Load(), the flat fields are always populated (resolved from the active team
// if using team layout). Callers access ChatID, Agents, etc. without caring about teams.
type Config struct {
	// Resolved fields — always populated after Load().
	ChatID         string                 `toml:"chat_id"`
	LifecycleAgent string                 `toml:"lifecycle_agent"`
	Agents         map[string]AgentConfig `toml:"agents"`
	Voice          VoiceConfig            `toml:"voice"`
	Shell          string                 `toml:"shell"`

	// Team-aware fields — optional, empty for legacy configs.
	DefaultTeam string                `toml:"default_team"`
	Teams       map[string]TeamConfig `toml:"teams"`

	// Resolved at load time, not from TOML.
	resolvedDataDir    string
	resolvedTaskRC     string
	resolvedTeamName   string
	resolvedDefRuntime string
}

// TeamConfig holds per-team configuration.
type TeamConfig struct {
	DataDir         string                 `toml:"data_dir"`
	TaskRC          string                 `toml:"taskrc"`
	ChatID          string                 `toml:"chat_id"`
	LifecycleAgent  string                 `toml:"lifecycle_agent"`
	DefaultRuntime  string                 `toml:"default_runtime"`
	Agents          map[string]AgentConfig `toml:"agents"`
	VoiceVocabulary []string               `toml:"voice_vocabulary"`
}

// VoiceConfig holds voice-related settings (legacy flat layout).
type VoiceConfig struct {
	Vocabulary []string `toml:"vocabulary"`
}

// AgentConfig holds per-agent Telegram credentials.
// ChatID is optional — falls back to the team/global ChatID.
type AgentConfig struct {
	BotToken string `toml:"bot_token"`
	ChatID   string `toml:"chat_id"`
}

// AgentChatID returns the effective chat ID for an agent (per-agent override or global).
func (c *Config) AgentChatID(agent string) string {
	if ac, ok := c.Agents[agent]; ok && ac.ChatID != "" {
		return ac.ChatID
	}
	return c.ChatID
}

// DataDir returns the resolved data directory for the active team.
func (c *Config) DataDir() string {
	return c.resolvedDataDir
}

// TaskRC returns the resolved taskrc path for the active team.
func (c *Config) TaskRC() string {
	return c.resolvedTaskRC
}

// TeamName returns the resolved active team name.
func (c *Config) TeamName() string {
	return c.resolvedTeamName
}

// DefaultRuntime returns the team's default runtime ("claude-code" if unset).
func (c *Config) DefaultRuntime() runtime.Runtime {
	if c.resolvedDefRuntime != "" {
		return runtime.Runtime(c.resolvedDefRuntime)
	}
	return runtime.ClaudeCode
}

const DefaultShell = "zsh"

var validShells = map[string]bool{"zsh": true, "fish": true}

func (c *Config) GetShell() string {
	if c.Shell != "" {
		if validShells[c.Shell] {
			return c.Shell
		}
		fmt.Fprintf(os.Stderr, "warning: invalid shell %q in config, falling back to %s\n", c.Shell, DefaultShell)
	}
	return DefaultShell
}

func (c *Config) ShellCommand(cmd string) string {
	shell := c.GetShell()
	switch shell {
	case "fish":
		return fmt.Sprintf("fish -C '%s'", cmd)
	default:
		return fmt.Sprintf("zsh -c '%s'", cmd)
	}
}

func (c *Config) BuildEnvShellCommand(envParts []string, cmd string) string {
	shell := c.GetShell()
	envStr := ""
	if len(envParts) > 0 {
		envStr = fmt.Sprintf("env %s ", strings.Join(envParts, " "))
	}
	switch shell {
	case "fish":
		return fmt.Sprintf("%sfish -C '%s'", envStr, cmd)
	default:
		return fmt.Sprintf("%szsh -c '%s'", envStr, cmd)
	}
}

// Path returns the default path to config.toml.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttal", "config.toml"), nil
}

// Load reads and validates ~/.config/ttal/config.toml.
// If the config uses [teams], the active team is resolved and its fields
// are promoted to the top-level Config fields for backward compatibility.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s (run: ttal daemon install)", path)
		}
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if err := cfg.resolve(); err != nil {
		return nil, err
	}

	if len(cfg.Agents) == 0 {
		return nil, fmt.Errorf("config has no agents defined")
	}

	return &cfg, nil
}

// resolve populates flat fields from the active team config.
// For legacy (flat) configs, it just sets default data dir and taskrc.
func (c *Config) resolve() error {
	if len(c.Teams) == 0 {
		// Legacy flat config — use defaults for data dir and taskrc.
		c.resolvedTeamName = "default"
		c.resolvedDataDir = defaultDataDir()
		c.resolvedTaskRC = defaultTaskRC()
		return nil
	}

	// Resolve active team: TTAL_TEAM env > default_team > "default"
	teamName := os.Getenv("TTAL_TEAM")
	if teamName == "" {
		teamName = c.DefaultTeam
	}
	if teamName == "" {
		teamName = "default"
	}

	team, ok := c.Teams[teamName]
	if !ok {
		return fmt.Errorf("team %q not found in config", teamName)
	}

	c.resolvedTeamName = teamName

	// Promote team fields to top-level.
	c.ChatID = team.ChatID
	c.LifecycleAgent = team.LifecycleAgent
	c.Agents = team.Agents
	c.Voice = VoiceConfig{Vocabulary: team.VoiceVocabulary}

	if team.DataDir != "" {
		c.resolvedDataDir = expandHome(team.DataDir)
	} else {
		c.resolvedDataDir = defaultDataDir()
	}

	if team.TaskRC != "" {
		c.resolvedTaskRC = expandHome(team.TaskRC)
	} else {
		c.resolvedTaskRC = defaultTaskRC()
	}

	c.resolvedDefRuntime = team.DefaultRuntime

	return nil
}

var (
	resolveOnce    sync.Once
	resolvedDirVal string
)

// ResolveDataDir returns the data directory for the active team without
// requiring a full config load. Falls back to ~/.ttal if config is unavailable.
// Used by path helpers that need to work before config is loaded (e.g. db.DefaultPath).
// Result is cached after first call.
func ResolveDataDir() string {
	resolveOnce.Do(func() {
		cfg, err := Load()
		if err != nil {
			resolvedDirVal = defaultDataDir()
			return
		}
		resolvedDirVal = cfg.resolvedDataDir
	})
	return resolvedDirVal
}

// DefaultDataDir returns the default data directory (~/.ttal).
func DefaultDataDir() string {
	return defaultDataDir()
}

// DefaultTaskRC returns the default taskrc path (~/.taskrc).
func DefaultTaskRC() string {
	return defaultTaskRC()
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ttal")
}

func defaultTaskRC() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".taskrc")
}

// clearResolvedFields zeroes out the flat fields that were promoted from a team config.
// Call this before serializing a team-aware config to avoid duplication in TOML output.
func (c *Config) clearResolvedFields() {
	c.ChatID = ""
	c.LifecycleAgent = ""
	c.Agents = nil
	c.Voice = VoiceConfig{}
}

// expandHome replaces a leading ~ or ~/ with the user's home directory.
// Does NOT expand ~username syntax (that would require OS-specific user lookup).
func expandHome(path string) string {
	if len(path) == 0 {
		return path
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// WriteTemplate creates a starter config.toml with example config.
func WriteTemplate() error {
	path, err := Path()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	template := `chat_id = "TODO"
lifecycle_agent = "kestrel"

[agents.kestrel]
bot_token = "TODO"

# [voice]
# vocabulary = ["ttal", "treemd", "taskwarrior"]

# Multi-team setup (optional):
# default_team = "personal"
#
# [teams.personal]
# data_dir = "~/.ttal"
# taskrc = "~/.taskrc"
# chat_id = "TODO"
# lifecycle_agent = "kestrel"
# default_runtime = "claude-code"  # or "opencode"
# voice_vocabulary = ["ttal"]
#
# [teams.personal.agents.kestrel]
# bot_token = "TODO"
`

	return os.WriteFile(path, []byte(template), 0o600)
}
