package doctor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// Level indicates the severity of a check result.
type Level int

const (
	LevelOK Level = iota
	LevelWarn
	LevelError
)

// Check is a single diagnostic result.
type Check struct {
	Name    string
	Level   Level
	Message string
}

// Section groups related checks under a heading.
type Section struct {
	Name   string
	Checks []Check
}

func (s *Section) add(level Level, name, message string) {
	s.Checks = append(s.Checks, Check{Name: name, Level: level, Message: message})
}

// Report contains all diagnostic sections.
type Report struct {
	Sections []Section
}

// Errors returns the total number of error-level checks.
func (r *Report) Errors() int {
	n := 0
	for _, s := range r.Sections {
		for _, c := range s.Checks {
			if c.Level == LevelError {
				n++
			}
		}
	}
	return n
}

// Warnings returns the total number of warning-level checks.
func (r *Report) Warnings() int {
	n := 0
	for _, s := range r.Sections {
		for _, c := range s.Checks {
			if c.Level == LevelWarn {
				n++
			}
		}
	}
	return n
}

// Run executes all diagnostic checks and returns a report.
func Run(fix bool) *Report {
	r := &Report{}
	r.Sections = append(r.Sections, checkPrerequisites())
	r.Sections = append(r.Sections, checkConfig(fix))
	r.Sections = append(r.Sections, checkTaskwarrior(fix))
	r.Sections = append(r.Sections, checkTaskSync(fix))
	r.Sections = append(r.Sections, checkDatabase())
	r.Sections = append(r.Sections, checkDaemon())
	r.Sections = append(r.Sections, checkEnvironment())
	r.Sections = append(r.Sections, checkVoice())
	r.Sections = append(r.Sections, checkMatrix(fix))
	r.Sections = append(r.Sections, checkCCIntegration(fix))
	r.Sections = append(r.Sections, checkHooks(fix))
	r.Sections = append(r.Sections, checkPipelines(fix))
	return r
}

// Print renders the report with ANSI colors.
func Print(report *Report) {
	for _, section := range report.Sections {
		fmt.Printf("\n\033[1m%s\033[0m\n", section.Name)
		for _, check := range section.Checks {
			switch check.Level {
			case LevelOK:
				fmt.Printf("  \033[32m✓\033[0m %s\n", check.Message)
			case LevelWarn:
				fmt.Printf("  \033[33m!\033[0m %s\n", check.Message)
			case LevelError:
				fmt.Printf("  \033[31m✗\033[0m %s\n", check.Message)
			}
		}
	}

	errors := report.Errors()
	warnings := report.Warnings()
	fmt.Println()
	if errors == 0 && warnings == 0 {
		fmt.Println("\033[32mAll checks passed.\033[0m")
	} else {
		fmt.Printf("%d errors, %d warnings\n", errors, warnings)
	}
}

// --- Prerequisites ---

type prerequisite struct {
	name     string
	bin      string
	required bool
	hint     string
}

var prerequisites = []prerequisite{
	{"tmux", "tmux", true, "brew install tmux"},
	{"taskwarrior", "task", true, "brew install task"},
	{"git", "git", true, "brew install git"},
	{"ffmpeg", "ffmpeg", false, "brew install ffmpeg (needed for voice)"},
}

func checkPrerequisites() Section {
	section := Section{Name: "Prerequisites"}
	for _, tool := range prerequisites {
		if _, err := exec.LookPath(tool.bin); err != nil {
			level := LevelError
			if !tool.required {
				level = LevelWarn
			}
			section.add(level, tool.name,
				fmt.Sprintf("%s not found — install: %s", tool.bin, tool.hint))
			continue
		}
		version := getToolVersion(tool.bin)
		section.add(LevelOK, tool.name,
			fmt.Sprintf("%s installed (%s)", tool.bin, version))
	}
	return section
}

func getToolVersion(bin string) string {
	flag := "--version"
	switch bin {
	case "tmux":
		flag = "-V"
	case "ffmpeg":
		flag = "-version"
	}

	out, err := exec.Command(bin, flag).CombinedOutput()
	if err != nil || len(out) == 0 {
		return "unknown"
	}

	// Strip ANSI escape codes
	line := stripANSI(strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]))

	for _, prefix := range []string{
		"tmux ",           // "tmux 3.6a"
		"git version ",    // "git version 2.47.1"
		"task ",           // "task 3.1.0"
		"ffmpeg version ", // "ffmpeg version 7.1 ..."
	} {
		if strings.HasPrefix(line, prefix) {
			v := strings.TrimPrefix(line, prefix)
			// Trim anything after a space (e.g. copyright text)
			if i := strings.IndexByte(v, ' '); i > 0 {
				v = v[:i]
			}
			return v
		}
	}

	return line
}

func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// --- Config ---

func checkConfig(fix bool) Section {
	section := Section{Name: "Config"}

	cfgPath, err := config.Path()
	if err != nil {
		section.add(LevelError, "config", fmt.Sprintf("cannot determine config path: %v", err))
		return section
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if fix {
			section.add(LevelError, "config", fmt.Sprintf("%s not found (run: ttal init)", cfgPath))
		} else {
			section.add(LevelError, "config", fmt.Sprintf("%s not found (run: ttal doctor --fix)", cfgPath))
		}
		return section
	}

	// Check for deprecated default_runtime before loading
	rawContent, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(rawContent), "default_runtime") {
		section.add(LevelError, "default_runtime",
			"deprecated: rename default_runtime to worker_runtime (and add agent_runtime if needed)")
	}

	cfg, err := config.Load()
	if err != nil {
		section.add(LevelError, "config", fmt.Sprintf("invalid config: %v", err))
		return section
	}

	section.add(LevelOK, "config", cfgPath+" exists")

	// Warn if [prompts] section still exists in config.toml (moved to prompts.toml).
	// Check line-by-line to avoid false positives from comments or string values.
	for _, line := range strings.Split(string(rawContent), "\n") {
		if strings.TrimSpace(line) == "[prompts]" {
			section.add(LevelWarn, "prompts_migration",
				"[prompts] section found in config.toml — move to ~/.config/ttal/prompts.toml")
			break
		}
	}

	checkPrompts(&section, cfg.Prompts)

	if cfg.ChatID == "" {
		section.add(LevelError, "chat_id", "chat_id not set")
	} else {
		section.add(LevelOK, "chat_id", "chat_id set")
	}

	if cfg.NotificationToken == "" {
		section.add(LevelWarn, "notification_token",
			"notification bot token not configured (set {TEAM}_NOTIFICATION_BOT_TOKEN in .env)")
	} else {
		section.add(LevelOK, "notification_token", "notification bot token configured")
	}

	checkDotEnv(&section, cfg, fix)

	return section
}

// checkPrompts verifies that prompts.toml was loaded and has required keys.
// Receives the already-loaded PromptsConfig from cfg.Prompts to avoid a second disk read.
func checkPrompts(section *Section, prompts config.PromptsConfig) {
	if prompts.Execute == "" && prompts.Review == "" {
		section.add(LevelWarn, "prompts",
			"prompts.toml not found or missing 'execute'/'review' keys — create ~/.config/ttal/prompts.toml")
		return
	}
	section.add(LevelOK, "prompts", "prompts.toml loaded")
}

// checkDotEnv verifies ~/.config/ttal/.env exists and agents have bot tokens.
func checkDotEnv(section *Section, cfg *config.Config, fix bool) {
	envPath, _ := config.DotEnvPath()

	// Discover agent names from filesystem for .env template and token checks.
	agentNames, _ := agentfs.DiscoverAgents(cfg.TeamPath())
	sort.Strings(agentNames)

	if _, statErr := os.Stat(envPath); os.IsNotExist(statErr) {
		if fix {
			lines := make([]string, 0, 8+len(agentNames))
			lines = append(lines, "# ttal secrets — ~/.config/ttal/.env")
			lines = append(lines, "# All entries are injected into worker and agent sessions.")
			lines = append(lines, "")
			lines = append(lines, "# API tokens")
			lines = append(lines, "GITHUB_TOKEN=")
			lines = append(lines, "FORGEJO_TOKEN=")
			lines = append(lines, "")
			lines = append(lines, "# Woodpecker CI")
			lines = append(lines, "WOODPECKER_URL=")
			lines = append(lines, "WOODPECKER_TOKEN=")
			lines = append(lines, "")
			lines = append(lines, "# Bot tokens — convention: {UPPER_AGENT}_BOT_TOKEN")
			for _, name := range agentNames {
				envKey := strings.ToUpper(name) + "_BOT_TOKEN"
				lines = append(lines, envKey+"=")
			}
			content := strings.Join(lines, "\n") + "\n"
			if writeErr := os.MkdirAll(filepath.Dir(envPath), 0o755); writeErr != nil {
				section.add(LevelError, "dotenv", fmt.Sprintf("failed to create dir: %v", writeErr))
			} else if writeErr := os.WriteFile(envPath, []byte(content), 0o600); writeErr != nil {
				section.add(LevelError, "dotenv", fmt.Sprintf("failed to create .env: %v", writeErr))
			} else {
				section.add(LevelWarn, "dotenv", fmt.Sprintf("created template .env: %s", envPath))
			}
		} else {
			section.add(LevelError, "dotenv",
				fmt.Sprintf(".env file missing: %s (run: ttal doctor --fix)", envPath))
		}
	} else {
		section.add(LevelOK, "dotenv", fmt.Sprintf(".env file: %s", envPath))
	}

	// Check bot tokens using convention: {UPPER_NAME}_BOT_TOKEN.
	for _, name := range agentNames {
		if config.AgentBotToken(name) == "" {
			section.add(LevelError, name,
				fmt.Sprintf("Agent %s: bot token not found in .env", name))
		} else {
			section.add(LevelOK, name, fmt.Sprintf("Agent %s: bot_token set", name))
		}
	}

	// Check common API tokens (warn, not error — not all setups need both)
	env, loadErr := config.LoadDotEnv()
	if loadErr != nil {
		section.add(LevelWarn, "dotenv", fmt.Sprintf(".env read error: %v", loadErr))
		return
	}
	if env["GITHUB_TOKEN"] == "" {
		section.add(LevelWarn, "github_token", "GITHUB_TOKEN not set in .env")
	} else {
		section.add(LevelOK, "github_token", "GITHUB_TOKEN set in .env")
	}
	if env["FORGEJO_TOKEN"] == "" {
		section.add(LevelWarn, "forgejo_token", "FORGEJO_TOKEN not set in .env")
	} else {
		section.add(LevelOK, "forgejo_token", "FORGEJO_TOKEN set in .env")
	}
	if env["WOODPECKER_URL"] == "" {
		section.add(LevelWarn, "woodpecker_url", "WOODPECKER_URL not set in .env")
	} else {
		section.add(LevelOK, "woodpecker_url", "WOODPECKER_URL set in .env")
	}
	if env["WOODPECKER_TOKEN"] == "" {
		section.add(LevelWarn, "woodpecker_token", "WOODPECKER_TOKEN not set in .env")
	} else {
		section.add(LevelOK, "woodpecker_token", "WOODPECKER_TOKEN set in .env")
	}
}

// --- Taskwarrior UDAs ---

func checkTaskwarrior(fix bool) Section {
	section := Section{Name: "Taskwarrior UDAs"}

	cfg, err := config.Load()
	if err != nil {
		section.add(LevelError, "config", fmt.Sprintf("failed to load config: %v", err))
		return section
	}
	taskrc := cfg.TaskRC()

	home, _ := os.UserHomeDir()
	taskrcTtalPath := home + "/.taskrc.ttal"

	if !ensureTaskrc(&section, cfg, taskrc, fix) {
		return section
	}

	ensureCompanionFile(&section, taskrcTtalPath, ".taskrc.ttal",
		"~/.taskrc.ttal exists", "created with UDA definitions", taskrcTtalContent, fix)

	if !checkTaskrcIncludes(&section, cfg, taskrc, fix) {
		return section
	}
	checkUDAs(&section, taskrcTtalPath)
	checkTaskDataDir(&section, cfg, fix)

	return section
}

// ensureTaskrc checks that the taskrc file exists and creates it if --fix is set.
// Returns false if further checks should be skipped.
func ensureTaskrc(section *Section, cfg *config.Config, taskrc string, fix bool) bool {
	if _, err := os.Stat(taskrc); err == nil {
		return true
	}
	if !fix {
		section.add(LevelError, ".taskrc",
			fmt.Sprintf("%s not found (run: ttal doctor --fix)", taskrc))
		return false
	}
	taskData := cfg.TaskData()
	if err := os.MkdirAll(taskData, 0o755); err != nil {
		section.add(LevelError, "task_data",
			fmt.Sprintf("failed to create data dir %s: %v", taskData, err))
		return false
	}
	if err := os.MkdirAll(filepath.Dir(taskrc), 0o755); err != nil {
		section.add(LevelError, ".taskrc",
			fmt.Sprintf("failed to create parent dir: %v", err))
		return false
	}
	taskrcContent := "# Auto-generated by ttal doctor --fix\n" +
		"data.location=" + taskData + "\n" +
		"news.version=3.4.2\n\n" +
		"include ~/.taskrc.ttal\n"
	if cfg.TaskSyncURL() != "" {
		syncFilePath := filepath.Join(cfg.DataDir(), "taskrc.sync")
		taskrcContent += "include " + syncFilePath + "\n"
	}
	if err := os.WriteFile(taskrc, []byte(taskrcContent), 0o644); err != nil {
		section.add(LevelError, ".taskrc",
			fmt.Sprintf("failed to create %s: %v", taskrc, err))
		return false
	}
	section.add(LevelWarn, ".taskrc",
		fmt.Sprintf("created %s with data.location=%s", taskrc, taskData))
	return true
}

// ensureCompanionFile checks that a companion file exists and optionally creates it.
func ensureCompanionFile(section *Section, path, name, okMsg, createdMsg, content string, fix bool) {
	if _, err := os.Stat(path); err == nil {
		section.add(LevelOK, name, okMsg)
		return
	}
	if !fix {
		section.add(LevelError, name, "not found (run: ttal doctor --fix)")
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		section.add(LevelError, name, fmt.Sprintf("failed to create parent dir: %v", err))
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		section.add(LevelError, name, fmt.Sprintf("failed to create: %v", err))
	} else {
		section.add(LevelWarn, name, createdMsg)
	}
}

// checkTaskrcIncludes verifies include lines in .taskrc. Returns false on read error.
func checkTaskrcIncludes(section *Section, cfg *config.Config, taskrc string, fix bool) bool {
	content, err := os.ReadFile(taskrc)
	if err != nil {
		section.add(LevelError, ".taskrc",
			fmt.Sprintf("cannot read .taskrc: %v", err))
		return false
	}
	checkTaskrcInclude(section, taskrc, string(content), "include ~/.taskrc.ttal", fix)
	syncFilePath := filepath.Join(cfg.DataDir(), "taskrc.sync")
	if _, err := os.Stat(syncFilePath); err == nil {
		syncInc := "include " + syncFilePath
		checkTaskrcInclude(section, taskrc, string(content), syncInc, fix)
	}
	return true
}

// checkUDAs verifies UDA definitions exist in .taskrc.ttal.
func checkUDAs(section *Section, taskrcTtalPath string) {
	ttalContent, err := os.ReadFile(taskrcTtalPath)
	if err != nil {
		section.add(LevelError, ".taskrc.ttal", fmt.Sprintf("cannot read: %v", err))
		return
	}
	for _, uda := range []string{"pr_id", "spawner"} {
		if strings.Contains(string(ttalContent), "uda."+uda+".type") {
			section.add(LevelOK, uda, "UDA "+uda+" defined")
		} else {
			section.add(LevelWarn, uda, "UDA "+uda+" not found in .taskrc.ttal")
		}
	}
}

// checkTaskDataDir verifies the task data directory exists.
func checkTaskDataDir(section *Section, cfg *config.Config, fix bool) {
	taskData, tdErr := taskwarrior.ResolveDataLocation()
	if tdErr != nil {
		taskData = cfg.TaskData()
	}
	if _, err := os.Stat(taskData); err == nil {
		section.add(LevelOK, "task_data",
			fmt.Sprintf("data directory %s exists", taskData))
		return
	}
	if !fix {
		section.add(LevelError, "task_data",
			fmt.Sprintf("data directory %s not found (run: ttal doctor --fix)", taskData))
		return
	}
	if err := os.MkdirAll(taskData, 0o755); err != nil {
		section.add(LevelError, "task_data",
			fmt.Sprintf("failed to create %s: %v", taskData, err))
	} else {
		section.add(LevelWarn, "task_data",
			fmt.Sprintf("created data directory %s", taskData))
	}
}

func checkTaskrcInclude(section *Section, taskrc, content, inc string, fix bool) {
	if strings.Contains(content, inc) {
		section.add(LevelOK, "include", "~/.taskrc includes "+strings.TrimPrefix(inc, "include "))
		return
	}
	if !fix {
		section.add(LevelError, "include", "~/.taskrc missing: "+inc)
		return
	}
	f, err := os.OpenFile(taskrc, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		section.add(LevelError, "include", fmt.Sprintf("failed to update .taskrc: %v", err))
		return
	}
	if _, err := f.WriteString("\n" + inc + "\n"); err != nil {
		f.Close()
		section.add(LevelError, "include", fmt.Sprintf("failed to write to .taskrc: %v", err))
		return
	}
	if err := f.Close(); err != nil {
		section.add(LevelError, "include", fmt.Sprintf("failed to close .taskrc: %v", err))
		return
	}
	section.add(LevelWarn, "include", "added "+inc+" to .taskrc")
}

const taskrcTtalContent = `# TTAL Worker UDAs
uda.pr_id.type=string
uda.pr_id.label=PR ID

uda.spawner.type=string
uda.spawner.label=Spawner
`

// --- TaskChampion Sync ---

func checkTaskSync(fix bool) Section {
	section := Section{Name: "TaskChampion Sync"}

	cfg, err := config.Load()
	if err != nil {
		section.add(LevelError, "config", fmt.Sprintf("failed to load config: %v", err))
		return section
	}

	syncURL := cfg.TaskSyncURL()
	if syncURL == "" {
		section.add(LevelOK, "sync", "sync not configured (no task_sync_url)")
		return section
	}

	syncFilePath := filepath.Join(cfg.DataDir(), "taskrc.sync")

	if _, err := os.Stat(syncFilePath); err == nil {
		section.add(LevelOK, "credentials", fmt.Sprintf("sync credentials present: %s", syncFilePath))
		return section
	}

	if !fix {
		section.add(LevelWarn, "credentials",
			"sync URL configured but no credentials (run: ttal doctor --fix or ttal sync setup)")
		return section
	}

	if err := GenerateSyncCredentials(cfg.DataDir(), syncURL); err != nil {
		section.add(LevelError, "credentials", fmt.Sprintf("failed to generate sync credentials: %v", err))
		return section
	}

	section.add(LevelWarn, "credentials", fmt.Sprintf("generated sync credentials at %s", syncFilePath))
	return section
}

// GenerateSyncCredentials creates a taskrc.sync file with TaskChampion sync credentials.
// It generates a UUID v4 client_id and a 32-byte random base64-encoded encryption_secret.
func GenerateSyncCredentials(dataDir, syncURL string) error {
	syncFilePath := filepath.Join(dataDir, "taskrc.sync")

	clientID := uuid.New().String()

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return fmt.Errorf("generating encryption secret: %w", err)
	}
	secret := base64.StdEncoding.EncodeToString(secretBytes)

	content := fmt.Sprintf("# TaskChampion sync configuration\n"+
		"# Auto-generated by ttal\n"+
		"# Created: %s\n"+
		"sync.server.url=%s\n"+
		"sync.server.client_id=%s\n"+
		"sync.encryption_secret=%s\n",
		time.Now().Format("2006-01-02"),
		syncURL,
		clientID,
		secret,
	)

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	return os.WriteFile(syncFilePath, []byte(content), 0o600)
}

// --- Projects ---

func checkDatabase() Section {
	section := Section{Name: "Projects"}
	projectsPath := config.ResolveProjectsPath()

	store := project.NewStore(projectsPath)
	projects, err := store.List(false)
	if err != nil {
		section.add(LevelWarn, "projects", fmt.Sprintf("could not read projects: %v", err))
		return section
	}

	if len(projects) == 0 {
		section.add(LevelWarn, "projects", "no projects found (run: ttal project add)")
	} else {
		section.add(LevelOK, "projects", fmt.Sprintf("%d active projects in %s", len(projects), projectsPath))
	}

	// Count agents from filesystem (team_path)
	cfg, err := config.Load()
	if err != nil {
		section.add(LevelWarn, "agents", fmt.Sprintf("could not load config for agent count: %v", err))
	} else {
		count, err := countAgents(cfg.TeamPath())
		if err != nil {
			section.add(LevelWarn, "agents", fmt.Sprintf("could not count agents: %v", err))
		} else {
			section.add(LevelOK, "agents", fmt.Sprintf("%d agents discovered", count))
		}
	}

	return section
}

// --- Daemon ---

func checkDaemon() Section {
	section := Section{Name: "Daemon"}

	running, pid, err := daemon.IsRunning()
	if err != nil {
		section.add(LevelWarn, "daemon", fmt.Sprintf("could not check daemon: %v", err))
		return section
	}

	if running {
		section.add(LevelOK, "daemon", fmt.Sprintf("running (pid=%d)", pid))
	} else {
		section.add(LevelError, "daemon", "not running (run: ttal daemon start)")
	}

	sockPath, _ := daemon.SocketPath()
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", sockPath, 2*time.Second)
			},
		},
	}
	resp, err := client.Get("http://daemon/health")
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		section.add(LevelOK, "http", fmt.Sprintf("HTTP server: %s", sockPath))
	} else if running {
		if err == nil {
			resp.Body.Close()
		}
		section.add(LevelWarn, "http", fmt.Sprintf("daemon running but HTTP not responding: %s", sockPath))
	}

	return section
}

// --- Environment ---

func checkEnvironment() Section {
	section := Section{Name: "Environment"}

	if os.Getenv("FORGEJO_TOKEN") != "" {
		section.add(LevelOK, "FORGEJO_TOKEN", "FORGEJO_TOKEN set")
	} else {
		section.add(LevelError, "FORGEJO_TOKEN", "FORGEJO_TOKEN not set (required for PR operations)")
	}

	return section
}

// --- Voice (optional) ---

func checkVoice() Section {
	section := Section{Name: "Voice (optional)"}

	// Check voice server
	if isVoiceServerRunning() {
		section.add(LevelOK, "voice_server", "voice server running")
	} else {
		section.add(LevelWarn, "voice_server", "voice server not running (ttal voice install to set up)")
	}

	return section
}

// --- CC Integration (optional) ---

func checkCCIntegration(fix bool) Section {
	section := Section{Name: "CC Integration (optional)"}

	home, err := os.UserHomeDir()
	if err != nil {
		section.add(LevelWarn, "cc", fmt.Sprintf("cannot determine home dir: %v", err))
		return section
	}

	settingsPath := home + "/.claude/settings.json"
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			section.add(LevelWarn, "statusline",
				fmt.Sprintf("cannot read settings.json: %v", err))
			return section
		}
		// File does not exist
		if !fix {
			section.add(LevelWarn, "statusline",
				"~/.claude/settings.json not found — run: ttal doctor --fix")
			return section
		}
		initial := map[string]interface{}{
			"statusLine": map[string]interface{}{
				"type":    "command",
				"command": "ttal statusline",
				"padding": 0,
			},
		}
		if err := writeSettingsJSON(settingsPath, initial); err != nil {
			section.add(LevelWarn, "statusline",
				fmt.Sprintf("could not create settings.json: %v", err))
		} else {
			section.add(LevelOK, "statusline",
				"created ~/.claude/settings.json with statusLine (type: command, command: ttal statusline)")
		}
		return section
	}

	if strings.Contains(string(data), "ttal statusline") {
		section.add(LevelOK, "statusline", "statusline configured: ttal statusline")
		return section
	}

	if !fix {
		section.add(LevelWarn, "statusline",
			"statusLine not properly configured — run: ttal doctor --fix")
		return section
	}

	// Merge statusLine key into existing settings
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		section.add(LevelWarn, "statusline",
			fmt.Sprintf("could not parse settings.json: %v", err))
		return section
	}

	statusLine, ok := settings["statusLine"].(map[string]interface{})
	if !ok {
		statusLine = map[string]interface{}{}
	}
	statusLine["type"] = "command"
	statusLine["command"] = "ttal statusline"
	statusLine["padding"] = 0
	settings["statusLine"] = statusLine

	if err := writeSettingsJSON(settingsPath, settings); err != nil {
		section.add(LevelWarn, "statusline",
			fmt.Sprintf("could not update settings.json: %v", err))
	} else {
		section.add(LevelOK, "statusline",
			"set statusLine in ~/.claude/settings.json (type: command, command: ttal statusline, padding: 0)")
	}

	return section
}

// writeSettingsJSON marshals v to indented JSON and writes to path (0o644).
// Creates parent directory if needed.
func writeSettingsJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// --- Hooks ---

func checkHooks(fix bool) Section {
	section := Section{Name: "Hooks"}
	checkTaskwarriorHooks(&section, fix)
	checkFlicknoteHooks(&section, fix)
	return section
}

func checkTaskwarriorHooks(section *Section, fix bool) {
	cfg, err := config.Load()
	if err != nil {
		section.add(LevelWarn, "tw-hooks", fmt.Sprintf("cannot load config: %v", err))
		return
	}

	taskDataDir, err := taskwarrior.ResolveDataLocation()
	if err != nil {
		section.add(LevelWarn, "tw-hooks", fmt.Sprintf("taskwarrior data dir not found (%v) — skipping hook check", err))
		return
	}
	hookDir := filepath.Join(taskDataDir, "hooks")
	teamName := cfg.TeamName()

	allPresent := true
	for _, name := range []string{"on-add-ttal", "on-modify-ttal"} {
		if _, statErr := os.Stat(filepath.Join(hookDir, name)); statErr != nil {
			allPresent = false
			break
		}
	}

	if allPresent {
		section.add(LevelOK, "tw-hooks", fmt.Sprintf("taskwarrior hooks installed: %s", hookDir))
		return
	}

	if !fix {
		section.add(LevelError, "tw-hooks",
			"taskwarrior hooks not found (run: ttal doctor --fix)")
		return
	}

	if err := worker.InstallHooks(hookDir, teamName); err != nil {
		section.add(LevelError, "tw-hooks",
			fmt.Sprintf("failed to install taskwarrior hooks: %v", err))
		return
	}
	section.add(LevelWarn, "tw-hooks",
		fmt.Sprintf("installed taskwarrior hooks: %s", hookDir))
}

// cmdHookScript is the shared template for on-add and on-modify hooks.
// Both extract the command name from the $2 arg (e.g. "command:replace" → "replace").
const cmdHookScript = `#!/bin/sh
# FlickNote hook — logs usage to ttal worklog.
# Installed by: ttal doctor --fix
CMD=$(echo "$2" | sed 's/^command://')
ttal usage log flicknote "$CMD" 2>/dev/null
exit 0
`

// flicknoteHooks maps hook file names to their script content.
// Each script extracts the command from $2 (e.g., "command:replace" → "replace")
// and logs it via ttal usage log. Always exits 0 to never reject operations.
//
// on-archive is special: logs coarse "archive" regardless of archive/unarchive.
var flicknoteHooks = []struct {
	File   string
	Script string
}{
	{"on-add", cmdHookScript},
	{"on-modify", cmdHookScript},
	{"on-archive", `#!/bin/sh
# FlickNote hook — logs usage to ttal worklog.
# Installed by: ttal doctor --fix
# Logs both archive and unarchive as "archive".
ttal usage log flicknote archive 2>/dev/null
exit 0
`},
	{"on-get", `#!/bin/sh
# FlickNote hook — logs usage to ttal worklog.
# Installed by: ttal doctor --fix
ttal usage log flicknote get 2>/dev/null
exit 0
`},
}

func checkFlicknoteHooks(section *Section, fix bool) {
	// Check if flicknote is installed — skip section entirely if not
	if _, err := exec.LookPath("flicknote"); err != nil {
		section.add(LevelWarn, "fn-hooks", "flicknote not installed (skipping hook check)")
		return
	}

	hooksDir, err := config.FlicknoteHooksDir()
	if err != nil {
		section.add(LevelWarn, "fn-hooks", fmt.Sprintf("cannot determine hooks dir: %v", err))
		return
	}

	for _, h := range flicknoteHooks {
		hookPath := filepath.Join(hooksDir, h.File)

		data, readErr := os.ReadFile(hookPath)
		if readErr != nil && !os.IsNotExist(readErr) {
			section.add(LevelError, h.File,
				fmt.Sprintf("cannot read %s: %v", hookPath, readErr))
			continue
		}

		if readErr == nil {
			// File exists — check if it's ours via the exact sentinel line
			if strings.Contains(string(data), "# Installed by: ttal doctor --fix") {
				section.add(LevelOK, h.File, fmt.Sprintf("flicknote hook: %s", hookPath))
				continue
			}
			// Exists but not installed by ttal
			if !fix {
				section.add(LevelWarn, h.File,
					fmt.Sprintf("flicknote hook %s exists but not managed by ttal", h.File))
				continue
			}
			// Back up existing hook (warn if .bak already exists — it will be overwritten)
			backupPath := hookPath + ".bak"
			if _, bakErr := os.Stat(backupPath); bakErr == nil {
				section.add(LevelWarn, h.File, fmt.Sprintf("overwriting existing backup: %s", backupPath))
			}
			if backupErr := os.Rename(hookPath, backupPath); backupErr != nil {
				section.add(LevelError, h.File,
					fmt.Sprintf("failed to backup existing %s: %v", h.File, backupErr))
				continue
			}
			section.add(LevelWarn, h.File, fmt.Sprintf("backed up existing hook: %s", backupPath))
			// Fall through to install below
		}

		// Hook doesn't exist (or was just backed up) — install or report
		if !fix {
			section.add(LevelError, h.File,
				fmt.Sprintf("flicknote hook %s not found (run: ttal doctor --fix)", h.File))
			continue
		}

		if mkErr := os.MkdirAll(hooksDir, 0o755); mkErr != nil {
			section.add(LevelError, h.File,
				fmt.Sprintf("failed to create hooks dir: %v", mkErr))
			continue
		}

		if writeErr := os.WriteFile(hookPath, []byte(h.Script), 0o755); writeErr != nil {
			section.add(LevelError, h.File,
				fmt.Sprintf("failed to write %s: %v", h.File, writeErr))
		} else {
			section.add(LevelWarn, h.File, fmt.Sprintf("installed flicknote hook: %s", hookPath))
		}
	}
}

// --- Pipelines ---

func checkPipelines(fix bool) Section {
	section := Section{Name: "Pipelines"}
	configDir := config.DefaultConfigDir()
	pipelinesPath := filepath.Join(configDir, "pipelines.toml")
	ensureCompanionFile(&section, pipelinesPath, "pipelines.toml",
		"pipelines.toml exists", "created default pipelines.toml", defaultPipelinesContent, fix)
	return section
}
