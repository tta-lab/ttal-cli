package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/license"
	"github.com/tta-lab/ttal-cli/internal/runtime"
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

// AgentSessionName returns the tmux session name for an agent.
// Convention: "ttal-<team>-<agent>" (e.g. "ttal-default-athena", "ttal-guion-mira").
//
// This is distinct from worker sessions which use "w-<uuid[:8]>-<slug>"
// (e.g. "w-e9d4b7c1-fix-auth"). See taskwarrior.Task.SessionName().
func AgentSessionName(team, agent string) string {
	return fmt.Sprintf("ttal-%s-%s", team, agent)
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

// UserConfig holds the human user's identity for the GUI and message queries.
type UserConfig struct {
	// Human's display name (e.g. "neil"). Falls back to $USER env var if empty.
	Name string `toml:"name"`
}

// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// Requires [teams] sections. After Load(), resolved fields are populated from the active team.
// Callers access ChatID etc. without caring about which team is active.
type Config struct {
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

	// Active team — falls back to "default" if unset
	DefaultTeam string `toml:"default_team"` //nolint:lll
	// Per-team configuration sections
	Teams map[string]TeamConfig `toml:"teams"`
	// Human user identity (used by GUI ChatService and message queries)
	User UserConfig `toml:"user"`

	// Resolved at load time, not from TOML.
	resolvedDataDir          string
	resolvedTaskRC           string
	resolvedTaskData         string
	resolvedTeamName         string
	resolvedAgentRuntime     string
	resolvedWorkerRuntime    string
	resolvedReviewerRuntime  string
	resolvedMergeMode        string
	resolvedTeamPath         string
	resolvedProjectsPath     string
	resolvedTaskSyncURL      string
	resolvedEmojiReactions   bool
	resolvedBreatheThreshold float64 // resolved from *float64, default defaultBreatheThreshold
	resolvedRoles            *RolesConfig
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
	// Runtime for agent sessions
	AgentRuntime string `toml:"agent_runtime" jsonschema:"enum=claude-code"` //nolint:lll
	// Runtime for spawned workers
	WorkerRuntime string `toml:"worker_runtime" jsonschema:"enum=claude-code,enum=codex"` //nolint:lll
	// Runtime for spawned reviewers (falls back to worker_runtime)
	ReviewerRuntime string `toml:"reviewer_runtime" jsonschema:"enum=claude-code,enum=codex"` //nolint:lll
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

// SyncConfig holds paths for subagent and rule deployment.
type SyncConfig struct {
	// Directories for subagent definitions (team agents deployed to ~/.claude/agents/)
	SubagentsPaths []string `toml:"subagents_paths"`
	// Directories for RULE.md files
	RulesPaths []string `toml:"rules_paths"`
	// Path to global CLAUDE.md prompt
	GlobalPromptPath string `toml:"global_prompt_path"`
	// CC plugin marketplace source — local path or git URL.
	// Default: resolved from project store ("ttal" alias).
	MarketplaceSource string `toml:"marketplace_source"`
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

// AgentConfig is deprecated. Per-agent config now lives in CLAUDE.md frontmatter and roles.toml.
// Kept for backward-compatible TOML parsing only — all fields are ignored at runtime.
// Agents are discovered from the filesystem: any subdir of team_path with CLAUDE.md is an agent.
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

// AgentRuntimeFor returns the team-level agent runtime.
func (c *Config) AgentRuntimeFor(_ string) runtime.Runtime {
	return c.AgentRuntime()
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

// TeamName returns the resolved active team name.
func (c *Config) TeamName() string {
	return c.resolvedTeamName
}

// UserName returns the configured human name, falling back to the $USER env var.
func (c *Config) UserName() string {
	if c.User.Name != "" {
		return c.User.Name
	}
	return os.Getenv("USER")
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

// ReviewerRuntime returns the team's reviewer runtime.
// Falls back to WorkerRuntime if not set.
func (c *Config) ReviewerRuntime() runtime.Runtime {
	if c.resolvedReviewerRuntime != "" {
		return runtime.Runtime(c.resolvedReviewerRuntime)
	}
	return c.WorkerRuntime()
}

// TaskSyncURL returns the TaskChampion sync server URL for the active team.
func (c *Config) TaskSyncURL() string {
	return c.resolvedTaskSyncURL
}

const (
	DefaultTeamName         = "default"
	DefaultModel            = "sonnet"
	MergeModeAuto           = "auto"
	MergeModeManual         = "manual"
	defaultBreatheThreshold = 40.0 // % context usage below which auto-breathe is skipped
)

// checkTeamLicense loads the license and checks if the team count is within limits.
func checkTeamLicense(teamCount int) error {
	lic, err := license.Load()
	if err != nil {
		return fmt.Errorf("license check: %w", err)
	}
	return lic.CheckTeamLimit(teamCount)
}

// GetMergeMode returns the resolved merge mode ("auto" if unset).
// "auto" merges immediately; "manual" sends a notification instead.
func (c *Config) GetMergeMode() string {
	if c.resolvedMergeMode != "" {
		return c.resolvedMergeMode
	}
	return MergeModeAuto
}

// EmojiReactions returns whether emoji reactions on Telegram tool messages are enabled (default: false).
func (c *Config) EmojiReactions() bool {
	return c.resolvedEmojiReactions
}

// BreatheThreshold returns the context usage % below which auto-breathe is skipped.
func (c *Config) BreatheThreshold() float64 {
	return c.resolvedBreatheThreshold
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
func (c *Config) Prompt(key string) string {
	roles := c.resolvedRoles
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

	if c.hasAnyPromptConfigured() {
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

// Roles returns the resolved roles config for use by external packages.
func (c *Config) Roles() *RolesConfig {
	return c.resolvedRoles
}

// HeartbeatPrompt returns the heartbeat_prompt for an agent's role from roles.toml.
// agentName is used directly as the role key (e.g. "yuki" → [yuki] in roles.toml).
// Returns empty string if not configured.
func (c *Config) HeartbeatPrompt(agentName string) string {
	if c.resolvedRoles == nil || c.resolvedRoles.HeartbeatPrompts == nil {
		return ""
	}
	return c.resolvedRoles.HeartbeatPrompts[agentName]
}

func (c *Config) hasAnyPromptConfigured() bool {
	return c.Prompts.Context != "" || c.Prompts.Triage != "" ||
		c.Prompts.Review != "" || c.Prompts.ReReview != "" ||
		c.Prompts.PlanReview != "" || c.Prompts.PlanReReview != "" ||
		c.Prompts.PlanTriage != ""
}

// RenderPrompt resolves {{task-id}} and {{skill:name}} placeholders in a prompt template.
func (c *Config) RenderPrompt(key, taskID string, rt runtime.Runtime) string {
	tmpl := c.Prompt(key)
	return RenderTemplate(tmpl, taskID, rt)
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
			skills = append(skills, runtime.FormatSkillInvocation(rt, skillName))
		}

		// Remove the placeholder (including any trailing newline that follows {{skill:xxx}}\n)
		remainder := result[end:]
		// Skip leading whitespace/newlines after placeholder removal
		trimmed := strings.TrimPrefix(remainder, "\n")
		result = result[:start] + trimmed
	}

	// Prepend skills at start if any found
	if len(skills) > 0 {
		skillLine := strings.Join(skills, "\n")
		result = skillLine + "\n\n" + result
	}

	return result
}

const (
	defaultAskReferencesPath = "~/.ttal/references/"
)

// AskReferencesPath returns the resolved path for cloned reference repos.
// Defaults to ~/.ttal/references/ if not configured.
func (c *Config) AskReferencesPath() string {
	if c.Ask.ReferencesPath != "" {
		return expandHome(c.Ask.ReferencesPath)
	}
	return expandHome(defaultAskReferencesPath)
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

	// Cache roles at load time so Prompt() doesn't re-read roles.toml on every call.
	roles, err := LoadRoles()
	if err != nil {
		return nil, fmt.Errorf("roles.toml: %w", err)
	}
	cfg.resolvedRoles = roles

	// Load prompts from dedicated file (prompts.toml overrides config.toml [prompts]).
	prompts, err := LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("failed to load prompts.toml: %w", err)
	}
	cfg.Prompts = prompts

	return &cfg, nil
}

// resolve populates resolved fields from the active team config.
func (c *Config) resolve() error {
	if len(c.Teams) == 0 {
		return fmt.Errorf("config requires [teams] sections (flat config no longer supported)")
	}

	// Enforce team count limit based on license tier.
	if err := checkTeamLicense(len(c.Teams)); err != nil {
		return err
	}

	// Resolve active team: default_team > "default"
	teamName := c.DefaultTeam
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

	// Load .env vars into process so AgentBotToken() can find them.
	loadDotEnvIntoProcess()

	// Resolve notification bot token from .env
	c.NotificationToken = resolveNotificationToken(teamName, team.NotificationTokenEnv)

	// Resolve voice config with merged vocabulary
	c.VoiceResolved = c.resolveVoiceConfig(team)

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

	// Resolve ProjectsPath: colocated with config.toml in ~/.config/ttal/
	c.resolvedProjectsPath = projectsPathForTeam(teamName)

	c.resolvedAgentRuntime = team.AgentRuntime
	c.resolvedWorkerRuntime = team.WorkerRuntime
	c.resolvedReviewerRuntime = team.ReviewerRuntime
	c.resolvedTaskSyncURL = team.TaskSyncURL

	if err := validateTeamRuntimes(c.resolvedWorkerRuntime, c.resolvedReviewerRuntime); err != nil {
		return err
	}

	// Merge mode: from team config (defaults to empty = "auto" behavior).
	c.resolvedMergeMode = team.MergeMode

	// Emoji reactions: from team config (defaults to false).
	c.resolvedEmojiReactions = resolveEmojiReactions(team)

	// Breathe threshold: % context usage below which auto-breathe is skipped (default: 40).
	if team.BreatheThreshold != nil {
		c.resolvedBreatheThreshold = *team.BreatheThreshold
	} else {
		c.resolvedBreatheThreshold = defaultBreatheThreshold
	}

	// Default flicknote inline projects to ["plan"] if not configured.
	if len(c.Flicknote.InlineProjects) == 0 {
		c.Flicknote.InlineProjects = DefaultInlineProjects
	}

	return c.validateMergeMode()
}

// resolveVoiceConfig resolves the voice config with merged vocabulary for a team.
func (c *Config) resolveVoiceConfig(team TeamConfig) VoiceConfig {
	allTeamNames := make([]string, 0, len(c.Teams))
	allAgentNames := make([]string, 0)
	seenAgents := make(map[string]bool)
	for tn, t := range c.Teams {
		allTeamNames = append(allTeamNames, tn)
		if t.TeamPath != "" {
			names, err := agentfs.DiscoverAgents(expandHome(t.TeamPath))
			if err == nil {
				for _, name := range names {
					if !seenAgents[name] {
						seenAgents[name] = true
						allAgentNames = append(allAgentNames, name)
					}
				}
			}
		}
	}
	mergedVocab := c.Voice.EffectiveVocabulary(team.VoiceVocabulary, allTeamNames, allAgentNames)
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
func resolveEmojiReactions(team TeamConfig) bool {
	return team.EmojiReactions != nil && *team.EmojiReactions
}

// validateTeamRuntimes validates worker_runtime and reviewer_runtime for a team config.
// Combines both checks into a single call to keep callers at low cyclomatic complexity.
func validateTeamRuntimes(workerRuntime, reviewerRuntime string) error {
	if err := validateWorkerPlaneRuntime("worker_runtime", "workers", workerRuntime); err != nil {
		return err
	}
	return validateWorkerPlaneRuntime("reviewer_runtime", "reviewers", reviewerRuntime)
}

// validateWorkerPlaneRuntime returns an error if the given runtime string is set but not
// valid for worker-plane sessions.
// role is the human-readable noun for the error message (e.g. "workers", "reviewers").
func validateWorkerPlaneRuntime(field, role, value string) error {
	if value == "" {
		return nil
	}
	if !runtime.Runtime(value).IsWorkerRuntime() {
		return fmt.Errorf("%s %q is not valid for %s (use claude-code or codex)", field, value, role)
	}
	return nil
}

func (c *Config) validateMergeMode() error {
	if c.resolvedMergeMode != "" && c.resolvedMergeMode != MergeModeAuto && c.resolvedMergeMode != MergeModeManual {
		return fmt.Errorf("invalid merge_mode %q (must be %q or %q)", c.resolvedMergeMode, MergeModeAuto, MergeModeManual)
	}
	return nil
}

// DaemonConfig holds all teams' resolved configurations.
type DaemonConfig struct {
	Global *Config                  // Raw config (Sync, Shell, Prompts, etc.)
	Teams  map[string]*ResolvedTeam // team name -> resolved team config
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
	AgentRuntime      string
	WorkerRuntime     string
	ReviewerRuntime   string
	MergeMode         string
	CommentSync       string
	Voice             VoiceConfig
	EmojiReactions    bool
	UserName          string            // human identity for this team
	Matrix            *MatrixTeamConfig // nil for telegram teams
}

// UserNameForTeam returns the human identity for a given team.
// Falls back to the global [user] name, then $USER.
func (d *DaemonConfig) UserNameForTeam(teamName string) string {
	if team, ok := d.Teams[teamName]; ok && team.UserName != "" {
		return team.UserName
	}
	return d.Global.UserName()
}

// DefaultTeamName returns the default team name with fallback to "default".
func (m *DaemonConfig) DefaultTeamName() string {
	if m.Global.DefaultTeam != "" {
		return m.Global.DefaultTeam
	}
	return DefaultTeamName
}

// TeamAgent pairs an agent with its team context.
type TeamAgent struct {
	TeamName  string
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

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s (run: ttal daemon install)", path)
		}
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if len(cfg.Teams) == 0 {
		return nil, fmt.Errorf("config requires [teams] sections")
	}

	// Enforce team count limit based on license tier.
	if err := checkTeamLicense(len(cfg.Teams)); err != nil {
		return nil, err
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
	cfg.resolvedRoles = roles

	// Load prompts from dedicated file.
	prompts, err := LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("failed to load prompts.toml: %w", err)
	}
	cfg.Prompts = prompts

	for teamName, team := range cfg.Teams {
		rt, err := resolveTeam(teamName, team, &cfg.Voice, cfg.Teams)
		if err != nil {
			return nil, fmt.Errorf("team %q: %w", teamName, err)
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
		mcfg.Teams[teamName] = rt
	}

	return mcfg, nil
}

// resolveTeam resolves a single team's config fields.
func resolveTeam(
	teamName string,
	team TeamConfig,
	globalVoice *VoiceConfig,
	allTeams map[string]TeamConfig,
) (*ResolvedTeam, error) {
	if team.TeamPath == "" {
		return nil, fmt.Errorf("missing required field: team_path")
	}

	// Build all team names and agent names for vocabulary
	allTeamNames := make([]string, 0, len(allTeams))
	allAgentNames := make([]string, 0)
	seenAgents := make(map[string]bool)
	for tn, t := range allTeams {
		allTeamNames = append(allTeamNames, tn)
		if t.TeamPath != "" {
			names, err := agentfs.DiscoverAgents(expandHome(t.TeamPath))
			if err == nil {
				for _, name := range names {
					if !seenAgents[name] {
						seenAgents[name] = true
						allAgentNames = append(allAgentNames, name)
					}
				}
			}
		}
	}

	// Merge global + team vocabulary with all team/agent names
	var mergedVocab []string
	if globalVoice != nil {
		mergedVocab = globalVoice.EffectiveVocabulary(team.VoiceVocabulary, allTeamNames, allAgentNames)
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
		AgentRuntime:      team.AgentRuntime,
		WorkerRuntime:     team.WorkerRuntime,
		ReviewerRuntime:   team.ReviewerRuntime,
		MergeMode:         team.MergeMode,
		CommentSync:       team.CommentSync,
		Voice: VoiceConfig{
			Vocabulary: mergedVocab,
			Language:   lang,
		},
		EmojiReactions: resolveEmojiReactions(team),
		Matrix:         team.Matrix,
	}

	// Resolve DataDir
	if team.DataDir != "" {
		rt.DataDir = expandHome(team.DataDir)
	} else if teamName == DefaultTeamName {
		rt.DataDir = defaultDataDir()
	} else {
		rt.DataDir = filepath.Join(defaultDataDir(), teamName)
	}

	// Resolve TaskRC: explicit > convention (<data_dir>/taskrc) > default (~/.taskrc)
	if team.TaskRC != "" {
		rt.TaskRC = expandHome(team.TaskRC)
	} else if teamName == DefaultTeamName {
		rt.TaskRC = defaultTaskRC()
	} else {
		rt.TaskRC = filepath.Join(rt.DataDir, "taskrc")
	}

	if err := validateTeamRuntimes(team.WorkerRuntime, team.ReviewerRuntime); err != nil {
		return nil, err
	}

	return rt, nil
}

// AllAgents returns all agents across all teams, sorted by team then agent name.
// Agents are discovered from the filesystem: any subdir of team_path containing CLAUDE.md.
func (m *DaemonConfig) AllAgents() []TeamAgent {
	var agents []TeamAgent
	for teamName, team := range m.Teams {
		if team.TeamPath == "" {
			continue
		}
		names, err := agentfs.DiscoverAgents(team.TeamPath)
		if err != nil {
			continue
		}
		for _, agentName := range names {
			agents = append(agents, TeamAgent{
				TeamName:  teamName,
				AgentName: agentName,
				ChatID:    team.ChatID,
				TeamPath:  team.TeamPath,
			})
		}
	}
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].TeamName != agents[j].TeamName {
			return agents[i].TeamName < agents[j].TeamName
		}
		return agents[i].AgentName < agents[j].AgentName
	})
	return agents
}

// FindAgent looks up which team an agent belongs to by scanning team paths.
// Returns the first match if agent names are unique across teams.
// Uses agentfs.HasAgent for discovery.
func (m *DaemonConfig) FindAgent(agentName string) (*TeamAgent, bool) {
	for teamName, team := range m.Teams {
		if team.TeamPath == "" {
			continue
		}
		if agentfs.HasAgent(team.TeamPath, agentName) {
			ta := TeamAgent{
				TeamName:  teamName,
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
		TeamName:  teamName,
		AgentName: agentName,
		ChatID:    team.ChatID,
		TeamPath:  team.TeamPath,
	}
	return &ta, true
}

// AgentRuntimeForTeam returns the team-level agent runtime.
// Per-agent overrides are no longer supported; configure via team agent_runtime.
func (m *DaemonConfig) AgentRuntimeForTeam(teamName, _ string) runtime.Runtime {
	team, ok := m.Teams[teamName]
	if !ok {
		return runtime.ClaudeCode
	}
	if team.AgentRuntime != "" {
		return runtime.Runtime(team.AgentRuntime)
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

func ensureResolvedPaths() {
	resolvedPaths.once.Do(func() {
		cfg, err := Load()
		if err != nil {
			resolvedPaths.dir = defaultDataDir()
			resolvedPaths.projectsPath = filepath.Join(defaultConfigDir(), "projects.toml")
			return
		}
		resolvedPaths.dir = cfg.resolvedDataDir
		resolvedPaths.projectsPath = cfg.resolvedProjectsPath
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

// ResolveProjectsPath returns the projects.toml path for the active team.
// Used by project.Store for default path resolution.
func ResolveProjectsPath() string {
	ensureResolvedPaths()
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
	if teamName == DefaultTeamName {
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
// falling back to DefaultTeamName) is deliberately inlined here to stay
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
		teamName = DefaultTeamName
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
