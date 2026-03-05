package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/tta-lab/ttal-cli/internal/license"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// PromptsConfig holds configurable prompt templates for task routing and worker spawn.
// Supports {{task-id}} and {{skill:name}} template variables.
type PromptsConfig struct {
	Design   string `toml:"design" jsonschema:"description=Prompt for design agent"`
	Research string `toml:"research" jsonschema:"description=Prompt for research agent"`
	Test     string `toml:"test" jsonschema:"description=Prompt for test agent"`
	Execute  string `toml:"execute" jsonschema:"description=Prompt prefix for worker spawn"`
	Triage   string `toml:"triage" jsonschema:"description=Prompt sent to coder after PR review. Supports {{review-file}}"`
	Review   string `toml:"review" jsonschema:"description=Initial reviewer prompt. Supports {{pr-number}} {{pr-title}} {{owner}} {{repo}} {{branch}}"` //nolint:lll
	ReReview string `toml:"re_review" jsonschema:"description=Re-review prompt sent to reviewer. Supports {{review-scope}} {{coder-comment}}"`          //nolint:lll
}

// DefaultPrompts returns sensible defaults for all prompt templates.
func DefaultPrompts() PromptsConfig {
	return PromptsConfig{
		Design: `{{skill:sp-writing-plans}}
Write an implementation plan for this task.

When done: task {{task-id}} annotate 'Plan: docs/plans/YYYY-MM-DD-topic.md'`,

		Research: `{{skill:tell-me-more}}
Research this topic thoroughly.

When done: task {{task-id}} annotate 'Research: docs/research/YYYY-MM-DD-topic.md'`,

		Test: `{{skill:sp-tdd}}
Integration test this end-to-end.

When done: task {{task-id}} annotate 'Tested: <pass/fail summary>'`,

		Execute: `{{skill:sp-executing-plans}}
Use the executing-plans skill to implement this plan task-by-task.
Follow each task in order: read the plan, make changes, verify, commit.`,

		Triage: `{{skill:triage}}
PR review posted.{{review-file}} Read it, assess and fix issues.
Post your triage update with ttal pr comment create when done.
If verdict is LGTM and no remaining issues, merge with: ttal pr merge`,

		Review: `You are a code reviewer for PR #{{pr-number}} — "{{pr-title}}" in {{owner}}/{{repo}}.
Branch: {{branch}}

## Your Task

1. Run {{skill:pr-review}} to perform a comprehensive code review
   - Review scope: ONLY changes in this PR (the diff), not the entire codebase
   - Focus on: correctness, security, architecture, tests

2. Structure your findings as a PR comment with clear sections:
   - Critical Issues (must fix before merge)
   - Important Issues (should fix)
   - Suggestions (nice to have)
   - Strengths (what's well done)

3. Post your review using:
   ttal pr comment create "your structured review"

4. End your comment with one of:
   - VERDICT: NEEDS_WORK (if any critical issues)
   - VERDICT: LGTM (if no critical issues)

Do NOT merge the PR. The coder handles merging after triage.

## Important
- Only review what changed in the PR, not pre-existing code
- Be specific: reference file:line for each finding
- Be constructive: suggest fixes, not just problems
- If you're unsure about something, say so rather than raising a false alarm
- NEVER use --no-review flag when posting comments — your review must trigger the coder to triage`,

		ReReview: `Worker has pushed fixes addressing your review.{{coder-comment}} Please re-review:
1. Run {{skill:pr-review}} {{review-scope}}
2. Post updated review via: ttal pr comment create "your review" (NEVER use --no-review)
3. End with VERDICT: LGTM if all issues addressed, or VERDICT: NEEDS_WORK if not`,
	}
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

// FlicknoteConfig holds flicknote-related settings.
type FlicknoteConfig struct {
	InlineProjects []string `toml:"inline_projects" jsonschema:"description=Project substrings to inline (default: plan)"`
}

// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// Requires [teams] sections. After Load(), resolved fields are populated from the active team.
// Callers access ChatID, Agents, etc. without caring about which team is active.
type Config struct {
	// Resolved fields — populated from active team after Load(). Not directly settable in TOML.
	ChatID            string                 `toml:"-" json:"-"`
	LifecycleAgent    string                 `toml:"-" json:"-"` // Deprecated: use NotificationToken instead
	NotificationToken string                 `toml:"-" json:"-"`
	Agents            map[string]AgentConfig `toml:"-" json:"-"`
	Voice             VoiceConfig            `toml:"-" json:"-"`

	// Global fields — not per-team.
	Shell     string          `toml:"shell" jsonschema:"enum=zsh,enum=fish,description=Shell for spawning workers"`
	Sync      SyncConfig      `toml:"sync" jsonschema:"description=Paths for subagent and skill deployment"`
	Prompts   PromptsConfig   `toml:"prompts" jsonschema:"description=Prompt templates for task routing"`
	Flicknote FlicknoteConfig `toml:"flicknote" jsonschema:"description=Flicknote integration settings"`

	// Team-aware fields.
	DefaultTeam string                `toml:"default_team" jsonschema:"description=Active team when TTAL_TEAM env is not set"` //nolint:lll
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
	resolvedProjectsPath  string
	resolvedGatewayURL    string
	resolvedHooksToken    string
	resolvedDesignAgent   string
	resolvedResearchAgent string
	resolvedTestAgent     string
	resolvedTaskSyncURL   string
}

// TeamConfig holds per-team configuration.
type TeamConfig struct {
	TeamPath             string                 `toml:"team_path" jsonschema:"description=Root path for agent workspaces. Agent path = team_path/agent_name."` //nolint:lll
	DataDir              string                 `toml:"data_dir" jsonschema:"description=ttal data directory (default: ~/.ttal/<team>)"`                       //nolint:lll
	TaskRC               string                 `toml:"taskrc" jsonschema:"description=Taskwarrior config file path (default: <data_dir>/taskrc)"`             //nolint:lll
	ChatID               string                 `toml:"chat_id" jsonschema:"description=Telegram chat ID for this team"`
	LifecycleAgent       string                 `toml:"lifecycle_agent" jsonschema:"description=Deprecated: use notification_token_env instead"`                                                    //nolint:lll
	NotificationTokenEnv string                 `toml:"notification_token_env" jsonschema:"description=Override env var for notification bot token (default: {UPPER_TEAM}_NOTIFICATION_BOT_TOKEN)"` //nolint:lll
	AgentRuntime         string                 `toml:"agent_runtime" jsonschema:"enum=claude-code,enum=opencode,enum=codex,enum=openclaw,description=Runtime for agent sessions"`                  //nolint:lll
	WorkerRuntime        string                 `toml:"worker_runtime" jsonschema:"enum=claude-code,enum=opencode,enum=codex,description=Runtime for spawned workers"`                              //nolint:lll
	GatewayURL           string                 `toml:"gateway_url" jsonschema:"description=OpenClaw Gateway URL"`
	HooksToken           string                 `toml:"hooks_token" jsonschema:"description=OpenClaw hooks auth token"`
	MergeMode            string                 `toml:"merge_mode" jsonschema:"enum=auto,enum=manual,description=PR merge mode override for this team"`                  //nolint:lll
	VoiceLanguage        string                 `toml:"voice_language" jsonschema:"description=ISO 639-1 language code for Whisper (default: en; auto for auto-detect)"` //nolint:lll
	DesignAgent          string                 `toml:"design_agent" jsonschema:"description=Design/brainstorm agent"`
	ResearchAgent        string                 `toml:"research_agent" jsonschema:"description=Research agent"`
	TestAgent            string                 `toml:"test_agent" jsonschema:"description=Test writing agent"`
	Agents               map[string]AgentConfig `toml:"agents" jsonschema:"description=Per-agent credentials for this team"`                                  //nolint:lll
	VoiceVocabulary      []string               `toml:"voice_vocabulary" jsonschema:"description=Custom vocabulary words for Whisper transcription accuracy"` //nolint:lll
	TaskSyncURL          string                 `toml:"task_sync_url" jsonschema:"description=TaskChampion sync server URL for ttal doctor --fix"`            //nolint:lll
}

// SyncConfig holds paths for subagent, skill, command, and rule deployment.
type SyncConfig struct {
	SubagentsPaths []string `toml:"subagents_paths" jsonschema:"description=Directories to scan for subagent definitions"`
	SkillsPaths    []string `toml:"skills_paths" jsonschema:"description=Directories to scan for skill definitions"`
	CommandsPaths  []string `toml:"commands_paths" jsonschema:"description=Directories to scan for command definitions"`
	RulesPaths     []string `toml:"rules_paths" jsonschema:"description=Directories to scan for RULE.md files"`
}

// VoiceConfig holds voice-related settings resolved from the active team.
type VoiceConfig struct {
	Vocabulary []string `toml:"vocabulary" jsonschema:"description=Custom vocabulary words for Whisper"`
	Language   string   `toml:"language" jsonschema:"description=ISO 639-1 language code (default: en)"`
}

// AgentConfig holds per-agent Telegram credentials and runtime settings.
type AgentConfig struct {
	// BotToken is resolved from ~/.config/ttal/.env at load time (not stored in TOML).
	BotToken    string `toml:"-" jsonschema:"-"`                                                                                             //nolint:lll
	BotTokenEnv string `toml:"bot_token_env" jsonschema:"description=Override env var name for bot token (default: {UPPER_NAME}_BOT_TOKEN)"` //nolint:lll
	Port        int    `toml:"port" jsonschema:"description=API server port for opencode/codex runtimes"`
	Runtime     string `toml:"runtime" jsonschema:"enum=claude-code,enum=opencode,enum=codex,enum=openclaw,description=Per-agent runtime override (falls back to team agent_runtime)"` //nolint:lll
	Model       string `toml:"model" jsonschema:"enum=haiku,enum=sonnet,enum=opus,description=Claude model tier (falls back to opus)"`                                                 //nolint:lll
}

// AgentRuntimeFor returns the effective runtime for an agent:
// per-agent override > team agent_runtime > claude-code.
func (c *Config) AgentRuntimeFor(agentName string) runtime.Runtime {
	if ac, ok := c.Agents[agentName]; ok && ac.Runtime != "" {
		return runtime.Runtime(ac.Runtime)
	}
	return c.AgentRuntime()
}

// AgentModelFor returns the effective model for an agent (default: "opus").
func (c *Config) AgentModelFor(agentName string) string {
	if ac, ok := c.Agents[agentName]; ok && ac.Model != "" {
		return ac.Model
	}
	return DefaultModel
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

// resolveBotTokens loads .env and populates BotToken for all agents.
// Convention: {UPPER_AGENT}_BOT_TOKEN.
// Override: agent's bot_token_env field takes priority.
// Non-fatal: if .env can't be loaded, tokens remain empty (doctor checks this).
func resolveBotTokens(agents map[string]AgentConfig) {
	env, err := LoadDotEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load .env for bot tokens: %v\n", err)
		return
	}

	for name, ac := range agents {
		envKey := ac.BotTokenEnv
		if envKey == "" {
			envKey = strings.ToUpper(name) + "_BOT_TOKEN"
		}
		ac.BotToken = env[envKey]
		agents[name] = ac
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
	DefaultModel    = "opus"
	MergeModeAuto   = "auto"
	MergeModeManual = "manual"
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

// Prompt returns the prompt template for a given key, falling back to defaults.
func (c *Config) Prompt(key string) string {
	defaults := DefaultPrompts()
	switch key {
	case "design":
		if c.Prompts.Design != "" {
			return c.Prompts.Design
		}
		return defaults.Design
	case "research":
		if c.Prompts.Research != "" {
			return c.Prompts.Research
		}
		return defaults.Research
	case "test":
		if c.Prompts.Test != "" {
			return c.Prompts.Test
		}
		return defaults.Test
	case "execute":
		if c.Prompts.Execute != "" {
			return c.Prompts.Execute
		}
		return defaults.Execute
	case "triage":
		if c.Prompts.Triage != "" {
			return c.Prompts.Triage
		}
		return defaults.Triage
	case "review":
		if c.Prompts.Review != "" {
			return c.Prompts.Review
		}
		return defaults.Review
	case "re_review":
		if c.Prompts.ReReview != "" {
			return c.Prompts.ReReview
		}
		return defaults.ReReview
	default:
		return ""
	}
}

// RenderPrompt resolves {{task-id}} and {{skill:name}} placeholders in a prompt template.
func (c *Config) RenderPrompt(key, taskID string, rt runtime.Runtime) string {
	tmpl := c.Prompt(key)
	return RenderTemplate(tmpl, taskID, rt)
}

// RenderTemplate resolves {{skill:name}} and {{task-id}} in an arbitrary template string.
func RenderTemplate(tmpl, taskID string, rt runtime.Runtime) string {
	result := strings.ReplaceAll(tmpl, "{{task-id}}", taskID)
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
		skillName := result[start+len("{{skill:") : end-2]
		if skillName == "" {
			break
		}
		invocation := runtime.FormatSkillInvocation(rt, skillName)
		result = result[:start] + invocation + result[end:]
	}
	return result
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

	// Enforce team count limit based on license tier.
	if err := checkTeamLicense(len(c.Teams)); err != nil {
		return err
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
	// Note: resolveBotTokens is also called in resolveTeam() for LoadAll().
	// Each path resolves independently — Load() uses resolve(), LoadAll() uses resolveTeam().
	resolveBotTokens(c.Agents)

	// Resolve notification bot token from .env
	c.NotificationToken = resolveNotificationToken(teamName, team.NotificationTokenEnv)
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

	// Resolve ProjectsPath: colocated with config.toml in ~/.config/ttal/
	c.resolvedProjectsPath = projectsPathForTeam(teamName)

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

	// Default flicknote inline projects to ["plan"] if not configured.
	if len(c.Flicknote.InlineProjects) == 0 {
		c.Flicknote.InlineProjects = DefaultInlineProjects
	}

	return c.validateMergeMode()
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

// ResolvedTeam holds a single team's fully resolved config.
type ResolvedTeam struct {
	Name              string
	TeamPath          string
	DataDir           string
	TaskRC            string
	ChatID            string
	LifecycleAgent    string // Deprecated: use NotificationToken instead
	NotificationToken string
	AgentRuntime      string
	WorkerRuntime     string
	MergeMode         string
	GatewayURL        string
	HooksToken        string
	Voice             VoiceConfig
	Agents            map[string]AgentConfig
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
	Config    AgentConfig
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

	for teamName, team := range cfg.Teams {
		rt, err := resolveTeam(teamName, team)
		if err != nil {
			return nil, fmt.Errorf("team %q: %w", teamName, err)
		}
		mcfg.Teams[teamName] = rt
	}

	return mcfg, nil
}

// resolveTeam resolves a single team's config fields.
func resolveTeam(teamName string, team TeamConfig) (*ResolvedTeam, error) {
	if team.TeamPath == "" {
		return nil, fmt.Errorf("missing required field: team_path")
	}

	rt := &ResolvedTeam{
		Name:              teamName,
		TeamPath:          expandHome(team.TeamPath),
		ChatID:            team.ChatID,
		LifecycleAgent:    team.LifecycleAgent,
		NotificationToken: resolveNotificationToken(teamName, team.NotificationTokenEnv),
		AgentRuntime:      team.AgentRuntime,
		WorkerRuntime:     team.WorkerRuntime,
		MergeMode:         team.MergeMode,
		GatewayURL:        team.GatewayURL,
		HooksToken:        team.HooksToken,
		Voice: VoiceConfig{
			Vocabulary: team.VoiceVocabulary,
			Language:   team.VoiceLanguage,
		},
		Agents: team.Agents,
	}

	resolveBotTokens(rt.Agents)

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

	return rt, nil
}

// AllAgents returns all agents across all teams, sorted by team then agent name.
func (m *DaemonConfig) AllAgents() []TeamAgent {
	var agents []TeamAgent
	for teamName, team := range m.Teams {
		for agentName, ac := range team.Agents {
			agents = append(agents, TeamAgent{
				TeamName:  teamName,
				AgentName: agentName,
				Config:    ac,
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

// FindAgent looks up which team an agent belongs to.
// Returns the first match if agent names are unique across teams.
func (m *DaemonConfig) FindAgent(agentName string) (*TeamAgent, bool) {
	for teamName, team := range m.Teams {
		if ac, ok := team.Agents[agentName]; ok {
			ta := TeamAgent{
				TeamName:  teamName,
				AgentName: agentName,
				Config:    ac,
				ChatID:    team.ChatID,
				TeamPath:  team.TeamPath,
			}
			return &ta, true
		}
	}
	return nil, false
}

// FindAgentInTeam looks up an agent within a specific team.
func (m *DaemonConfig) FindAgentInTeam(teamName, agentName string) (*TeamAgent, bool) {
	team, ok := m.Teams[teamName]
	if !ok {
		return nil, false
	}
	ac, ok := team.Agents[agentName]
	if !ok {
		return nil, false
	}
	ta := TeamAgent{
		TeamName:  teamName,
		AgentName: agentName,
		Config:    ac,
		ChatID:    team.ChatID,
		TeamPath:  team.TeamPath,
	}
	return &ta, true
}

// AgentRuntimeForTeam resolves effective runtime for an agent in a team.
func (m *DaemonConfig) AgentRuntimeForTeam(teamName, agentName string) runtime.Runtime {
	team, ok := m.Teams[teamName]
	if !ok {
		return runtime.ClaudeCode
	}
	if ac, ok := team.Agents[agentName]; ok && ac.Runtime != "" {
		return runtime.Runtime(ac.Runtime)
	}
	if team.AgentRuntime != "" {
		return runtime.Runtime(team.AgentRuntime)
	}
	return runtime.ClaudeCode
}

// AgentModelForTeam resolves effective model for an agent in a team.
func (m *DaemonConfig) AgentModelForTeam(teamName, agentName string) string {
	team, ok := m.Teams[teamName]
	if !ok {
		return DefaultModel
	}
	if ac, ok := team.Agents[agentName]; ok && ac.Model != "" {
		return ac.Model
	}
	return DefaultModel
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

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".ttal")
}

func defaultConfigDir() string {
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

# Configurable prompts for task routing and worker spawn.
# Supports {{task-id}} and {{skill:name}} template variables.
# {{skill:name}} resolves to /name (CC/OC) or $name (Codex) based on agent runtime.
[prompts]
design = """
{{skill:sp-writing-plans}}
Write an implementation plan for this task.

When done: task {{task-id}} annotate 'Plan: docs/plans/YYYY-MM-DD-topic.md'"""

research = """
{{skill:tell-me-more}}
Research this topic thoroughly.

When done: task {{task-id}} annotate 'Research: docs/research/YYYY-MM-DD-topic.md'"""

test = """
{{skill:sp-tdd}}
Integration test this end-to-end.

When done: task {{task-id}} annotate 'Tested: <pass/fail summary>'"""

execute = """
{{skill:sp-executing-plans}}
Use the executing-plans skill to implement this plan task-by-task.
Follow each task in order: read the plan, make changes, verify, commit."""

[flicknote]
# inline_projects = ["plan", "fix"]  # project name substrings to inline into worker prompts

[teams.default]
chat_id = ""                 # Telegram chat ID for this team
team_path = ""               # Root path for agent workspaces
design_agent = "inke"        # Agent for ttal task design
research_agent = "athena"    # Agent for ttal task research
# notification_token_env = "DEFAULT_NOTIFICATION_BOT_TOKEN"  # optional override
# test_agent = ""            # Agent for ttal task test
# worker_runtime = "claude-code"
# agent_runtime = "claude-code"
# merge_mode = "auto"

# Bot tokens are stored in ~/.config/ttal/.env (not in this file)
# Convention: {UPPER_AGENT}_BOT_TOKEN for agents, {UPPER_TEAM}_NOTIFICATION_BOT_TOKEN for notifications
# Run 'ttal doctor --fix' to generate a template .env file
[teams.default.agents.kestrel]

# Voice settings go under teams:
# [teams.default.voice]
# vocabulary = ["ttal", "treemd", "taskwarrior"]
# language = "en"
`

	return os.WriteFile(path, []byte(template), 0o600)
}
