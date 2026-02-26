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
	ChatID         string                 `toml:"chat_id" jsonschema:"description=Telegram chat ID for notifications"`
	LifecycleAgent string                 `toml:"lifecycle_agent" jsonschema:"description=Agent responsible for worker lifecycle (e.g. kestrel)"`
	MergeMode      string                 `toml:"merge_mode" jsonschema:"enum=auto,enum=manual,description=PR merge mode: auto (merge immediately) or manual (notify only)"`
	Agents         map[string]AgentConfig `toml:"agents" jsonschema:"description=Per-agent Telegram credentials and settings"`
	Voice          VoiceConfig            `toml:"voice" jsonschema:"description=Voice settings (legacy flat layout)"`
	Shell          string                 `toml:"shell" jsonschema:"enum=zsh,enum=fish,description=Shell for spawning workers"`
	Sync           SyncConfig             `toml:"sync" jsonschema:"description=Paths for subagent and skill deployment"`

	// Team-aware fields — optional, empty for legacy configs.
	DefaultTeam string                `toml:"default_team" jsonschema:"description=Active team when TTAL_TEAM env is not set"`
	Teams       map[string]TeamConfig `toml:"teams" jsonschema:"description=Per-team configuration sections"`

	// Resolved at load time, not from TOML.
	resolvedDataDir    string
	resolvedTaskRC     string
	resolvedTaskData   string
	resolvedTeamName   string
	resolvedDefRuntime string
	resolvedMergeMode  string
}

// TeamConfig holds per-team configuration.
type TeamConfig struct {
	DataDir         string                 `toml:"data_dir" jsonschema:"description=ttal data directory (default: ~/.ttal/<team>)"`
	TaskRC          string                 `toml:"taskrc" jsonschema:"description=Taskwarrior config file path (default: <data_dir>/taskrc)"`
	ChatID          string                 `toml:"chat_id" jsonschema:"description=Telegram chat ID for this team"`
	LifecycleAgent  string                 `toml:"lifecycle_agent" jsonschema:"description=Agent responsible for worker lifecycle"`
	DefaultRuntime  string                 `toml:"default_runtime" jsonschema:"enum=claude-code,enum=opencode,enum=codex,description=Default runtime for workers"`
	MergeMode       string                 `toml:"merge_mode" jsonschema:"enum=auto,enum=manual,description=PR merge mode override for this team"`
	VoiceLanguage   string                 `toml:"voice_language" jsonschema:"description=ISO 639-1 language code for Whisper (default: en; auto for auto-detect)"`
	Agents          map[string]AgentConfig `toml:"agents" jsonschema:"description=Per-agent credentials for this team"`
	VoiceVocabulary []string               `toml:"voice_vocabulary" jsonschema:"description=Custom vocabulary words for Whisper transcription accuracy"`
}

// SyncConfig holds paths for subagent, skill, and command deployment.
type SyncConfig struct {
	SubagentsPaths []string `toml:"subagents_paths" jsonschema:"description=Directories to scan for subagent definitions"`
	SkillsPaths    []string `toml:"skills_paths" jsonschema:"description=Directories to scan for skill definitions"`
	CommandsPaths  []string `toml:"commands_paths" jsonschema:"description=Directories to scan for command definitions"`
}

// VoiceConfig holds voice-related settings (legacy flat layout).
type VoiceConfig struct {
	Vocabulary []string `toml:"vocabulary" jsonschema:"description=Custom vocabulary words for Whisper"`
	Language   string   `toml:"language" jsonschema:"description=ISO 639-1 language code (default: en)"`
}

// AgentConfig holds per-agent Telegram credentials and runtime settings.
// ChatID is optional — falls back to the team/global ChatID.
type AgentConfig struct {
	BotToken string `toml:"bot_token" jsonschema:"description=Telegram bot token for this agent"`
	ChatID   string `toml:"chat_id" jsonschema:"description=Per-agent chat ID override (falls back to team/global)"`
	Port     int    `toml:"port" jsonschema:"description=API server port for opencode/codex runtimes"`
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

// TaskData returns the resolved taskwarrior data directory for the active team.
func (c *Config) TaskData() string {
	return c.resolvedTaskData
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

const (
	DefaultTeamName = "default"
	MergeModeAuto   = "auto"
	MergeModeManual = "manual"
)

// GetMergeMode returns the resolved merge mode ("auto" if unset).
// "auto" merges immediately; "manual" sends a notification instead.
func (c *Config) GetMergeMode() string {
	if c.resolvedMergeMode != "" {
		return c.resolvedMergeMode
	}
	return MergeModeAuto
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
		c.resolvedTeamName = DefaultTeamName
		c.resolvedDataDir = defaultDataDir()
		c.resolvedTaskRC = defaultTaskRC()
		c.resolvedTaskData = filepath.Join(c.resolvedDataDir, "tasks")
		c.resolvedMergeMode = c.MergeMode
		return c.validateMergeMode()
	}

	// Resolve active team: TTAL_TEAM env > default_team > "default"
	teamName := os.Getenv("TTAL_TEAM")
	if teamName == "" {
		teamName = c.DefaultTeam
	}
	if teamName == "" {
		teamName = DefaultTeamName
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
	c.Voice = VoiceConfig{
		Vocabulary: team.VoiceVocabulary,
		Language:   team.VoiceLanguage,
	}

	// Resolve DataDir: explicit override > convention
	if team.DataDir != "" {
		c.resolvedDataDir = expandHome(team.DataDir)
	} else if teamName == DefaultTeamName {
		c.resolvedDataDir = defaultDataDir()
	} else {
		// Non-default teams use convention: ~/.ttal/<teamName>/
		c.resolvedDataDir = filepath.Join(defaultDataDir(), teamName)
	}

	// Resolve TaskRC: explicit override > convention
	if team.TaskRC != "" {
		c.resolvedTaskRC = expandHome(team.TaskRC)
	} else if teamName == DefaultTeamName {
		c.resolvedTaskRC = defaultTaskRC()
	} else {
		c.resolvedTaskRC = filepath.Join(c.resolvedDataDir, "taskrc")
	}

	// TaskData: always derived from DataDir
	c.resolvedTaskData = filepath.Join(c.resolvedDataDir, "tasks")

	c.resolvedDefRuntime = team.DefaultRuntime

	// Merge mode: team > global > default("auto")
	if team.MergeMode != "" {
		c.resolvedMergeMode = team.MergeMode
	} else {
		c.resolvedMergeMode = c.MergeMode
	}

	return c.validateMergeMode()
}

func (c *Config) validateMergeMode() error {
	if c.resolvedMergeMode != "" && c.resolvedMergeMode != MergeModeAuto && c.resolvedMergeMode != MergeModeManual {
		return fmt.Errorf("invalid merge_mode %q (must be %q or %q)", c.resolvedMergeMode, MergeModeAuto, MergeModeManual)
	}
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

// ExpandHome replaces a leading ~ or ~/ with the user's home directory.
// Does NOT expand ~username syntax (that would require OS-specific user lookup).
func ExpandHome(path string) string {
	return expandHome(path)
}

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

	template := `#:schema https://ttal.guion.io/schema/config.schema.json
chat_id = "TODO"
lifecycle_agent = "kestrel"

[agents.kestrel]
bot_token = "TODO"

# merge_mode = "auto"  # "auto" (merge immediately) or "manual" (notify, human merges)

# [voice]
# vocabulary = ["ttal", "treemd", "taskwarrior"]

# Multi-team setup (optional):
# default_team = "default"
#
# [teams.default]
# chat_id = "TODO"
# lifecycle_agent = "kestrel"
#
# [teams.guion]
# chat_id = "TODO"
# lifecycle_agent = "kestrel"
# # Paths auto-derived: ~/.ttal/guion/{ttal.db, taskrc, tasks/}
# # Override only if needed: data_dir, taskrc
`

	return os.WriteFile(path, []byte(template), 0o600)
}
