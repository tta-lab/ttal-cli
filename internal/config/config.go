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
// Convention: "ttal-<team>-<agent>" (e.g. "ttal-default-athena", "ttal-guion-mira").
//
// This is distinct from worker sessions which use "w-<uuid[:8]>-<slug>"
// (e.g. "w-e9d4b7c1-fix-auth"). See taskwarrior.Task.SessionName().
func AgentSessionName(team, agent string) string {
	return fmt.Sprintf("ttal-%s-%s", team, agent)
}

// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// Requires [teams] sections. After Load(), resolved fields are populated from the active team.
// Callers access ChatID, Agents, etc. without caring about which team is active.
type Config struct {
	// Resolved fields — populated from active team after Load(). Not directly settable in TOML.
	ChatID         string                 `toml:"-" json:"-"`
	LifecycleAgent string                 `toml:"-" json:"-"`
	Agents         map[string]AgentConfig `toml:"-" json:"-"`
	Voice          VoiceConfig            `toml:"-" json:"-"`

	// Global fields — not per-team.
	Shell string     `toml:"shell" jsonschema:"enum=zsh,enum=fish,description=Shell for spawning workers"`
	Sync  SyncConfig `toml:"sync" jsonschema:"description=Paths for subagent and skill deployment"`

	// Team-aware fields.
	DefaultTeam string                `toml:"default_team" jsonschema:"description=Active team when TTAL_TEAM env is not set"`
	Teams       map[string]TeamConfig `toml:"teams" jsonschema:"description=Per-team configuration sections"`

	// Resolved at load time, not from TOML.
	resolvedDataDir       string
	resolvedTaskRC        string
	resolvedTaskData      string
	resolvedTeamName      string
	resolvedAgentRuntime  string
	resolvedWorkerRuntime string
	resolvedMergeMode     string
	resolvedTeamPath      string
	resolvedDBPath        string
	resolvedGatewayURL    string
	resolvedHooksToken    string
	resolvedDesignAgent   string
	resolvedResearchAgent string
	resolvedTestAgent     string
	resolvedTaskSyncURL   string
}

// TeamConfig holds per-team configuration.
type TeamConfig struct {
	TeamPath        string                 `toml:"team_path" jsonschema:"description=Root path for agent workspaces. Agent path = team_path/agent_name."`
	DBPath          string                 `toml:"db_path" jsonschema:"description=Path to ttal.db (default: <data_dir>/ttal.db). Set to share DB across teams."`
	DataDir         string                 `toml:"data_dir" jsonschema:"description=ttal data directory (default: ~/.ttal/<team>)"`
	TaskRC          string                 `toml:"taskrc" jsonschema:"description=Taskwarrior config file path (default: <data_dir>/taskrc)"`
	ChatID          string                 `toml:"chat_id" jsonschema:"description=Telegram chat ID for this team"`
	LifecycleAgent  string                 `toml:"lifecycle_agent" jsonschema:"description=Agent responsible for worker lifecycle"`
	AgentRuntime    string                 `toml:"agent_runtime" jsonschema:"enum=claude-code,enum=opencode,enum=codex,enum=openclaw,description=Runtime for agent sessions"` //nolint:lll
	WorkerRuntime   string                 `toml:"worker_runtime" jsonschema:"enum=claude-code,enum=opencode,enum=codex,description=Runtime for spawned workers"`             //nolint:lll
	GatewayURL      string                 `toml:"gateway_url" jsonschema:"description=OpenClaw Gateway URL"`
	HooksToken      string                 `toml:"hooks_token" jsonschema:"description=OpenClaw hooks auth token"`
	MergeMode       string                 `toml:"merge_mode" jsonschema:"enum=auto,enum=manual,description=PR merge mode override for this team"`
	VoiceLanguage   string                 `toml:"voice_language" jsonschema:"description=ISO 639-1 language code for Whisper (default: en; auto for auto-detect)"`
	DesignAgent     string                 `toml:"design_agent" jsonschema:"description=Design/brainstorm agent"`
	ResearchAgent   string                 `toml:"research_agent" jsonschema:"description=Research agent"`
	TestAgent       string                 `toml:"test_agent" jsonschema:"description=Test writing agent"`
	Agents          map[string]AgentConfig `toml:"agents" jsonschema:"description=Per-agent credentials for this team"`
	VoiceVocabulary []string               `toml:"voice_vocabulary" jsonschema:"description=Custom vocabulary words for Whisper transcription accuracy"`
	TaskSyncURL     string                 `toml:"task_sync_url" jsonschema:"description=TaskChampion sync server URL for ttal doctor --fix"`
}

// SyncConfig holds paths for subagent, skill, and command deployment.
type SyncConfig struct {
	SubagentsPaths []string `toml:"subagents_paths" jsonschema:"description=Directories to scan for subagent definitions"`
	SkillsPaths    []string `toml:"skills_paths" jsonschema:"description=Directories to scan for skill definitions"`
	CommandsPaths  []string `toml:"commands_paths" jsonschema:"description=Directories to scan for command definitions"`
}

// VoiceConfig holds voice-related settings resolved from the active team.
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

// TeamPath returns the resolved team path for the active team.
func (c *Config) TeamPath() string {
	return c.resolvedTeamPath
}

// AgentPath returns the workspace path for an agent, derived from team_path.
func (c *Config) AgentPath(agentName string) string {
	if c.resolvedTeamPath == "" {
		return ""
	}
	return filepath.Join(c.resolvedTeamPath, agentName)
}

// DBPath returns the resolved database path for the active team.
func (c *Config) DBPath() string {
	return c.resolvedDBPath
}

// TeamName returns the resolved active team name.
func (c *Config) TeamName() string {
	return c.resolvedTeamName
}

// AgentRuntime returns the team's agent runtime ("claude-code" if unset).
func (c *Config) AgentRuntime() runtime.Runtime {
	if c.resolvedAgentRuntime != "" {
		return runtime.Runtime(c.resolvedAgentRuntime)
	}
	return runtime.ClaudeCode
}

// WorkerRuntime returns the team's worker runtime ("claude-code" if unset).
func (c *Config) WorkerRuntime() runtime.Runtime {
	if c.resolvedWorkerRuntime != "" {
		return runtime.Runtime(c.resolvedWorkerRuntime)
	}
	return runtime.ClaudeCode
}

const DefaultGatewayURL = "http://127.0.0.1:18789"

// GatewayURL returns the OpenClaw Gateway URL for the active team.
func (c *Config) GatewayURL() string {
	if c.resolvedGatewayURL != "" {
		return c.resolvedGatewayURL
	}
	return DefaultGatewayURL
}

// HooksToken returns the OpenClaw hooks auth token for the active team.
func (c *Config) HooksToken() string {
	return c.resolvedHooksToken
}

// DesignAgent returns the team's design agent name.
func (c *Config) DesignAgent() string {
	return c.resolvedDesignAgent
}

// ResearchAgent returns the team's research agent name.
func (c *Config) ResearchAgent() string {
	return c.resolvedResearchAgent
}

// TestAgent returns the team's test agent name.
func (c *Config) TestAgent() string {
	return c.resolvedTestAgent
}

// TaskSyncURL returns the TaskChampion sync server URL for the active team.
func (c *Config) TaskSyncURL() string {
	return c.resolvedTaskSyncURL
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

// resolve populates resolved fields from the active team config.
func (c *Config) resolve() error {
	if len(c.Teams) == 0 {
		return fmt.Errorf("config requires [teams] sections (flat config no longer supported)")
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

	// Resolve TeamPath (required — agent paths are derived from it)
	if team.TeamPath == "" {
		return fmt.Errorf("team %q missing required field: team_path", teamName)
	}
	c.resolvedTeamPath = expandHome(team.TeamPath)

	// Resolve DBPath: explicit override > convention (<data_dir>/ttal.db)
	if team.DBPath != "" {
		c.resolvedDBPath = expandHome(team.DBPath)
	} else {
		c.resolvedDBPath = filepath.Join(c.resolvedDataDir, "ttal.db")
	}

	c.resolvedAgentRuntime = team.AgentRuntime
	c.resolvedWorkerRuntime = team.WorkerRuntime
	c.resolvedGatewayURL = team.GatewayURL
	c.resolvedHooksToken = team.HooksToken
	c.resolvedDesignAgent = team.DesignAgent
	c.resolvedResearchAgent = team.ResearchAgent
	c.resolvedTestAgent = team.TestAgent
	c.resolvedTaskSyncURL = team.TaskSyncURL

	// Validate worker_runtime is not openclaw (agent-only)
	if c.resolvedWorkerRuntime != "" {
		rt := runtime.Runtime(c.resolvedWorkerRuntime)
		if !rt.IsWorkerRuntime() {
			return fmt.Errorf("worker_runtime %q is not valid for workers"+
				" (use claude-code, opencode, or codex)", c.resolvedWorkerRuntime)
		}
	}

	// Merge mode: from team config (defaults to empty = "auto" behavior).
	c.resolvedMergeMode = team.MergeMode

	return c.validateMergeMode()
}

func (c *Config) validateMergeMode() error {
	if c.resolvedMergeMode != "" && c.resolvedMergeMode != MergeModeAuto && c.resolvedMergeMode != MergeModeManual {
		return fmt.Errorf("invalid merge_mode %q (must be %q or %q)", c.resolvedMergeMode, MergeModeAuto, MergeModeManual)
	}
	return nil
}

// resolvedPaths caches both dataDir and dbPath together from a single config load,
// preventing divergence between the two values.
var resolvedPaths struct {
	once   sync.Once
	dir    string
	dbPath string
}

func ensureResolvedPaths() {
	resolvedPaths.once.Do(func() {
		cfg, err := Load()
		if err != nil {
			resolvedPaths.dir = defaultDataDir()
			resolvedPaths.dbPath = filepath.Join(defaultDataDir(), "ttal.db")
			return
		}
		resolvedPaths.dir = cfg.resolvedDataDir
		resolvedPaths.dbPath = cfg.resolvedDBPath
	})
}

// ResolveDataDir returns the data directory for the active team without
// requiring a full config load. Falls back to ~/.ttal if config is unavailable.
// Used by path helpers that need to work before config is loaded (e.g. db.DefaultPath).
// Result is cached after first call.
func ResolveDataDir() string {
	ensureResolvedPaths()
	return resolvedPaths.dir
}

// ResolveDBPath returns the database path for the active team without
// requiring a full config load. Used by db.DefaultPath() and hook code.
func ResolveDBPath() string {
	ensureResolvedPaths()
	return resolvedPaths.dbPath
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
default_team = "default"

[teams.default]
chat_id = "TODO"
lifecycle_agent = "kestrel"
team_path = "TODO"           # Root path for agent workspaces
design_agent = "inke"        # Agent for ttal task design
research_agent = "athena"    # Agent for ttal task research
# test_agent = ""            # Agent for ttal task test
# worker_runtime = "claude-code"
# agent_runtime = "claude-code"
# merge_mode = "auto"

[teams.default.agents.kestrel]
bot_token = "TODO"

# Voice settings go under teams:
# [teams.default.voice]
# vocabulary = ["ttal", "treemd", "taskwarrior"]
# language = "en"
`

	return os.WriteFile(path, []byte(template), 0o600)
}
