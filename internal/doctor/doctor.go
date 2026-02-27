package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/daemon"
	"codeberg.org/clawteam/ttal-cli/internal/db"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
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
	r.Sections = append(r.Sections, checkDatabase())
	r.Sections = append(r.Sections, checkDaemon())
	r.Sections = append(r.Sections, checkEnvironment())
	r.Sections = append(r.Sections, checkVoice())
	r.Sections = append(r.Sections, checkCCIntegration())
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
			if err := config.WriteTemplate(); err != nil {
				section.add(LevelError, "config", fmt.Sprintf("failed to create template: %v", err))
			} else {
				section.add(LevelWarn, "config", fmt.Sprintf("created template at %s — edit before continuing", cfgPath))
			}
		} else {
			section.add(LevelError, "config", fmt.Sprintf("%s not found (run: ttal doctor --fix)", cfgPath))
		}
		return section
	}

	cfg, err := config.Load()
	if err != nil {
		section.add(LevelError, "config", fmt.Sprintf("invalid config: %v", err))
		return section
	}

	section.add(LevelOK, "config", cfgPath+" exists")

	if cfg.ChatID == "" || cfg.ChatID == "TODO" {
		section.add(LevelError, "chat_id", "chat_id not set")
	} else {
		section.add(LevelOK, "chat_id", "chat_id set")
	}

	if cfg.LifecycleAgent == "" {
		section.add(LevelWarn, "lifecycle_agent", "lifecycle_agent not set")
	} else {
		section.add(LevelOK, "lifecycle_agent", "lifecycle_agent: "+cfg.LifecycleAgent)
	}

	for name, ac := range cfg.Agents {
		if ac.BotToken == "" || ac.BotToken == "TODO" {
			section.add(LevelError, name, fmt.Sprintf("Agent %s: bot_token missing", name))
		} else {
			chatSource := "set"
			if ac.ChatID == "" {
				chatSource = "inherited"
			}
			section.add(LevelOK, name, fmt.Sprintf("Agent %s: bot_token ✓, chat_id ✓ (%s)", name, chatSource))
		}
	}

	return section
}

// --- Taskwarrior UDAs ---

func checkTaskwarrior(fix bool) Section {
	section := Section{Name: "Taskwarrior UDAs"}

	// Resolve taskrc from active team config
	cfg, err := config.Load()
	if err != nil {
		section.add(LevelError, "config", fmt.Sprintf("failed to load config: %v", err))
		return section
	}
	taskrc := cfg.TaskRC()

	home, _ := os.UserHomeDir()
	// Companion files (.taskrc.ttal, .taskrc.tasktui) live next to the default taskrc
	// and are shared across teams (they define UDAs, not team-specific config).
	taskrcTtalPath := home + "/.taskrc.ttal"

	// Check .taskrc exists — auto-create if missing
	if _, err := os.Stat(taskrc); os.IsNotExist(err) {
		if !fix {
			section.add(LevelError, ".taskrc", fmt.Sprintf("%s not found (run: ttal doctor --fix)", taskrc))
			return section
		}
		taskData := cfg.TaskData()
		if err := os.MkdirAll(taskData, 0o755); err != nil {
			section.add(LevelError, "task_data", fmt.Sprintf("failed to create data dir %s: %v", taskData, err))
			return section
		}
		// Ensure the taskrc's parent directory exists (for convention paths like ~/.ttal/guion/taskrc)
		if err := os.MkdirAll(filepath.Dir(taskrc), 0o755); err != nil {
			section.add(LevelError, ".taskrc", fmt.Sprintf("failed to create parent dir: %v", err))
			return section
		}
		taskrcContent := "# Auto-generated by ttal doctor --fix\n" +
			"data.location=" + taskData + "\n" +
			"news.version=3.4.2\n\n" +
			"include ~/.taskrc.ttal\n" +
			"include ~/.taskrc.tasktui\n"
		if err := os.WriteFile(taskrc, []byte(taskrcContent), 0o644); err != nil {
			section.add(LevelError, ".taskrc", fmt.Sprintf("failed to create %s: %v", taskrc, err))
			return section
		}
		section.add(LevelWarn, ".taskrc", fmt.Sprintf("created %s with data.location=%s", taskrc, taskData))
	}

	// Check .taskrc.ttal exists
	if _, err := os.Stat(taskrcTtalPath); os.IsNotExist(err) {
		if fix {
			if err := os.WriteFile(taskrcTtalPath, []byte(taskrcTtalContent), 0o644); err != nil {
				section.add(LevelError, ".taskrc.ttal", fmt.Sprintf("failed to create: %v", err))
			} else {
				section.add(LevelWarn, ".taskrc.ttal", "created with UDA definitions")
			}
		} else {
			section.add(LevelError, ".taskrc.ttal", "not found (run: ttal doctor --fix)")
		}
	} else {
		section.add(LevelOK, ".taskrc.ttal", "~/.taskrc.ttal exists")
	}

	// Check .taskrc.tasktui exists
	taskrcTasktuiPath := home + "/.taskrc.tasktui"
	if _, err := os.Stat(taskrcTasktuiPath); os.IsNotExist(err) {
		if fix {
			if err := os.WriteFile(taskrcTasktuiPath, []byte(taskrcTasktuiContent), 0o644); err != nil {
				section.add(LevelError, ".taskrc.tasktui", fmt.Sprintf("failed to create: %v", err))
			} else {
				section.add(LevelWarn, ".taskrc.tasktui", "created with taskwarrior-tui shortcuts")
			}
		} else {
			section.add(LevelError, ".taskrc.tasktui", "not found (run: ttal doctor --fix)")
		}
	} else {
		section.add(LevelOK, ".taskrc.tasktui", "~/.taskrc.tasktui exists")
	}

	// Check include lines in .taskrc
	content, err := os.ReadFile(taskrc)
	if err != nil {
		section.add(LevelError, ".taskrc", fmt.Sprintf("cannot read .taskrc: %v", err))
		return section
	}
	for _, inc := range []string{"include ~/.taskrc.ttal", "include ~/.taskrc.tasktui"} {
		checkTaskrcInclude(&section, taskrc, string(content), inc, fix)
	}

	// Check individual UDAs exist in .taskrc.ttal
	ttalContent, err := os.ReadFile(taskrcTtalPath)
	if err != nil {
		section.add(LevelError, ".taskrc.ttal", fmt.Sprintf("cannot read: %v", err))
		return section
	}
	for _, uda := range []string{"branch", "project_path", "pr_id"} {
		if strings.Contains(string(ttalContent), "uda."+uda+".type") {
			section.add(LevelOK, uda, "UDA "+uda+" defined")
		} else {
			section.add(LevelWarn, uda, "UDA "+uda+" not found in .taskrc.ttal")
		}
	}

	// Check task data directory exists (use taskwarrior's resolved location)
	taskData, tdErr := taskwarrior.ResolveDataLocation()
	if tdErr != nil {
		// Fall back to config-derived path if taskwarrior isn't available yet
		taskData = cfg.TaskData()
	}
	if _, err := os.Stat(taskData); os.IsNotExist(err) {
		if fix {
			if err := os.MkdirAll(taskData, 0o755); err != nil {
				section.add(LevelError, "task_data", fmt.Sprintf("failed to create %s: %v", taskData, err))
			} else {
				section.add(LevelWarn, "task_data", fmt.Sprintf("created data directory %s", taskData))
			}
		} else {
			section.add(LevelError, "task_data", fmt.Sprintf("data directory %s not found (run: ttal doctor --fix)", taskData))
		}
	} else {
		section.add(LevelOK, "task_data", fmt.Sprintf("data directory %s exists", taskData))
	}

	return section
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

const taskrcTasktuiContent = `# Taskwarrior-tui Shortcuts & Keybindings
uda.taskwarrior-tui.shortcuts.1=ttal open session
uda.taskwarrior-tui.shortcuts.2=ttal open pr
uda.taskwarrior-tui.shortcuts.3=ttal open term

# Map shortcuts to keys
uda.taskwarrior-tui.keyconfig.shortcut1=z
uda.taskwarrior-tui.keyconfig.shortcut2=p
uda.taskwarrior-tui.keyconfig.shortcut3=o
`

const taskrcTtalContent = `# TTAL Worker UDAs
uda.branch.type=string
uda.branch.label=Branch

uda.project_path.type=string
uda.project_path.label=Project

uda.pr_id.type=string
uda.pr_id.label=PR ID
`

// --- Database ---

func checkDatabase() Section {
	section := Section{Name: "Database"}
	dbPath := db.DefaultPath()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		section.add(LevelError, "database", fmt.Sprintf("%s not found", dbPath))
		return section
	}

	section.add(LevelOK, "database", dbPath+" exists")

	count, err := countAgents(dbPath)
	if err != nil {
		section.add(LevelWarn, "agents", fmt.Sprintf("could not count agents: %v", err))
	} else {
		section.add(LevelOK, "agents", fmt.Sprintf("%d agents registered", count))
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
	if _, err := os.Stat(sockPath); err == nil {
		section.add(LevelOK, "socket", sockPath)
	} else if running {
		section.add(LevelWarn, "socket", fmt.Sprintf("%s not found (daemon running but socket missing?)", sockPath))
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

func checkCCIntegration() Section {
	section := Section{Name: "CC Integration (optional)"}

	home, err := os.UserHomeDir()
	if err != nil {
		section.add(LevelWarn, "cc", fmt.Sprintf("cannot determine home dir: %v", err))
		return section
	}

	settingsPath := home + "/.claude/settings.json"
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		section.add(LevelWarn, "statusline", "~/.claude/settings.json not found")
		return section
	}

	if strings.Contains(string(data), "ttal statusline") {
		section.add(LevelOK, "statusline", "statusline_command configured: ttal statusline")
	} else {
		section.add(LevelWarn, "statusline", "statusline_command not set (add to ~/.claude/settings.json)")
	}

	return section
}
