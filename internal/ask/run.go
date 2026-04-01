package ask

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// Request is the wire type for POST /ask.
type Request struct {
	Question   string `json:"question"`
	Mode       Mode   `json:"mode"`                  // project, repo, url, web, general
	Project    string `json:"project,omitempty"`     // project alias (mode=project)
	Repo       string `json:"repo,omitempty"`        // repo ref: org/repo or full URL (mode=repo)
	URL        string `json:"url,omitempty"`         // web page URL (mode=url)
	MaxSteps   int    `json:"max_steps,omitempty"`   // 0 = config default
	MaxTokens  int    `json:"max_tokens,omitempty"`  // 0 = config default
	Save       bool   `json:"save,omitempty"`        // save final answer to flicknote
	WorkingDir string `json:"working_dir,omitempty"` // CWD for general mode
}

// EventFunc is the callback for streaming events to the caller.
type EventFunc func(Event)

// RunAsk executes the ask agent loop server-side.
// cfg is the loaded ttal config (daemon has this).
func RunAsk(ctx context.Context, req Request, cfg *config.Config, emit EventFunc) error {
	params, err := resolveParams(ctx, req, cfg, emit)
	if err != nil {
		return fmt.Errorf("resolve params: %w", err)
	}

	systemPrompt, _, err := BuildSystemPromptForMode(req.Mode, params)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	provider, modelID, err := BuildProvider(cfg.AskModel())
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	tc, err := NewTemenosClient(ctx)
	if err != nil {
		return err
	}

	if err := preWarmURL(ctx, tc, req, emit); err != nil {
		return err
	}

	maxSteps := req.MaxSteps
	if maxSteps == 0 {
		maxSteps = cfg.AskMaxSteps()
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = cfg.AskMaxTokens()
	}

	logosCfg := logos.Config{
		Provider:     provider,
		Model:        modelID,
		SystemPrompt: systemPrompt,
		MaxSteps:     maxSteps,
		MaxTokens:    maxTokens,
		Temenos:      tc,
		AllowedPaths: buildAllowedPaths(req.Mode, params),
	}

	question := req.Question
	if req.Mode == ModeURL && req.URL != "" {
		question = fmt.Sprintf("URL: %s\n\nQuestion: %s", req.URL, req.Question)
	}

	result, err := logos.Run(ctx, logosCfg, nil, question, buildLogosCallbacks(emit))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "max steps") {
			errMsg += "\n\nTip: increase the limit with --max-steps"
		}
		log.Printf("[ask] logos.Run error: %v", err)
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

// preWarmURL pre-fetches the URL via temenos before running the agent loop.
// Only active for ModeURL when a URL is provided.
func preWarmURL(ctx context.Context, tc logos.BlockRunner, req Request, emit EventFunc) error {
	if req.Mode != ModeURL || req.URL == "" {
		return nil
	}
	emit(Event{Type: EventStatus, Message: "Fetching " + req.URL + "..."})
	quotedURL := "'" + strings.ReplaceAll(req.URL, "'", "'\\''") + "'"
	resp, err := tc.RunBlock(ctx, logos.RunBlockRequest{Block: "url " + quotedURL})
	if err != nil {
		return fmt.Errorf("pre-fetch %s: %w", req.URL, err)
	}
	// Check if any command in the block failed
	for _, result := range resp.Results {
		if result.ExitCode != 0 {
			return fmt.Errorf("pre-fetch %s failed (exit %d): %s",
				req.URL, result.ExitCode, strings.TrimSpace(result.Stderr))
		}
	}
	return nil
}

// buildAllowedPaths returns the filesystem paths the agent may read.
// URL and web modes have no filesystem access (returns nil).
func buildAllowedPaths(mode Mode, params ModeParams) []logos.AllowedPath {
	switch mode {
	case ModeProject:
		return []logos.AllowedPath{{Path: params.ProjectPath, ReadOnly: true}}
	case ModeRepo:
		return []logos.AllowedPath{{Path: params.RepoLocalPath, ReadOnly: true}}
	case ModeGeneral:
		if params.WorkingDir != "" {
			return []logos.AllowedPath{{Path: params.WorkingDir, ReadOnly: true}}
		}
	}
	return nil
}

// buildLogosCallbacks wires logos callbacks to NDJSON event emissions.
func buildLogosCallbacks(emit EventFunc) logos.Callbacks {
	return logos.Callbacks{
		OnDelta: func(text string) {
			emit(Event{Type: EventDelta, Text: text})
		},
		OnCommandResult: func(command, output string, exitCode int) {
			emit(Event{Type: EventCommandResult, Command: command, Output: output, ExitCode: exitCode})
		},
		OnRetry: func(reason string, step int) {
			emit(Event{Type: EventRetry, Reason: reason, Step: step})
		},
	}
}

// resolveParams converts the request mode + params into ModeParams for prompt building.
// The emit callback is used to send status events (e.g. "Cloning repo...").
func resolveParams(ctx context.Context, req Request, cfg *config.Config, emit EventFunc) (ModeParams, error) {
	params := ModeParams{
		WorkingDir: req.WorkingDir,
		Question:   req.Question,
		RawURL:     req.URL,
	}

	switch req.Mode {
	case ModeProject:
		return resolveProjectParams(req, params)
	case ModeRepo:
		return resolveRepoParams(ctx, req, cfg, params, emit)
	case ModeURL:
		if req.URL == "" {
			return params, fmt.Errorf("--url required")
		}
	case ModeWeb:
		// No resolution needed
	case ModeGeneral:
		if params.WorkingDir == "" {
			return params, fmt.Errorf("working_dir required for general mode")
		}
	default:
		return params, fmt.Errorf("unknown mode: %s", req.Mode)
	}

	return params, nil
}

func resolveProjectParams(req Request, params ModeParams) (ModeParams, error) {
	if req.Project == "" {
		return params, fmt.Errorf("--project alias required")
	}
	projectPath, err := project.GetProjectPath(req.Project)
	if err != nil {
		return params, err
	}
	if _, err := os.Stat(projectPath); err != nil {
		return params, fmt.Errorf("project path %q does not exist: %w", projectPath, err)
	}
	params.ProjectPath = projectPath
	params.WorkingDir = projectPath
	return params, nil
}

func resolveRepoParams(
	ctx context.Context, req Request, cfg *config.Config, params ModeParams, emit EventFunc,
) (ModeParams, error) {
	if req.Repo == "" {
		return params, fmt.Errorf("--repo reference required")
	}
	cloneURL, localPath, err := ResolveRepoRef(req.Repo, cfg.AskReferencesPath())
	if err != nil {
		return params, err
	}
	emit(Event{Type: EventStatus, Message: "Updating " + req.Repo + "..."})
	if err := EnsureRepo(ctx, cloneURL, localPath); err != nil {
		return params, err
	}
	params.RepoLocalPath = localPath
	params.WorkingDir = localPath
	return params, nil
}
