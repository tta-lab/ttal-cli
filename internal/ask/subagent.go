package ask

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	internalsync "github.com/tta-lab/ttal-cli/internal/sync"
)

// SubagentRequest is the wire type for POST /subagent/run.
type SubagentRequest struct {
	Name       string            `json:"name"`                  // agent name (matches frontmatter)
	Prompt     string            `json:"prompt"`                // user prompt
	Project    string            `json:"project,omitempty"`     // --project flag
	Repo       string            `json:"repo,omitempty"`        // --repo flag
	MaxSteps   int               `json:"max_steps,omitempty"`   // 0 = config default
	MaxTokens  int               `json:"max_tokens,omitempty"`  // 0 = config default
	SandboxEnv map[string]string `json:"sandbox_env,omitempty"` // --env KEY=VALUE
	WorkingDir string            `json:"working_dir,omitempty"` // CWD when no --project/--repo
}

// claudeMDInstruction is appended to every subagent's system prompt.
// It instructs the agent to discover and read CLAUDE.md / AGENTS.md conventions.
const claudeMDInstruction = `

## Project Conventions

Before starting work, check for CLAUDE.md and AGENTS.md in the project root and subfolders. If found,
read them — they contain project conventions, architecture notes, and coding guidelines you must follow.`

// CommandsForAccess returns the command docs appropriate for the given access level.
// "rw" agents get AllCommands + src edit operations; "ro" agents get AllCommands only.
func CommandsForAccess(access string) []logos.CommandDoc {
	if access == "rw" {
		return RWCommands()
	}
	return AllCommands()
}

// BuildSubagentSandboxPaths constructs AllowedPaths from sandbox.toml + CWD.
// allowWrite paths → rw, allowRead paths → ro, CWD → rw/ro per access field.
// Paths appearing in both lists are deduplicated (rw wins).
func BuildSubagentSandboxPaths(sandbox *config.SandboxConfig, cwd, access string) []logos.AllowedPath {
	cwdReadOnly := access != "rw"

	// Build a deduplicated map: path → readOnly. RW wins over RO.
	seen := make(map[string]bool) // true = readOnly
	var ordered []string

	addPath := func(p string, readOnly bool) {
		if existing, ok := seen[p]; ok {
			// RW wins — upgrade ro to rw if needed.
			if existing && !readOnly {
				seen[p] = false
			}
			return
		}
		seen[p] = readOnly
		ordered = append(ordered, p)
	}

	for _, p := range sandbox.ExpandedAllowWrite() {
		addPath(p, false)
	}
	for _, p := range sandbox.ExpandedAllowRead() {
		addPath(p, true)
	}
	// CWD goes last (may upgrade an existing entry or add a new one).
	addPath(cwd, cwdReadOnly)

	paths := make([]logos.AllowedPath, 0, len(ordered))
	for _, p := range ordered {
		paths = append(paths, logos.AllowedPath{Path: p, ReadOnly: seen[p]})
	}
	return paths
}

// RunSubagent executes a subagent loop server-side.
// This is the daemon-side implementation — temenos is reachable here.
func RunSubagent(ctx context.Context, req SubagentRequest, cfg *config.Config, emit EventFunc) error {
	agent, err := findAgent(req.Name, cfg.Sync.SubagentsPaths)
	if err != nil {
		return err
	}

	access, err := validateAgentAccess(agent, req.Name)
	if err != nil {
		return err
	}

	logosCfg, err := buildSubagentConfig(ctx, req, cfg, agent, access)
	if err != nil {
		return err
	}

	result, err := logos.Run(ctx, *logosCfg, nil, req.Prompt, buildLogosCallbacks(emit))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "max steps") {
			errMsg += "\n\nTip: increase the limit with --max-steps"
		}
		log.Printf("[subagent] logos.Run error: %v", err)
		emit(Event{Type: EventError, Message: errMsg})
		return nil // error already emitted as event
	}

	response := ""
	if result != nil {
		response = result.Response
	}
	emit(Event{Type: EventDone, Response: response})
	return nil
}

// validateAgentAccess checks that the agent has a valid ttal: config block.
func validateAgentAccess(agent *internalsync.ParsedAgent, name string) (string, error) {
	if agent.Frontmatter.Ttal == nil {
		return "", fmt.Errorf(
			"agent %q has no ttal: block — add 'ttal: access: ro' or 'ttal: access: rw'", name)
	}
	access := agent.Frontmatter.Ttal.Access
	if access != "ro" && access != "rw" {
		return "", fmt.Errorf("agent %q has invalid access %q (want ro or rw)", name, access)
	}
	return access, nil
}

// buildSubagentConfig assembles the logos.Config for a subagent run.
func buildSubagentConfig(
	ctx context.Context,
	req SubagentRequest,
	cfg *config.Config,
	agent *internalsync.ParsedAgent,
	access string,
) (*logos.Config, error) {
	model := agent.Frontmatter.Ttal.Model
	if model == "" {
		model = cfg.AskModel()
	}

	provider, modelID, err := BuildProvider(model)
	if err != nil {
		return nil, fmt.Errorf("build provider: %w", err)
	}

	cwd, effectiveAccess, err := resolveSubagentCWD(ctx, req, cfg, access)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := buildSubagentSystemPrompt(agent, cwd, effectiveAccess)
	if err != nil {
		return nil, err
	}

	tc, err := NewTemenosClient(ctx)
	if err != nil {
		return nil, err
	}

	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = cfg.AskMaxSteps()
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = cfg.AskMaxTokens()
	}

	sandbox := config.LoadSandbox()
	allowedPaths := BuildSubagentSandboxPaths(sandbox, cwd, effectiveAccess)

	return &logos.Config{
		Provider:     provider,
		Model:        modelID,
		SystemPrompt: systemPrompt,
		MaxSteps:     maxSteps,
		MaxTokens:    maxTokens,
		Temenos:      tc,
		SandboxEnv:   req.SandboxEnv,
		AllowedPaths: allowedPaths,
	}, nil
}

// buildSubagentSystemPrompt creates the full system prompt for a subagent.
func buildSubagentSystemPrompt(
	agent *internalsync.ParsedAgent, cwd, access string,
) (string, error) {
	promptData := logos.PromptData{
		WorkingDir: cwd,
		Platform:   runtime.GOOS,
		Date:       time.Now().Format("2006-01-02"),
		Commands:   CommandsForAccess(access),
	}
	systemPrompt, err := logos.BuildSystemPrompt(promptData)
	if err != nil {
		return "", fmt.Errorf("build system prompt: %w", err)
	}
	if agent.Body != "" {
		systemPrompt += "\n\n" + agent.Body
	}
	systemPrompt += claudeMDInstruction
	return systemPrompt, nil
}

// findAgent discovers ttal-configured agents and returns the one matching name.
func findAgent(name string, paths []string) (*internalsync.ParsedAgent, error) {
	agents, err := internalsync.DiscoverTtalAgents(paths)
	if err != nil {
		return nil, fmt.Errorf("discover agents: %w", err)
	}
	for _, a := range agents {
		if a.Frontmatter.Name == name {
			return a, nil
		}
	}
	available := make([]string, len(agents))
	for i, a := range agents {
		available[i] = a.Frontmatter.Name
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("agent %q not found (no agents with ttal: frontmatter discovered)", name)
	}
	return nil, fmt.Errorf("agent %q not found — available: %s", name, strings.Join(available, ", "))
}

// resolveSubagentCWD returns the working directory for the subagent based on the request.
func resolveSubagentCWD(ctx context.Context, req SubagentRequest, cfg *config.Config, agentAccess string) (
	cwd, effectiveAccess string, err error,
) {
	switch {
	case req.Project != "" && req.Repo != "":
		return "", "", fmt.Errorf("--project and --repo are mutually exclusive")
	case req.Project != "":
		p, resolveErr := project.GetProjectPath(req.Project)
		if resolveErr != nil {
			return "", "", fmt.Errorf("resolve project: %w", resolveErr)
		}
		return p, agentAccess, nil
	case req.Repo != "":
		cloneURL, localPath, resolveErr := ResolveRepoRef(req.Repo, cfg.AskReferencesPath())
		if resolveErr != nil {
			return "", "", fmt.Errorf("resolve repo: %w", resolveErr)
		}
		if ensureErr := EnsureRepo(ctx, cloneURL, localPath); ensureErr != nil {
			return "", "", fmt.Errorf("ensure repo: %w", ensureErr)
		}
		return localPath, "ro", nil // repos are always read-only
	default:
		if req.WorkingDir == "" {
			return "", "", fmt.Errorf("working_dir required when no --project/--repo specified")
		}
		return req.WorkingDir, agentAccess, nil
	}
}
