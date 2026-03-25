package ask

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/tta-lab/logos"
)

//go:embed ask_prompts/project.md
var projectPrompt string

//go:embed ask_prompts/repo.md
var repoPrompt string

//go:embed ask_prompts/url.md
var urlPrompt string

//go:embed ask_prompts/web.md
var webPrompt string

//go:embed ask_prompts/general.md
var generalPrompt string

// Mode identifies the ask operating mode.
type Mode string

const (
	// ModeProject asks about a registered ttal project.
	ModeProject Mode = "project"
	// ModeRepo asks about an open-source repository (auto-clone/pull).
	ModeRepo Mode = "repo"
	// ModeURL asks about a web page using url for pre-fetching.
	ModeURL Mode = "url"
	// ModeWeb searches the web to answer a question.
	ModeWeb Mode = "web"
	// ModeGeneral asks about the current working directory with both filesystem and web tools.
	ModeGeneral Mode = "general"
)

// Valid reports whether m is a known ask mode.
func (m Mode) Valid() bool {
	switch m {
	case ModeProject, ModeRepo, ModeURL, ModeWeb, ModeGeneral:
		return true
	}
	return false
}

// ModeParams holds mode-specific parameters for prompt building.
type ModeParams struct {
	WorkingDir    string // CWD or project path
	ProjectPath   string // resolved project path (project mode)
	RepoLocalPath string // local clone path (repo mode)
	RawURL        string // URL to explore (url mode)
	Question      string // the user's question (used by web mode for {query})
}

// BuildSystemPromptForMode constructs the full system prompt for the given mode.
// Returns the system prompt string and the list of CommandDocs to expose.
func BuildSystemPromptForMode(mode Mode, params ModeParams) (string, []logos.CommandDoc, error) {
	promptData := logos.PromptData{
		WorkingDir: params.WorkingDir,
		Platform:   runtime.GOOS,
		Date:       time.Now().Format("2006-01-02"),
	}

	var extra string
	var commands []logos.CommandDoc

	switch mode {
	case ModeProject:
		if params.ProjectPath == "" {
			return "", nil, fmt.Errorf("project path required for project mode")
		}
		extra = strings.ReplaceAll(projectPrompt, "{projectPath}", params.ProjectPath)
		commands = AllCommands()
		promptData.WorkingDir = params.ProjectPath
		promptData.Commands = commands
	case ModeRepo:
		if params.RepoLocalPath == "" {
			return "", nil, fmt.Errorf("repo local path required for repo mode")
		}
		extra = strings.ReplaceAll(repoPrompt, "{localPath}", params.RepoLocalPath)
		commands = AllCommands()
		promptData.WorkingDir = params.RepoLocalPath
		promptData.Commands = commands
	case ModeURL:
		extra = strings.ReplaceAll(urlPrompt, "{rawURL}", params.RawURL)
		commands = NetworkCommands()
		promptData.Commands = commands
	case ModeWeb:
		extra = strings.ReplaceAll(webPrompt, "{query}", params.Question)
		commands = NetworkCommands()
		promptData.Commands = commands
	case ModeGeneral:
		extra = strings.ReplaceAll(generalPrompt, "{cwd}", params.WorkingDir)
		commands = AllCommands()
		promptData.Commands = commands
	default:
		return "", nil, fmt.Errorf("unknown ask mode: %s", mode)
	}

	systemPrompt, err := logos.BuildSystemPrompt(promptData)
	if err != nil {
		return "", nil, fmt.Errorf("build system prompt: %w", err)
	}
	if extra != "" {
		systemPrompt += "\n\n" + extra
	}
	return systemPrompt, commands, nil
}
