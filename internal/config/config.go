package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// PromptsConfig holds configurable prompt templates for task routing and worker spawn.
// Supports {{task-id}} template variables.
// Role-based keys (designer, researcher) come from roles.toml, not config.toml.
type PromptsConfig struct {
	Context      string `toml:"context" jsonschema:"description=Universal CC SessionStart context template. Lines prefixed with '$ ' are executed as shell commands."` //nolint:lll
	Triage       string `toml:"triage" jsonschema:"description=Prompt sent to coder after PR review. Supports {{review-file}}"`                                        //nolint:lll
	Review       string `toml:"review" jsonschema:"description=Initial reviewer prompt. Supports {{pr-number}} {{pr-title}} {{owner}} {{repo}} {{branch}}"`            //nolint:lll
	ReReview     string `toml:"re_review" jsonschema:"description=Re-review prompt sent to reviewer. Supports {{review-scope}} {{coder-comment}}"`                     //nolint:lll
	PlanReview   string `toml:"plan_review" jsonschema:"description=Plan reviewer prompt. Supports {{task-id}}"`                                                       //nolint:lll
	PlanReReview string `toml:"plan_re_review" jsonschema:"description=Plan re-review prompt. Supports {{task-id}}"`                                                   //nolint:lll
	PlanTriage   string `toml:"plan_triage" jsonschema:"description=Prompt sent to designer after plan review. Supports {{review-file}}"`                              //nolint:lll
}

// DefaultTeamName is the single, hardcoded team name.
const DefaultTeamName = "default"

// defaultTeamName is a package-local alias for backwards compatibility.
const defaultTeamName = DefaultTeamName

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

// Config is the top-level fully-resolved runtime configuration.
// Populated by [Load] from config.toml, roles.toml, prompts.toml, and .env.
//
// Requires [teams.default] section. Callers access fields directly.
type Config struct {
	// Team-scoped fields (from [teams.default])
	TeamPath          string
	DataDir           string
	TaskRC            string
	TaskData          string
	TaskSyncURL       string
	ChatID            string
	LifecycleAgent    string
	NotificationToken string
	Frontend          string
	DefaultRuntime    string
	MergeMode         string
	CommentSync       string
	EmojiReactions    bool
	BreatheThreshold  float64
	UserName          string

	VoiceResolved VoiceConfig
	Matrix        *MatrixTeamConfig
	Voice         VoiceConfig

	// Global fields (from top-level config.toml)
	Shell      string
	Sync       SyncConfig
	Prompts    PromptsConfig
	Ask        AskConfig
	Flicknote  FlicknoteConfig
	Kubernetes KubernetesConfig
	Roles      *RolesConfig

	// Derived
	ProjectsPath string
}

// AgentInfo describes an agent discovered under TeamPath.
// TeamPath is included so callers can iterate without holding a *Config ref.
type AgentInfo struct {
	AgentName string
	TeamPath  string
}

// Agents returns all agents discovered under TeamPath via agentfs.
func (c *Config) Agents() []AgentInfo {
	if c.TeamPath == "" {
		return nil
	}
	names, _ := agentfs.DiscoverAgents(c.TeamPath)
	out := make([]AgentInfo, 0, len(names))
	for _, name := range names {
		out = append(out, AgentInfo{AgentName: name, TeamPath: c.TeamPath})
	}
	return out
}

// FindAgent returns the agent info by name, or (nil, false).
func (c *Config) FindAgent(name string) (*AgentInfo, bool) {
	if c.TeamPath == "" || !agentfs.HasAgent(c.TeamPath, name) {
		return nil, false
	}
	return &AgentInfo{AgentName: name, TeamPath: c.TeamPath}, true
}

// RuntimeForAgent returns the runtime for the given agent name.
// Falls back to Config.DefaultRuntime when the agent has no override.
func (c *Config) RuntimeForAgent(name string) runtime.Runtime {
	if c.TeamPath != "" {
		if info, err := agentfs.Get(c.TeamPath, name); err == nil && info.DefaultRuntime != "" {
			return runtime.Runtime(info.DefaultRuntime)
		}
	}
	if c.DefaultRuntime != "" {
		return runtime.Runtime(c.DefaultRuntime)
	}
	return runtime.ClaudeCode
}

// AgentPath returns the workspace path for an agent.
func (c *Config) AgentPath(name string) string {
	if c.TeamPath == "" {
		return ""
	}
	return filepath.Join(c.TeamPath, name)
}

// HeartbeatPrompt returns the heartbeat_prompt for an agent from roles.toml.
func (c *Config) HeartbeatPrompt(agentName string) string {
	if c.Roles == nil || c.Roles.HeartbeatPrompts == nil {
		return ""
	}
	return c.Roles.HeartbeatPrompts[agentName]
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
	roles := c.Roles
	if roles != nil && roles.Roles != nil {
		if prompt, ok := roles.Roles[key]; ok && prompt != "" {
			return prompt
		}
		// Fall back to default prompt if key not found, but skip for worker-plane keys
		if key != defaultTeamName && !workerPromptKeys[key] {
			if prompt, ok := roles.Roles[defaultTeamName]; ok && prompt != "" {
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

func (c *Config) hasAnyPromptConfigured() bool {
	return c.Prompts.Context != "" || c.Prompts.Triage != "" ||
		c.Prompts.Review != "" || c.Prompts.ReReview != "" ||
		c.Prompts.PlanReview != "" || c.Prompts.PlanReReview != "" ||
		c.Prompts.PlanTriage != ""
}

// RenderPrompt resolves {{task-id}} in a prompt template.
func (c *Config) RenderPrompt(key, taskID string, rt runtime.Runtime) string {
	tmpl := c.Prompt(key)
	return RenderTemplate(tmpl, taskID, rt)
}

// WorkerAgentPaths returns the configured worker agent paths from the sync config.
func (c *Config) WorkerAgentPaths() []string {
	return c.Sync.WorkerAgentPaths
}

// SkillsDestDir returns the destination directory for deployed skills.
// Defaults to ~/.agents/skills if not configured.
func (c *Config) SkillsDestDir() string {
	if c.Sync.SkillsDest != "" {
		return expandHome(c.Sync.SkillsDest)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agents", "skills")
}

// AskReferencesPath returns the resolved path for cloned reference repos.
// Defaults to ~/.ttal/references/ if not configured.
func (c *Config) AskReferencesPath() string {
	if c.Ask.ReferencesPath != "" {
		return expandHome(c.Ask.ReferencesPath)
	}
	return expandHome(defaultAskReferencesPath)
}

const DefaultShell = "zsh"

const ShellFish = "fish"

var validShells = map[string]bool{"zsh": true, ShellFish: true}

// GetShell returns the configured shell (default: zsh).
func (c *Config) GetShell() string {
	if c.Shell != "" {
		if validShells[c.Shell] {
			return c.Shell
		}
		fmt.Fprintf(os.Stderr, "warning: invalid shell %q in config, falling back to %s\n", c.Shell, DefaultShell)
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
	// Directories for skill SKILL.md files (deployed to skills_dest)
	SkillsPaths []string `toml:"skills_paths"`
	// Destination directory for deployed skills (default: ~/.agents/skills)
	SkillsDest string `toml:"skills_dest"`
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

const (
	DefaultModel             = "sonnet"
	MergeModeAuto            = "auto"
	MergeModeManual          = "manual"
	defaultBreatheThreshold  = 40.0 // % context usage below which auto-breathe is skipped
	defaultAskReferencesPath = "~/.ttal/references/"
)

// Path returns the default path to config.toml.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttal", "config.toml"), nil
}

// Load reads and validates ~/.config/ttal/config.toml.
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

	team, ok := raw.Teams[defaultTeamName]
	if !ok {
		return nil, errors.New("config.toml missing [teams.default]")
	}
	if team.TeamPath == "" {
		return nil, errors.New("[teams.default] missing required field: team_path")
	}

	loadDotEnvIntoProcess()

	cfg := &Config{
		TeamPath:          expandHome(team.TeamPath),
		Frontend:          team.Frontend,
		LifecycleAgent:    team.LifecycleAgent,
		NotificationToken: resolveNotificationToken(defaultTeamName, team.NotificationTokenEnv),
		DefaultRuntime:    team.DefaultRuntime,
		MergeMode:         team.MergeMode,
		CommentSync:       team.CommentSync,
		ChatID:            team.ChatID,
		Shell:             raw.Shell,
		Sync:              raw.Sync,
		Ask:               raw.Ask,
		Flicknote:         raw.Flicknote,
		Kubernetes:        raw.Kubernetes,
	}

	// Resolve DataDir
	if team.DataDir != "" {
		cfg.DataDir = expandHome(team.DataDir)
	} else {
		cfg.DataDir = defaultDataDir()
	}
	cfg.TaskData = filepath.Join(cfg.DataDir, "tasks")

	// Resolve TaskRC
	if team.TaskRC != "" {
		cfg.TaskRC = expandHome(team.TaskRC)
	} else {
		cfg.TaskRC = defaultTaskRC()
	}

	cfg.TaskSyncURL = team.TaskSyncURL

	// Resolve emoji reactions
	cfg.EmojiReactions = team.EmojiReactions != nil && *team.EmojiReactions

	// Resolve breathe threshold
	if team.BreatheThreshold != nil {
		cfg.BreatheThreshold = *team.BreatheThreshold
	} else {
		cfg.BreatheThreshold = defaultBreatheThreshold
	}

	// Resolve voice config with merged vocabulary
	cfg.Voice = resolveVoiceConfigFlat(team, raw.Voice)
	cfg.Matrix = convertRawMatrix(team.Matrix)

	// Resolve user name
	cfg.UserName = resolveUserNameFlat(team.User, raw.User)

	// Validate default runtime
	if err := legacyValidateDefaultRuntime(team.DefaultRuntime); err != nil {
		return nil, err
	}

	// Validate merge mode
	if err := validateMergeMode(cfg.MergeMode); err != nil {
		return nil, err
	}

	// Default flicknote inline projects
	if len(cfg.Flicknote.InlineProjects) == 0 {
		cfg.Flicknote.InlineProjects = DefaultInlineProjects
	}

	// Side-loaded files
	roles, err := LoadRoles()
	if err != nil {
		return nil, fmt.Errorf("roles.toml: %w", err)
	}
	cfg.Roles = roles

	prompts, err := LoadPrompts()
	if err != nil {
		return nil, fmt.Errorf("prompts.toml: %w", err)
	}
	cfg.Prompts = prompts

	// Projects path
	cfg.ProjectsPath = filepath.Join(defaultConfigDir(), "projects.toml")

	return cfg, nil
}

// resolveVoiceConfigFlat resolves the voice config for the flat Config.
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
	mergedVocab := globalVoice.EffectiveVocabulary(team.VoiceVocabulary, []string{"default"}, allAgentNames)
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

// RenderTemplate resolves {{task-id}} in an arbitrary template string.
func RenderTemplate(tmpl, taskID string, rt runtime.Runtime) string { //nolint:unparam // kept for stable API signature
	return strings.ReplaceAll(tmpl, "{{task-id}}", taskID)
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

// resolvedPaths caches dataDir and projectsPath together from a single config load,
// preventing divergence between the values.
var resolvedPaths struct {
	once         sync.Once
	dir          string
	projectsPath string
}

func legacyEnsureResolvedPaths() {
	resolvedPaths.once.Do(func() {
		cfg, err := Load()
		if err != nil {
			resolvedPaths.dir = defaultDataDir()
			resolvedPaths.projectsPath = filepath.Join(defaultConfigDir(), "projects.toml")
			return
		}
		resolvedPaths.dir = cfg.DataDir
		resolvedPaths.projectsPath = cfg.ProjectsPath
	})
}

// ResolveDataDir returns the data directory for the active team without
// requiring a full config load. Falls back to ~/.ttal if config is unavailable.
// Result is cached after first call.
func ResolveDataDir() string {
	legacyEnsureResolvedPaths()
	return resolvedPaths.dir
}

// ResolveProjectsPathForTeam returns the projects.toml path for a specific team.
func ResolveProjectsPathForTeam(teamName string) string {
	cfgDir := defaultConfigDir()
	if teamName == defaultTeamName {
		return filepath.Join(cfgDir, "projects.toml")
	}
	return filepath.Join(cfgDir, teamName+"-projects.toml")
}

// ResolveProjectsPath returns the projects.toml path for the active team.
func ResolveProjectsPath() string {
	legacyEnsureResolvedPaths()
	return resolvedPaths.projectsPath
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
func DefaultConfigDir() string {
	return defaultConfigDir()
}

// LoadTeamPath reads only the team_path field from config.toml.
// This is a lightweight alternative to Load() for hook contexts — hooks run
// synchronously in taskwarrior's pipeline and cannot afford the overhead of
// Load() which also parses roles.toml and prompts.toml (and fails if they're
// missing required fields).
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
