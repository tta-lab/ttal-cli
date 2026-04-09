package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/skill"
)

// PromptsConfig holds configurable prompt templates for task routing and worker spawn.
// Supports {{task-id}} and {{skill:name}} template variables.
// Role-based keys (designer, researcher) come from roles.toml, not config.toml.
type PromptsConfig struct {
	Context      string `toml:"context" jsonschema:"description=Universal CC SessionStart context template. Lines prefixed with '$ ' are executed as shell commands."` //nolint:lll
	Triage       string `toml:"triage" jsonschema:"description=Prompt sent to coder after PR review. Supports {{review-file}}"`                                        //nolint:lll
	Review       string `toml:"review" jsonschema:"description=Initial reviewer prompt. Supports {{pr-number}} {{pr-title}} {{owner}} {{repo}} {{branch}}"`            //nolint:lll
	ReReview     string `toml:"re_review" jsonschema:"description=Re-review prompt sent to reviewer. Supports {{review-scope}} {{coder-comment}}"`                     //nolint:lll
	PlanReview   string `toml:"plan_review" jsonschema:"description=Plan reviewer prompt. Supports {{task-id}} {{skill:plan-review}}"`                                 //nolint:lll
	PlanReReview string `toml:"plan_re_review" jsonschema:"description=Plan re-review prompt. Supports {{task-id}}"`                                                   //nolint:lll
	PlanTriage   string `toml:"plan_triage" jsonschema:"description=Prompt sent to designer after plan review. Supports {{review-file}}"`                              //nolint:lll
}

const sessionPrefix = "ttal-default-"

// AgentSessionName returns the tmux session name for an agent.
// Convention: "ttal-default-<agent>" (e.g. "ttal-default-athena", "ttal-default-mira").
//
// This is distinct from worker sessions which use "w-<uuid[:8]>-<slug>"
// (e.g. "w-e9d4b7c1-fix-auth"). See taskwarrior.Task.SessionName().
func AgentSessionName(agent string) string {
	return sessionPrefix + agent
}

// DefaultInlineProjects is the default set of flicknote project keywords to inline.
var DefaultInlineProjects = []string{"plan"}

// AskConfig holds settings for reference repo resolution (used by ttal jump).
type AskConfig struct {
	// Local path for cloned OSS reference repos (default: ~/.ttal/references/)
	ReferencesPath string `toml:"references_path"`
}

// FlicknoteConfig holds flicknote-related settings.
type FlicknoteConfig struct {
	// Project substrings to inline (default: plan)
	InlineProjects []string `toml:"inline_projects" jsonschema:"description=Project substrings to inline (default: plan)"`
}

// KubernetesConfig holds kubectl/k8s settings for ttal log.
type KubernetesConfig struct {
	// Context is the kubectl context name to use (e.g. "do-sgp1-guion-k8s")
	Context string `toml:"context"`
	// AllowedNamespaces is the list of namespaces agents are allowed to query logs from.
	AllowedNamespaces []string `toml:"allowed_namespaces"`
}

// UserConfig holds the human user's identity for the GUI and message queries.
type UserConfig struct {
	// Human's display name (e.g. "neil"). Falls back to $USER env var if empty.
	Name string `toml:"name"`
}

// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// Requires [teams] sections. After Load(), resolved fields are populated from the active team.
// Callers access ChatID etc. without caring about which team is active.
type LegacyConfig struct {
	// Resolved fields — populated from active team after Load(). Not directly settable in TOML.
	ChatID            string      `toml:"-" json:"-"`
	LifecycleAgent    string      `toml:"-" json:"-"` // Deprecated: use NotificationToken instead
	NotificationToken string      `toml:"-" json:"-"`
	VoiceResolved     VoiceConfig `toml:"-" json:"-"`

	// Shell for spawning workers
	Shell string `toml:"shell" jsonschema:"enum=zsh,enum=fish"`
	// Paths for subagent and skill deployment
	Sync SyncConfig `toml:"sync"`
	// Prompt templates for task routing (loaded from prompts.toml, not config.toml)
	Prompts PromptsConfig `toml:"-"`
	// Ask holds reference repo path settings (used by ttal jump)
	Ask AskConfig `toml:"ask"`
	// Flicknote integration settings
	Flicknote FlicknoteConfig `toml:"flicknote"`
	// Global voice settings (vocabulary, language)
	Voice VoiceConfig `toml:"voice"`
	// Kubernetes settings for ttal log proxy
	Kubernetes KubernetesConfig `toml:"kubernetes"`

	// Active team — falls back to "default" if unset
	DefaultTeam string `toml:"default_team"` //nolint:lll
	// Per-team configuration sections
	Teams map[string]TeamConfig `toml:"teams"`
	// Human user identity (used by GUI ChatService and message queries)
	User UserConfig `toml:"user"`

	legacyResolvedDataDir          string
	legacyResolvedTaskRC           string
	legacyResolvedTaskData         string
	legacyResolvedDefaultRuntime   string
	legacyResolvedMergeMode        string
	legacyResolvedTeamPath         string
	legacyResolvedProjectsPath     string
	legacyResolvedTaskSyncURL      string
	legacyResolvedEmojiReactions   bool
	legacyResolvedBreatheThreshold float64 // resolved from *float64, default defaultBreatheThreshold
	legacyResolvedRoles            *RolesConfig
}

// TeamConfig holds per-team configuration.
type TeamConfig struct {
	// Messaging frontend for this team ("telegram" or "matrix"; default: "telegram")
	Frontend string `toml:"frontend" jsonschema:"enum=telegram,enum=matrix"`
	// Root path for agent workspaces. Agent path = team_path/agent_name
	TeamPath string `toml:"team_path"` //nolint:lll
	// ttal data directory (default: ~/.ttal/<team>)
	DataDir string `toml:"data_dir"` //nolint:lll
	// Taskwarrior config file path (default: <data_dir>/taskrc)
	TaskRC string `toml:"taskrc"` //nolint:lll
	// Telegram chat ID for this team
	ChatID string `toml:"chat_id"`
	// Deprecated: use notification_token_env instead
	LifecycleAgent string `toml:"lifecycle_agent"` //nolint:lll
	// Override env var for notification bot token (default: {UPPER_TEAM}_NOTIFICATION_BOT_TOKEN)
	NotificationTokenEnv string `toml:"notification_token_env"` //nolint:lll
	// Default runtime for all agents and workers in this team
	DefaultRuntime string `toml:"default_runtime" jsonschema:"enum=claude-code,enum=codex,enum=lenos"` //nolint:lll
	// PR merge mode override for this team
	MergeMode string `toml:"merge_mode" jsonschema:"enum=auto,enum=manual"` //nolint:lll
	// Comment sync mode: "none" (DB only) or "pr" (mirror to PR). Default: "pr".
	CommentSync string `toml:"comment_sync" jsonschema:"enum=none,enum=pr,default=pr"` //nolint:lll
	// ISO 639-1 language code for Whisper (default: en; auto for auto-detect)
	VoiceLanguage string `toml:"voice_language"` //nolint:lll
	// Per-agent credentials for this team
	Agents map[string]AgentConfig `toml:"agents"` //nolint:lll
	// Custom vocabulary words for Whisper transcription accuracy
	VoiceVocabulary []string `toml:"voice_vocabulary"` //nolint:lll
	// Enable emoji reactions on Telegram tool messages
	EmojiReactions *bool `toml:"emoji_reactions" jsonschema:"default=false"`
	// TaskChampion sync server URL for ttal doctor --fix
	TaskSyncURL string `toml:"task_sync_url"` //nolint:lll
	// Optional per-team human identity override (falls back to global [user])
	User UserConfig `toml:"user"` //nolint:lll
	// Matrix-specific configuration (only used when frontend = "matrix")
	Matrix *MatrixTeamConfig `toml:"matrix"`
	// Context usage threshold (%) below which auto-breathe is skipped (default: 40).
	// Use a pointer so that an explicit 0 (always breathe) is not silently promoted to 40.
	BreatheThreshold *float64 `toml:"breathe_threshold"` //nolint:lll
}

// Config is the flat, single-team configuration populated by Load().
// All fields are public and resolved at load time -- no hidden state.
type Config struct {
	// Team-scoped (from [teams.default])
	// Fields that conflict with methods get underscore suffix; others keep original name.
	TeamPath_          string      // conflict with TeamPath() method → keep
	DataDir_           string      // conflict with DataDir() method → keep
	TaskRC_            string      // conflict with TaskRC() method → keep
	TaskData_          string      // conflict with TaskData() method → keep
	TaskSyncURL_       string      // conflict with TaskSyncURL() method → keep
	ChatID_            string      // conflict with ChatID() method → keep
	LifecycleAgent     string      // no conflict → no underscore
	NotificationToken  string      // no conflict → no underscore
	Frontend_          string      // conflict with Frontend() method? no → no underscore
	DefaultRuntimeVal_ string      // conflict with DefaultRuntime() method → keep
	MergeMode_         string      // conflict with MergeMode() method → keep
	CommentSync_       string      // conflict with CommentSync() method → keep
	EmojiReactions_    bool        // conflict with EmojiReactions() method → keep
	BreatheThreshold_  float64     // conflict with BreatheThreshold() method → keep
	UserNameVal_       string      // conflict with UserName() method → keep
	VoiceResolved      VoiceConfig // no conflict → no underscore
	Matrix_            *MatrixTeamConfig
	Voice_             VoiceConfig

	// Global
	Shell_      string          // conflict with GetShell() → keep
	SyncConfig_ SyncConfig      // conflict with Sync() method → rename from Sync_
	Prompts_    PromptsConfig   // conflict with Prompts() method → keep
	Ask_        AskConfig       // conflict with Ask() method? no → no underscore
	Flicknote_  FlicknoteConfig // conflict with Flicknote() method → keep
	Voice       VoiceConfig     // no conflict → no underscore
	Kubernetes_ KubernetesConfig
	Roles_      *RolesConfig

	// Derived
	ProjectsPath string
}

// Teams returns a synthetic map containing only the "default" team,
// populated from the flat fields on Config. Used by doctor/matrix_provision.go
// and other callers that expect the old Teams map.
func (c *Config) Teams() map[string]TeamConfig {
	return map[string]TeamConfig{
		"default": {
			Frontend: c.Frontend_,
			Matrix:   c.Matrix_,
			TeamPath: c.TeamPath_,
		},
	}
}

// --- Accessors ---

// TeamPath returns the resolved team path.
func (c *Config) TeamPath() string { return c.TeamPath_ }

// AgentPath returns the workspace path for an agent.
func (c *Config) AgentPath(name string) string {
	if c.TeamPath_ == "" {
		return ""
	}
	return filepath.Join(c.TeamPath_, name)
}

// DataDir returns the resolved data directory.
func (c *Config) DataDir() string { return c.DataDir_ }

// TaskRC returns the resolved taskrc path.
func (c *Config) TaskRC() string { return c.TaskRC_ }

// TaskData returns the resolved taskwarrior data directory.
func (c *Config) TaskData() string { return c.TaskData_ }

// TaskSyncURL returns the TaskChampion sync server URL.
func (c *Config) TaskSyncURL() string { return c.TaskSyncURL_ }

// UserName returns the human identity, falling back to $USER.
func (c *Config) UserName() string {
	if c.UserNameVal_ != "" {
		return c.UserNameVal_
	}
	return os.Getenv("USER")
}

// DefaultRuntime returns the team's default runtime.
func (c *Config) DefaultRuntime() runtime.Runtime {
	if c.DefaultRuntimeVal_ != "" {
		return runtime.Runtime(c.DefaultRuntimeVal_)
	}
	return runtime.ClaudeCode
}

// GetMergeMode returns the resolved merge mode ("auto" if unset).
func (c *Config) GetMergeMode() string {
	if c.MergeMode_ != "" {
		return c.MergeMode_
	}
	return MergeModeAuto
}

// EmojiReactions returns whether emoji reactions are enabled.
func (c *Config) EmojiReactions() bool { return c.EmojiReactions_ }

// BreatheThreshold returns the context usage % below which auto-breathe is skipped.
func (c *Config) BreatheThreshold() float64 { return c.BreatheThreshold_ }

// ChatID returns the Telegram chat ID for this team.
func (c *Config) ChatID() string { return c.ChatID_ }

// Roles returns the resolved roles config.
func (c *Config) Roles() *RolesConfig { return c.Roles_ }

// HeartbeatPrompt returns the heartbeat_prompt for an agent from roles.toml.
func (c *Config) HeartbeatPrompt(agentName string) string {
	if c.Roles_ == nil || c.Roles_.HeartbeatPrompts == nil {
		return ""
	}
	return c.Roles_.HeartbeatPrompts[agentName]
}

// Prompt returns the prompt template for a key.
// logic with type-specific field access. Will be eliminated when LegacyConfig is removed.
//
//nolint:dupl // Structural duplication with LegacyConfig.Prompt — both serve the same
func (c *Config) Prompt(key string) string {
	roles := c.Roles_
	if roles != nil && roles.Roles != nil {
		if prompt, ok := roles.Roles[key]; ok && prompt != "" {
			return prompt
		}
		if key != "default" && !workerPromptKeys[key] {
			if prompt, ok := roles.Roles["default"]; ok && prompt != "" {
				return prompt
			}
		}
	}
	if c.hasAnyPromptConfigured_() {
		promptsMap := map[string]string{
			"context":        c.Prompts_.Context,
			"triage":         c.Prompts_.Triage,
			"review":         c.Prompts_.Review,
			"re_review":      c.Prompts_.ReReview,
			"plan_review":    c.Prompts_.PlanReview,
			"plan_re_review": c.Prompts_.PlanReReview,
			"plan_triage":    c.Prompts_.PlanTriage,
		}
		if prompt, ok := promptsMap[key]; ok {
			return prompt
		}
	}
	return ""
}

func (c *Config) hasAnyPromptConfigured_() bool {
	return c.Prompts_.Context != "" || c.Prompts_.Triage != "" ||
		c.Prompts_.Review != "" || c.Prompts_.ReReview != "" ||
		c.Prompts_.PlanReview != "" || c.Prompts_.PlanReReview != "" ||
		c.Prompts_.PlanTriage != ""
}

// RenderPrompt resolves {{skill:name}} and {{task-id}} in a prompt template.
func (c *Config) RenderPrompt(key, taskID string, rt runtime.Runtime) string {
	tmpl := c.Prompt(key)
	return RenderTemplate(tmpl, taskID, rt)
}

// WorkerAgentPaths returns the configured worker agent paths from the sync config.
func (c *Config) WorkerAgentPaths() []string {
	return c.SyncConfig_.WorkerAgentPaths
}

// AskReferencesPath returns the resolved path for cloned reference repos.
func (c *Config) AskReferencesPath() string {
	if c.Ask_.ReferencesPath != "" {
		return expandHome(c.Ask_.ReferencesPath)
	}
	return expandHome(defaultAskReferencesPath)
}

// GetShell returns the configured shell (default: zsh).
func (c *Config) GetShell() string {
	if c.Shell_ != "" {
		if validShells[c.Shell_] {
			return c.Shell_
		}
		fmt.Fprintf(os.Stderr, "warning: invalid shell %q in config, falling back to %s\n", c.Shell_, DefaultShell)
	}
	return DefaultShell
}

// ShellCommand returns a shell -c command string for the configured shell.
func (c *Config) ShellCommand(cmd string) string {
	shell := c.GetShell()
	switch shell {
	case ShellFish:
		return fmt.Sprintf("fish -C '%s'", cmd)
	default:
		return fmt.Sprintf("zsh -c '%s'", cmd)
	}
}

// BuildEnvShellCommand returns a shell command with env vars prepended.
func (c *Config) BuildEnvShellCommand(envParts []string, cmd string) string {
	shell := c.GetShell()
	envStr := ""
	if len(envParts) > 0 {
		envStr = fmt.Sprintf("env %s ", strings.Join(envParts, " "))
	}
	switch shell {
	case ShellFish:
		return fmt.Sprintf("%sfish -C '%s'", envStr, cmd)
	default:
		return fmt.Sprintf("%szsh -c '%s'", envStr, cmd)
	}
}

// SyncConfig holds paths for subagent and rule deployment.
type SyncConfig struct {
	// Directories for worker agent definitions (deployed to ~/.claude/agents/)
	WorkerAgentPaths []string `toml:"worker_agent_paths"`
	// Directories for RULE.md files
	RulesPaths []string `toml:"rules_paths"`
	// Path to global CLAUDE.md prompt
	GlobalPromptPath string `toml:"global_prompt_path"`
	// CC plugin marketplace source — local path or git URL.
	// Default: resolved from project store ("ttal" alias).
	MarketplaceSource string `toml:"marketplace_source"`
}

// WorkerAgentPaths returns the configured worker agent paths from the sync config.
func (c *LegacyConfig) WorkerAgentPaths() []string {
	return c.Sync.WorkerAgentPaths
}

// VoiceConfig holds voice-related settings resolved from the active team.
type VoiceConfig struct {
	// Custom vocabulary words for Whisper
	Vocabulary []string `toml:"vocabulary"`
	// ISO 639-1 language code (default: en)
	Language string `toml:"language"`
}

// EffectiveVocabulary returns effective vocabulary for a team:
// global vocabulary + team-specific vocabulary + ALL team names + ALL agent names
// (team names and agent names are global - included for all teams)
func (c *VoiceConfig) EffectiveVocabulary(teamVocabulary []string, allTeamNames, allAgentNames []string) []string {
	v := make([]string, 0, len(c.Vocabulary)+len(teamVocabulary)+len(allTeamNames)+len(allAgentNames))

	v = append(v, c.Vocabulary...)
	v = append(v, teamVocabulary...)
	v = append(v, allTeamNames...)
	v = append(v, allAgentNames...)

	return v
}

// MatrixTeamConfig holds Matrix-specific configuration for a team.
type MatrixTeamConfig struct {
	// Matrix homeserver URL (e.g. "https://matrix.example.com")
	Homeserver string `toml:"homeserver"`
	// Room ID for system notifications
	NotifyRoom string `toml:"notification_room"`
	// Env var name for notification bot access token
	NotifyTokenEnv string `toml:"notification_token_env"`
	// Matrix user ID of the human owner, invited to all provisioned rooms (e.g. "@neil:ttal.dev")
	HumanUserID string `toml:"human_user_id"`
	// Per-agent Matrix credentials
	Agents map[string]MatrixAgentConfig `toml:"agents"`
}

// MatrixAgentConfig holds per-agent Matrix credentials.
type MatrixAgentConfig struct {
	// Env var name for this agent's Matrix access token
	AccessTokenEnv string `toml:"access_token_env"`
	// Matrix room ID for this agent's chat (e.g. "!abc:example.com")
	RoomID string `toml:"room_id"`
}

// Validate checks that required Matrix config fields are set and constraints are met.
func (m *MatrixTeamConfig) Validate() error {
	if m.Homeserver == "" {
		return fmt.Errorf("matrix.homeserver is required")
	}
	if (m.NotifyRoom == "") != (m.NotifyTokenEnv == "") {
		return fmt.Errorf("matrix: notification_room and notification_token_env must both be set or both be empty")
	}
	return nil
}

// AgentConfig is deprecated. Per-agent config now lives in AGENTS.md frontmatter and roles.toml.
// Kept for backward-compatible TOML parsing only — all fields are ignored at runtime.
// Agents are discovered from the filesystem: any subdir of team_path with AGENTS.md is an agent.
type AgentConfig struct {
	BotToken          string `toml:"-"                jsonschema:"-"`
	BotTokenEnv       string `toml:"bot_token_env"    jsonschema:"-"`
	Port              int    `toml:"port"             jsonschema:"-"`
	Runtime           string `toml:"runtime"          jsonschema:"-"`
	Model             string `toml:"model"            jsonschema:"-"`
	HeartbeatInterval string `toml:"heartbeat_interval" jsonschema:"-"`
}

// AgentBotToken returns the bot token for an agent using the naming convention.
// Looks up {UPPER_NAME}_BOT_TOKEN from the loaded .env vars.
func AgentBotToken(agentName string) string {
	key := strings.ToUpper(agentName) + "_BOT_TOKEN"
	return os.Getenv(key)
}

// DefaultRuntime returns the team's default runtime ("claude-code" if unset).
func (c *LegacyConfig) DefaultRuntime() runtime.Runtime {
	if c.legacyResolvedDefaultRuntime != "" {
		return runtime.Runtime(c.legacyResolvedDefaultRuntime)
	}
	return runtime.ClaudeCode
}

// resolveNotificationToken reads the notification bot token from .env.
// Convention: {UPPER_TEAM}_NOTIFICATION_BOT_TOKEN (e.g. DEFAULT_NOTIFICATION_BOT_TOKEN).
// Override: team's notification_token_env field takes priority.
func resolveNotificationToken(teamName, envOverride string) string {
	envKey := envOverride
	if envKey == "" {
		envKey = strings.ToUpper(teamName) + "_NOTIFICATION_BOT_TOKEN"
	}
	env, err := LoadDotEnv()
	if err != nil {
		return ""
	}
	return env[envKey]
}

// loadDotEnvIntoProcess loads .env vars into process environment so AgentBotToken can find them.
// Non-fatal: if .env can't be loaded, AgentBotToken will return empty strings.
func loadDotEnvIntoProcess() {
	env, err := LoadDotEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load .env: %v\n", err)
		return
	}
	for k, v := range env {
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// DataDir returns the resolved data directory for the active team.
func (c *LegacyConfig) DataDir() string {
	return c.legacyResolvedDataDir
}

// TaskRC returns the resolved taskrc path for the active team.
func (c *LegacyConfig) TaskRC() string {
	return c.legacyResolvedTaskRC
}

// TaskData returns the resolved taskwarrior data directory for the active team.
func (c *LegacyConfig) TaskData() string {
	return c.legacyResolvedTaskData
}

// TeamPath returns the resolved team path for the active team.
func (c *LegacyConfig) TeamPath() string {
	return c.legacyResolvedTeamPath
}

// AgentPath returns the workspace path for an agent, derived from team_path.
func (c *LegacyConfig) AgentPath(agentName string) string {
	if c.legacyResolvedTeamPath == "" {
		return ""
	}
	return filepath.Join(c.legacyResolvedTeamPath, agentName)
}

// UserName returns the configured human name, falling back to the $USER env var.
func (c *LegacyConfig) UserName() string {
	if c.User.Name != "" {
		return c.User.Name
	}
	return os.Getenv("USER")
}

// TaskSyncURL returns the TaskChampion sync server URL for the active team.
func (c *LegacyConfig) TaskSyncURL() string {
	return c.legacyResolvedTaskSyncURL
}

const (
	defaultTeamName = "default"
	// DefaultTeamName is the legacy exported name for the default team. Use the unexported
	// defaultTeamName internally. External packages still reference this constant.
	DefaultTeamName         = "default"
	DefaultModel            = "sonnet"
	MergeModeAuto           = "auto"
	MergeModeManual         = "manual"
	defaultBreatheThreshold = 40.0 // % context usage below which auto-breathe is skipped
)

// GetMergeMode returns the resolved merge mode ("auto" if unset).
// "auto" merges immediately; "manual" sends a notification instead.
func (c *LegacyConfig) GetMergeMode() string {
	if c.legacyResolvedMergeMode != "" {
		return c.legacyResolvedMergeMode
	}
	return MergeModeAuto
}

// EmojiReactions returns whether emoji reactions on Telegram tool messages are enabled (default: false).
func (c *LegacyConfig) EmojiReactions() bool {
	return c.legacyResolvedEmojiReactions
}

// BreatheThreshold returns the context usage % below which auto-breathe is skipped.
func (c *LegacyConfig) BreatheThreshold() float64 {
	return c.legacyResolvedBreatheThreshold
}

// workerPromptKeys are worker-plane keys that must not inherit roles.toml[default].
// The default manager-plane prompt must not bleed into worker prompts.
// Keep in sync with PromptsConfig fields and the promptsMap in Prompt() below.
var workerPromptKeys = map[string]bool{
	"coder":          true,
	"context":        true,
	"review":         true,
	"re_review":      true,
	"triage":         true,
	"plan_review":    true,
	"plan_re_review": true,
	"plan_triage":    true,
}

// Prompt returns the prompt template for a given key.
// Priority: roles.toml[key] > roles.toml[default] > config.toml[prompts]
// Worker-plane keys (execute, review, re_review, triage) skip roles.toml[default]
// to prevent manager-plane prompts bleeding into worker sessions.
// logic with type-specific field access. Will be eliminated when LegacyConfig is removed.
//
//nolint:dupl // Structural duplication with Config.Prompt — both serve the same
func (c *LegacyConfig) Prompt(key string) string {
	roles := c.legacyResolvedRoles
	if roles != nil && roles.Roles != nil {
		if prompt, ok := roles.Roles[key]; ok && prompt != "" {
			return prompt
		}
		// Fall back to default prompt if key not found, but skip for worker-plane keys
		if key != "default" && !workerPromptKeys[key] {
			if prompt, ok := roles.Roles["default"]; ok && prompt != "" {
				return prompt
			}
		}
	}

	if c.legacyHasAnyPromptConfigured() {
		promptsMap := map[string]string{
			"context":        c.Prompts.Context,
			"triage":         c.Prompts.Triage,
			"review":         c.Prompts.Review,
			"re_review":      c.Prompts.ReReview,
			"plan_review":    c.Prompts.PlanReview,
			"plan_re_review": c.Prompts.PlanReReview,
			"plan_triage":    c.Prompts.PlanTriage,
		}
		if prompt, ok := promptsMap[key]; ok {
			return prompt
		}
	}

	return ""
}

// HeartbeatPrompt returns the heartbeat_prompt for an agent's role from roles.toml.
func (c *LegacyConfig) HeartbeatPrompt(agentName string) string {
	if c.legacyResolvedRoles == nil || c.legacyResolvedRoles.HeartbeatPrompts == nil {
		return ""
	}
	return c.legacyResolvedRoles.HeartbeatPrompts[agentName]
}

// Roles returns the resolved roles config (exported wrapper for daemon callers).
func (c *LegacyConfig) Roles() *RolesConfig {
	return c.legacyResolvedRoles
}

func (c *LegacyConfig) legacyHasAnyPromptConfigured() bool {
	return c.Prompts.Context != "" || c.Prompts.Triage != "" ||
		c.Prompts.Review != "" || c.Prompts.ReReview != "" ||
		c.Prompts.PlanReview != "" || c.Prompts.PlanReReview != "" ||
		c.Prompts.PlanTriage != ""
}

// RenderTemplate resolves {{skill:name}} and {{task-id}} in an arbitrary template string.
// All {{skill:xxx}} placeholders are collected and prepended at the start of the result.
func RenderTemplate(tmpl, taskID string, rt runtime.Runtime) string {
	result := strings.ReplaceAll(tmpl, "{{task-id}}", taskID)

	// Collect all {{skill:xxx}} and replace with invocation
	var skills []string
	for {
		start := strings.Index(result, "{{skill:")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2

		// Extract skill name
		skillName := result[start+len("{{skill:") : end-2]
		if skillName != "" {
			skillContent := skill.FetchContent(skillName)
			if skillContent != "" {
				skills = append(skills, fmt.Sprintf("# %s [skill]\n\n%s", skillName, skillContent))
			}
		}

		// Remove the placeholder (including any trailing newline that follows {{skill:xxx}}\n)
		remainder := result[end:]
		// Skip leading whitespace/newlines after placeholder removal
		trimmed := strings.TrimPrefix(remainder, "\n")
		result = result[:start] + trimmed
	}

	// Prepend skills at start if any found
	if len(skills) > 0 {
		skillLine := strings.Join(skills, "\n\n")
		result = skillLine + "\n\n" + result
	}

	return result
}

const (
	defaultAskReferencesPath = "~/.ttal/references/"
)

// AskReferencesPath returns the resolved path for cloned reference repos.
// Defaults to ~/.ttal/references/ if not configured.
func (c *LegacyConfig) AskReferencesPath() string {
	if c.Ask.ReferencesPath != "" {
		return expandHome(c.Ask.ReferencesPath)
	}
	return expandHome(defaultAskReferencesPath)
}

const DefaultShell = "zsh"

const ShellFish = "fish"

var validShells = map[string]bool{"zsh": true, ShellFish: true}

func (c *LegacyConfig) GetShell() string {
	if c.Shell != "" {
		if validShells[c.Shell] {
			return c.Shell
		}
		fmt.Fprintf(os.Stderr, "warning: invalid shell %q in config, falling back to %s\n", c.Shell, DefaultShell)
	}
	return DefaultShell
}

func (c *LegacyConfig) ShellCommand(cmd string) string {
	shell := c.GetShell()
	switch shell {
	case ShellFish:
		return fmt.Sprintf("fish -C '%s'", cmd)
	default:
		return fmt.Sprintf("zsh -c '%s'", cmd)
	}
}

func (c *LegacyConfig) BuildEnvShellCommand(envParts []string, cmd string) string {
	shell := c.GetShell()
	envStr := ""
	if len(envParts) > 0 {
		envStr = fmt.Sprintf("env %s ", strings.Join(envParts, " "))
	}
	switch shell {
	case ShellFish:
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

// Load reads and validates ~/.config/ttal/config.toml using the flat Config struct.
// This is the single-team Load() -- LoadAll and DaemonConfig are deprecated.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	var raw rawFile
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s (run: ttal daemon install)", path)
		}
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	team, ok := raw.Teams["default"]
	if !ok {
		return nil, errors.New("config.toml missing [teams.default]")
	}
	if team.TeamPath == "" {
		return nil, errors.New("[teams.default] missing required field: team_path")
	}

	loadDotEnvIntoProcess()

	cfg := &Config{
		TeamPath_:          expandHome(team.TeamPath),
		Frontend_:          team.Frontend,
		LifecycleAgent:     team.LifecycleAgent,
		NotificationToken:  resolveNotificationToken("default", team.NotificationTokenEnv),
		DefaultRuntimeVal_: team.DefaultRuntime,
		MergeMode_:         team.MergeMode,
		CommentSync_:       team.CommentSync,
		ChatID_:            team.ChatID,
		Shell_:             raw.Shell,
		SyncConfig_:        raw.Sync,
		Ask_:               raw.Ask,
		Flicknote_:         raw.Flicknote,
		Kubernetes_:        raw.Kubernetes,
	}

	// Resolve DataDir
	if team.DataDir != "" {
		cfg.DataDir_ = expandHome(team.DataDir)
	} else {
		cfg.DataDir_ = defaultDataDir()
	}
	cfg.TaskData_ = filepath.Join(cfg.DataDir_, "tasks")

	// Resolve TaskRC
	if team.TaskRC != "" {
		cfg.TaskRC_ = expandHome(team.TaskRC)
	} else {
		cfg.TaskRC_ = defaultTaskRC()
	}

	cfg.TaskSyncURL_ = team.TaskSyncURL

	// Resolve emoji reactions
	cfg.EmojiReactions_ = team.EmojiReactions != nil && *team.EmojiReactions

	// Resolve breathe threshold
	if team.BreatheThreshold != nil {
		cfg.BreatheThreshold_ = *team.BreatheThreshold
	} else {
		cfg.BreatheThreshold_ = defaultBreatheThreshold
	}

	// Resolve voice config with merged vocabulary
	cfg.Voice_ = resolveVoiceConfigFlat(team, raw.Voice)
	cfg.Matrix_ = convertRawMatrix(team.Matrix)

	// Resolve user name
	cfg.UserNameVal_ = resolveUserNameFlat(team.User, raw.User)

	// Validate default runtime
	if err := legacyValidateDefaultRuntime(team.DefaultRuntime); err != nil {
		return nil, err
	}

	// Validate merge mode
	if err := validateMergeMode(cfg.MergeMode_); err != nil {
		return nil, err
	}

	// Default flicknote inline projects
	if len(cfg.Flicknote_.InlineProjects) == 0 {
		cfg.Flicknote_.InlineProjects = DefaultInlineProjects
	}

	// Side-loaded files
	roles, err := LoadRoles()
	if err != nil {
		return nil, fmt.Errorf("roles.toml: %w", err)
	}
	cfg.Roles_ = roles

	prompts, err := LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("prompts.toml: %w", err)
	}
	cfg.Prompts_ = prompts

	// Projects path
	cfg.ProjectsPath = projectsPathForTeam("default")

	return cfg, nil
}

// resolveVoiceConfigFlat resolves the voice config for the new flat Config.
func resolveVoiceConfigFlat(team rawTeam, globalVoice VoiceConfig) VoiceConfig {
	allAgentNames := make([]string, 0)
	seenAgents := make(map[string]bool)
	if team.TeamPath != "" {
		names, err := agentfs.DiscoverAgents(expandHome(team.TeamPath))
		if err == nil {
			for _, name := range names {
				if !seenAgents[name] {
					seenAgents[name] = true
					allAgentNames = append(allAgentNames, name)
				}
			}
		}
	}
	mergedVocab := globalVoice.EffectiveVocabulary(team.VoiceVocabulary, []string{defaultTeamName}, allAgentNames)
	lang := globalVoice.Language
	if lang == "" {
		lang = team.VoiceLanguage
	}
	return VoiceConfig{
		Vocabulary: mergedVocab,
		Language:   lang,
	}
}

// resolveUserNameFlat resolves the human identity for the flat Config.
func resolveUserNameFlat(teamUser UserConfig, globalUser UserConfig) string {
	if teamUser.Name != "" {
		return teamUser.Name
	}
	if globalUser.Name != "" {
		return globalUser.Name
	}
	return os.Getenv("USER")
}

// convertRawMatrix converts a rawMatrix (TOML decode target) to MatrixTeamConfig.
func convertRawMatrix(rm *rawMatrix) *MatrixTeamConfig {
	if rm == nil {
		return nil
	}
	agents := make(map[string]MatrixAgentConfig)
	for k, v := range rm.Agents {
		agents[k] = MatrixAgentConfig(v)
	}
	return &MatrixTeamConfig{
		Homeserver:     rm.Homeserver,
		NotifyRoom:     rm.NotifyRoom,
		NotifyTokenEnv: rm.NotifyTokenEnv,
		HumanUserID:    rm.HumanUserID,
		Agents:         agents,
	}
}

// Load reads and validates ~/.config/ttal/config.toml.
// If the config uses [teams], the active team is resolved and its fields
// are promoted to the top-level LegacyConfig fields for backward compatibility.
// AsConfig converts a LegacyConfig to the new Config type for use with
// functions that expect *Config. Used during Step 1/4 daemon migration.
func (c *LegacyConfig) AsConfig() *Config {
	return &Config{
		TeamPath_:          c.legacyResolvedTeamPath,
		DataDir_:           c.legacyResolvedDataDir,
		TaskRC_:            c.legacyResolvedTaskRC,
		TaskData_:          c.legacyResolvedTaskData,
		TaskSyncURL_:       c.legacyResolvedTaskSyncURL,
		ChatID_:            c.ChatID,
		LifecycleAgent:     c.LifecycleAgent,
		NotificationToken:  c.NotificationToken,
		Frontend_:          c.Teams["default"].Frontend,
		DefaultRuntimeVal_: c.legacyResolvedDefaultRuntime,
		MergeMode_:         c.legacyResolvedMergeMode,
		EmojiReactions_:    c.legacyResolvedEmojiReactions,
		BreatheThreshold_:  c.legacyResolvedBreatheThreshold,
		UserNameVal_:       c.UserName(),
		Voice_:             c.VoiceResolved,
		Matrix_:            c.Teams["default"].Matrix,
		Shell_:             c.Shell,
		SyncConfig_:        c.Sync,
		Prompts_:           c.Prompts,
		Ask_:               c.Ask,
		Flicknote_:         c.Flicknote,
		Voice:              c.Voice,
		Kubernetes_:        c.Kubernetes,
		Roles_:             c.legacyResolvedRoles,
		ProjectsPath:       c.legacyResolvedProjectsPath,
	}
}

func legacyLoad() (*LegacyConfig, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	var cfg LegacyConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s (run: ttal daemon install)", path)
		}
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if err := cfg.legacyResolve(); err != nil {
		return nil, err
	}

	// Cache roles at load time so Prompt() doesn't re-read roles.toml on every call.
	roles, err := LoadRoles()
	if err != nil {
		return nil, fmt.Errorf("roles.toml: %w", err)
	}
	cfg.legacyResolvedRoles = roles

	// Load prompts from dedicated file (prompts.toml overrides config.toml [prompts]).
	prompts, err := LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("failed to load prompts.toml: %w", err)
	}
	cfg.Prompts = prompts

	return &cfg, nil
}

// resolve populates resolved fields from the active team config.
func (c *LegacyConfig) legacyResolve() error {
	if len(c.Teams) == 0 {
		return fmt.Errorf("config requires [teams] sections (flat config no longer supported)")
	}

	// Resolve active team: default_team > "default"
	teamName := c.DefaultTeam
	if teamName == "" {
		teamName = defaultTeamName
	}

	team, ok := c.Teams[teamName]
	if !ok {
		return fmt.Errorf("team %q not found in config", teamName)
	}

	// Promote team fields to top-level.
	c.ChatID = team.ChatID
	c.LifecycleAgent = team.LifecycleAgent

	// Load .env vars into process so AgentBotToken() can find them.
	loadDotEnvIntoProcess()

	// Resolve notification bot token from .env
	c.NotificationToken = resolveNotificationToken(teamName, team.NotificationTokenEnv)

	// Resolve voice config with merged vocabulary
	c.VoiceResolved = c.legacyResolveVoiceConfig(team)

	// Resolve DataDir: explicit override > convention
	if team.DataDir != "" {
		c.legacyResolvedDataDir = expandHome(team.DataDir)
	} else if teamName == defaultTeamName {
		c.legacyResolvedDataDir = defaultDataDir()
	} else {
		// Non-default teams use convention: ~/.ttal/<teamName>/
		c.legacyResolvedDataDir = filepath.Join(defaultDataDir(), teamName)
	}

	// Resolve TaskRC: explicit override > convention
	if team.TaskRC != "" {
		c.legacyResolvedTaskRC = expandHome(team.TaskRC)
	} else if teamName == defaultTeamName {
		c.legacyResolvedTaskRC = defaultTaskRC()
	} else {
		c.legacyResolvedTaskRC = filepath.Join(c.legacyResolvedDataDir, "taskrc")
	}

	// TaskData: always derived from DataDir
	c.legacyResolvedTaskData = filepath.Join(c.legacyResolvedDataDir, "tasks")

	// Resolve TeamPath (required — agent paths are derived from it)
	if team.TeamPath == "" {
		return fmt.Errorf("team %q missing required field: team_path", teamName)
	}
	c.legacyResolvedTeamPath = expandHome(team.TeamPath)

	// Resolve ProjectsPath: colocated with config.toml in ~/.config/ttal/
	c.legacyResolvedProjectsPath = projectsPathForTeam(teamName)

	c.legacyResolvedDefaultRuntime = team.DefaultRuntime
	c.legacyResolvedTaskSyncURL = team.TaskSyncURL

	if err := legacyValidateDefaultRuntime(team.DefaultRuntime); err != nil {
		return err
	}

	// Merge mode: from team config (defaults to empty = "auto" behavior).
	c.legacyResolvedMergeMode = team.MergeMode

	// Emoji reactions: from team config (defaults to false).
	c.legacyResolvedEmojiReactions = legacyResolveEmojiReactions(team)

	// Breathe threshold: % context usage below which auto-breathe is skipped (default: 40).
	if team.BreatheThreshold != nil {
		c.legacyResolvedBreatheThreshold = *team.BreatheThreshold
	} else {
		c.legacyResolvedBreatheThreshold = defaultBreatheThreshold
	}

	// Default flicknote inline projects to ["plan"] if not configured.
	if len(c.Flicknote.InlineProjects) == 0 {
		c.Flicknote.InlineProjects = DefaultInlineProjects
	}

	return c.legacyValidateMergeMode()
}

// resolveVoiceConfig resolves the voice config with merged vocabulary for a team.
func (c *LegacyConfig) legacyResolveVoiceConfig(team TeamConfig) VoiceConfig {
	allAgentNames := make([]string, 0)
	seenAgents := make(map[string]bool)
	if team.TeamPath != "" {
		names, err := agentfs.DiscoverAgents(expandHome(team.TeamPath))
		if err == nil {
			for _, name := range names {
				if !seenAgents[name] {
					seenAgents[name] = true
					allAgentNames = append(allAgentNames, name)
				}
			}
		}
	}
	mergedVocab := c.Voice.EffectiveVocabulary(team.VoiceVocabulary, []string{defaultTeamName}, allAgentNames)
	lang := c.Voice.Language
	if lang == "" {
		lang = team.VoiceLanguage
	}
	return VoiceConfig{
		Vocabulary: mergedVocab,
		Language:   lang,
	}
}

// resolveEmojiReactions resolves whether emoji reactions are enabled for a team.
func legacyResolveEmojiReactions(team TeamConfig) bool {
	return team.EmojiReactions != nil && *team.EmojiReactions
}

// validateMergeMode returns an error if the given merge mode string is invalid.
func validateMergeMode(mergeMode string) error {
	if mergeMode != "" && mergeMode != MergeModeAuto && mergeMode != MergeModeManual {
		return fmt.Errorf("invalid merge_mode %q (must be %q or %q)",
			mergeMode, MergeModeAuto, MergeModeManual)
	}
	return nil
}

// legacyValidateDefaultRuntime returns an error if the given runtime string is set but not valid.
func legacyValidateDefaultRuntime(value string) error {
	if value == "" {
		return nil
	}
	if !runtime.Runtime(value).IsWorkerRuntime() {
		return fmt.Errorf("default_runtime %q is not valid (use claude-code, codex, or lenos)", value)
	}
	return nil
}

func (c *LegacyConfig) legacyValidateMergeMode() error {
	return validateMergeMode(c.legacyResolvedMergeMode)
}

// DaemonConfig holds the default team's resolved configuration.
type DaemonConfig struct {
	Global *LegacyConfig            // Raw config (Sync, Shell, Prompts, etc.)
	Team   *ResolvedTeam            // Convenience: Teams["default"]
	Teams  map[string]*ResolvedTeam // TOML decoding artifact only
}

// ResolvedTeam holds one team's fully resolved configuration.
type ResolvedTeam struct {
	Name              string
	Frontend          string // "telegram" or "matrix" (default: "telegram")
	TeamPath          string
	DataDir           string
	TaskRC            string
	ChatID            string
	LifecycleAgent    string // Deprecated: use NotificationToken instead
	NotificationToken string
	DefaultRuntime    string
	MergeMode         string
	CommentSync       string
	Voice             VoiceConfig
	EmojiReactions    bool
	UserName          string            // human identity for this team
	Matrix            *MatrixTeamConfig // nil for telegram teams
}

// UserNameForTeam returns the human identity for the default team.
// Falls back to the global [user] name, then $USER.
func (d *DaemonConfig) UserNameForTeam(_ string) string {
	if d.Team != nil && d.Team.UserName != "" {
		return d.Team.UserName
	}
	return d.Global.UserName()
}

// TeamAgent pairs an agent with its team context.
type TeamAgent struct {
	AgentName string
	ChatID    string // team chat ID (all agents in a team share one chat)
	TeamPath  string
}

// LoadAll loads config.toml and resolves ALL teams.
// Used by the daemon to serve all teams from a single process.
func LoadAll() (*DaemonConfig, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	var cfg LegacyConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s (run: ttal daemon install)", path)
		}
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if _, ok := cfg.Teams[defaultTeamName]; !ok {
		return nil, fmt.Errorf("config.toml missing [teams.default] section")
	}

	mcfg := &DaemonConfig{
		Global: &cfg,
		Teams:  make(map[string]*ResolvedTeam),
	}

	// Cache roles at load time so HeartbeatPrompt() doesn't re-read roles.toml on every call.
	roles, err := LoadRoles()
	if err != nil {
		return nil, fmt.Errorf("roles.toml: %w", err)
	}
	cfg.legacyResolvedRoles = roles

	// Load prompts from dedicated file.
	prompts, err := LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("failed to load prompts.toml: %w", err)
	}
	cfg.Prompts = prompts

	team := cfg.Teams[defaultTeamName]
	rt, err := resolveTeam(defaultTeamName, team, &cfg.Voice)
	if err != nil {
		return nil, fmt.Errorf("team %q: %w", defaultTeamName, err)
	}
	// Resolve human username: per-team override → global → $USER
	userName := team.User.Name
	if userName == "" {
		userName = cfg.User.Name
	}
	if userName == "" {
		userName = os.Getenv("USER")
	}
	rt.UserName = userName
	mcfg.Teams[defaultTeamName] = rt
	mcfg.Team = rt

	// Mirror resolved team_path onto Global so legacy callers of
	// (*Config).AgentPath / TeamPath keep working until Step 3 migrates them.
	// Note: mcfg.Global == &cfg, so writing cfg.legacyResolvedTeamPath here is
	// observable via mcfg.Global.AgentPath() in cmdexec_bridge and routing.
	cfg.legacyResolvedTeamPath = rt.TeamPath

	return mcfg, nil
}

// resolveTeam resolves a single team's config fields.
func resolveTeam(
	teamName string,
	team TeamConfig,
	globalVoice *VoiceConfig,
) (*ResolvedTeam, error) {
	if team.TeamPath == "" {
		return nil, fmt.Errorf("missing required field: team_path")
	}

	// Discover agents in this team for vocabulary
	allAgentNames := make([]string, 0)
	seenAgents := make(map[string]bool)
	if team.TeamPath != "" {
		names, err := agentfs.DiscoverAgents(expandHome(team.TeamPath))
		if err == nil {
			for _, name := range names {
				if !seenAgents[name] {
					seenAgents[name] = true
					allAgentNames = append(allAgentNames, name)
				}
			}
		}
	}

	// Merge global + team vocabulary with this team's agent names
	var mergedVocab []string
	if globalVoice != nil {
		mergedVocab = globalVoice.EffectiveVocabulary(team.VoiceVocabulary, []string{defaultTeamName}, allAgentNames)
	} else {
		mergedVocab = team.VoiceVocabulary
	}

	// Use global language, fallback to team language
	lang := ""
	if globalVoice != nil {
		lang = globalVoice.Language
	}
	if lang == "" {
		lang = team.VoiceLanguage
	}

	// Load .env vars into process so AgentBotToken() can find them.
	loadDotEnvIntoProcess()

	rt := &ResolvedTeam{
		Name:              teamName,
		Frontend:          team.Frontend,
		TeamPath:          expandHome(team.TeamPath),
		ChatID:            team.ChatID,
		LifecycleAgent:    team.LifecycleAgent,
		NotificationToken: resolveNotificationToken(teamName, team.NotificationTokenEnv),
		DefaultRuntime:    team.DefaultRuntime,
		MergeMode:         team.MergeMode,
		CommentSync:       team.CommentSync,
		Voice: VoiceConfig{
			Vocabulary: mergedVocab,
			Language:   lang,
		},
		EmojiReactions: legacyResolveEmojiReactions(team),
		Matrix:         team.Matrix,
	}

	// Resolve DataDir
	if team.DataDir != "" {
		rt.DataDir = expandHome(team.DataDir)
	} else if teamName == defaultTeamName {
		rt.DataDir = defaultDataDir()
	} else {
		rt.DataDir = filepath.Join(defaultDataDir(), teamName)
	}

	// Resolve TaskRC: explicit > convention (<data_dir>/taskrc) > default (~/.taskrc)
	if team.TaskRC != "" {
		rt.TaskRC = expandHome(team.TaskRC)
	} else if teamName == defaultTeamName {
		rt.TaskRC = defaultTaskRC()
	} else {
		rt.TaskRC = filepath.Join(rt.DataDir, "taskrc")
	}

	if err := legacyValidateDefaultRuntime(team.DefaultRuntime); err != nil {
		return nil, err
	}

	return rt, nil
}

// AllAgents returns all agents across all teams, sorted by team then agent name.
// Agents are discovered from the filesystem: any subdir of team_path containing AGENTS.md.
func (m *DaemonConfig) AllAgents() []TeamAgent {
	var agents []TeamAgent
	for _, team := range m.Teams {
		if team.TeamPath == "" {
			continue
		}
		names, err := agentfs.DiscoverAgents(team.TeamPath)
		if err != nil {
			continue
		}
		for _, agentName := range names {
			agents = append(agents, TeamAgent{
				AgentName: agentName,
				ChatID:    team.ChatID,
				TeamPath:  team.TeamPath,
			})
		}
	}
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].AgentName < agents[j].AgentName
	})
	return agents
}

// FindAgent looks up which team an agent belongs to by scanning team paths.
// Returns the first match if agent names are unique across teams.
// Uses agentfs.HasAgent for discovery.
func (m *DaemonConfig) FindAgent(agentName string) (*TeamAgent, bool) {
	for _, team := range m.Teams {
		if team.TeamPath == "" {
			continue
		}
		if agentfs.HasAgent(team.TeamPath, agentName) {
			ta := TeamAgent{
				AgentName: agentName,
				ChatID:    team.ChatID,
				TeamPath:  team.TeamPath,
			}
			return &ta, true
		}
	}
	return nil, false
}

// FindAgentInTeam looks up an agent within a specific team by checking the filesystem.
// Uses agentfs.HasAgent for discovery.
func (m *DaemonConfig) FindAgentInTeam(teamName, agentName string) (*TeamAgent, bool) {
	team, ok := m.Teams[teamName]
	if !ok {
		return nil, false
	}
	if team.TeamPath == "" {
		return nil, false
	}
	if !agentfs.HasAgent(team.TeamPath, agentName) {
		return nil, false
	}
	ta := TeamAgent{
		AgentName: agentName,
		ChatID:    team.ChatID,
		TeamPath:  team.TeamPath,
	}
	return &ta, true
}

// RuntimeForAgent returns the runtime for a specific agent.
// It checks the agent's per-agent frontmatter override first, then falls back to
// the team-level default_runtime, then Claude Code.
func (m *DaemonConfig) RuntimeForAgent(teamName, teamPath, agentName string) runtime.Runtime {
	// Check per-agent frontmatter override
	if teamPath != "" {
		if info, err := agentfs.Get(teamPath, agentName); err == nil && info.DefaultRuntime != "" {
			return runtime.Runtime(info.DefaultRuntime)
		}
	}

	// Fall back to team-level runtime
	team, ok := m.Teams[teamName]
	if !ok {
		return runtime.ClaudeCode
	}
	if team.DefaultRuntime != "" {
		return runtime.Runtime(team.DefaultRuntime)
	}
	return runtime.ClaudeCode
}

// resolvedPaths caches dataDir, dbPath, and projectsPath together from a single config load,
// preventing divergence between the values.
var resolvedPaths struct {
	once         sync.Once
	dir          string
	projectsPath string
}

func legacyEnsureResolvedPaths() {
	resolvedPaths.once.Do(func() {
		cfg, err := legacyLoad()
		if err != nil {
			resolvedPaths.dir = defaultDataDir()
			resolvedPaths.projectsPath = filepath.Join(defaultConfigDir(), "projects.toml")
			return
		}
		resolvedPaths.dir = cfg.legacyResolvedDataDir
		resolvedPaths.projectsPath = cfg.legacyResolvedProjectsPath
	})
}

// ResolveDataDir returns the data directory for the active team without
// requiring a full config load. Falls back to ~/.ttal if config is unavailable.
// Used by path helpers that need to work before config is loaded (e.g. db.DefaultPath).
// Result is cached after first call.
func ResolveDataDir() string {
	legacyEnsureResolvedPaths()
	return resolvedPaths.dir
}

// ResolveProjectsPath returns the projects.toml path for the active team.
// Used by project.Store for default path resolution.
func ResolveProjectsPath() string {
	legacyEnsureResolvedPaths()
	return resolvedPaths.projectsPath
}

// ResolveProjectsPathForTeam returns the projects.toml path for a specific team.
func ResolveProjectsPathForTeam(teamName string) string {
	return projectsPathForTeam(teamName)
}

// projectsPathForTeam returns the projects.toml path for a given team name.
// All project files are colocated with config.toml in ~/.config/ttal/.
func projectsPathForTeam(teamName string) string {
	cfgDir := defaultConfigDir()
	if teamName == defaultTeamName {
		return filepath.Join(cfgDir, "projects.toml")
	}
	return filepath.Join(cfgDir, teamName+"-projects.toml")
}

// DefaultDataDir returns the default data directory (~/.ttal).
func DefaultDataDir() string {
	return defaultDataDir()
}

// DefaultTaskRC returns the default taskrc path (~/.taskrc).
func DefaultTaskRC() string {
	return defaultTaskRC()
}

// SocketPath returns the daemon unix socket path.
// TTAL_SOCKET_PATH env var overrides the default (~/.ttal/daemon.sock).
// Shared by daemon and worker packages to avoid circular imports.
func SocketPath() string {
	if p := os.Getenv("TTAL_SOCKET_PATH"); p != "" {
		return p
	}
	return filepath.Join(defaultDataDir(), "daemon.sock")
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ttal")
}

// DefaultConfigDir returns the path to the ttal configuration directory (~/.config/ttal).
func DefaultConfigDir() string { return defaultConfigDir() }

// LoadTeamPath reads only the team_path field from config.toml.
// This is a lightweight alternative to Load() for hook contexts — hooks run
// synchronously in taskwarrior's pipeline and cannot afford the overhead of
// Load() which also parses roles.toml and prompts.toml (and fails if they're
// missing required fields). The team resolution logic (reading default_team,
// falling back to defaultTeamName) is deliberately inlined here to stay
// independent of Load() and resolve().
// Returns empty string on any error (callers treat empty as "unknown").
func LoadTeamPath(configDir string) string {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}
	path := filepath.Join(configDir, "config.toml")

	// Minimal struct — only decode what we need.
	var cfg struct {
		DefaultTeam string `toml:"default_team"`
		Teams       map[string]struct {
			TeamPath string `toml:"team_path"`
		} `toml:"teams"`
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return ""
	}

	teamName := cfg.DefaultTeam
	if teamName == "" {
		teamName = defaultTeamName
	}
	team, ok := cfg.Teams[teamName]
	if !ok || team.TeamPath == "" {
		return ""
	}
	return ExpandHome(team.TeamPath)
}

func defaultConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ttal")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "ttal")
}

func defaultTaskRC() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".taskrc")
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

// WorktreesRoot returns the directory where ttal worktrees are stored.
// Defaults to ~/.ttal/worktrees.
func WorktreesRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ttal-worktrees"
	}
	return filepath.Join(home, ".ttal", "worktrees")
}

// EnsureWorktreeRoot creates the worktrees root directory if it doesn't exist.
// Returns the root path. Logs a warning to stderr on failure.
func EnsureWorktreeRoot() string {
	root := WorktreesRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create worktree root: %v\n", err)
	}
	return root
}
