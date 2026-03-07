package opencode

import (
	"context"
	"os/exec"

	acp "github.com/coder/acp-go-sdk"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

type ACPAdapter struct {
	cfg       runtime.AdapterConfig
	cmd       *exec.Cmd
	conn      *acp.ClientSideConnection
	client    *acpClient
	events    chan runtime.Event
	sessionID string
}

type acpClient struct {
	events chan runtime.Event
	agent  string
}

func (c *acpClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Selected: &acp.RequestPermissionOutcomeSelected{
				OptionId: params.Options[0].OptionId,
			},
		},
	}, nil
}

func (c *acpClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update
	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			c.events <- runtime.Event{
				Type:  runtime.EventText,
				Agent: c.agent,
				Text:  u.AgentMessageChunk.Content.Text.Text,
			}
		}
	case u.ToolCall != nil:
		toolName := mapACPToolKind(string(u.ToolCall.Kind))
		c.events <- runtime.Event{
			Type:     runtime.EventTool,
			Agent:    c.agent,
			ToolName: toolName,
		}
	case u.AgentThoughtChunk != nil:
	case u.ToolCallUpdate != nil:
		if u.ToolCallUpdate.Status != nil && *u.ToolCallUpdate.Status == acp.ToolCallStatusCompleted {
			c.events <- runtime.Event{
				Type:  runtime.EventIdle,
				Agent: c.agent,
			}
		}
	}
	return nil
}

func (c *acpClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, nil
}

func (c *acpClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, nil
}

func (c *acpClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{TerminalId: "terminal-1"}, nil
}

func (c *acpClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, nil
}

func (c *acpClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *acpClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, nil
}

func (c *acpClient) KillTerminalCommand(ctx context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, nil
}

func NewACPAdapter(cfg runtime.AdapterConfig) *ACPAdapter {
	return &ACPAdapter{
		cfg:    cfg,
		events: make(chan runtime.Event, 64),
	}
}

func (a *ACPAdapter) Runtime() runtime.Runtime { return runtime.OpenCode }

func (a *ACPAdapter) Start(ctx context.Context) error {
	a.cmd = exec.CommandContext(ctx, "opencode", "acp")
	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := a.cmd.Start(); err != nil {
		return err
	}

	a.client = &acpClient{events: a.events, agent: a.cfg.AgentName}
	a.conn = acp.NewClientSideConnection(a.client, stdin, stdout)

	_, err = a.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs:       acp.FileSystemCapability{ReadTextFile: true, WriteTextFile: true},
			Terminal: true,
		},
	})
	if err != nil {
		return err
	}

	a.sessionID, err = a.CreateSession(ctx)
	return err
}

func (a *ACPAdapter) Stop(ctx context.Context) error {
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
	}
	close(a.events)
	return nil
}

func (a *ACPAdapter) SendMessage(ctx context.Context, text string) error {
	_, err := a.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acp.SessionId(a.sessionID),
		Prompt:    []acp.ContentBlock{acp.TextBlock(text)},
	})
	return err
}

func (a *ACPAdapter) Events() <-chan runtime.Event { return a.events }

func (a *ACPAdapter) CreateSession(ctx context.Context) (string, error) {
	resp, err := a.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd: a.cfg.WorkDir,
	})
	if err != nil {
		return "", err
	}
	a.sessionID = string(resp.SessionId)
	return a.sessionID, nil
}

func (a *ACPAdapter) ResumeSession(ctx context.Context, sessionID string) (string, error) {
	_, err := a.conn.LoadSession(ctx, acp.LoadSessionRequest{
		SessionId: acp.SessionId(sessionID),
		Cwd:       a.cfg.WorkDir,
	})
	if err != nil {
		return "", err
	}
	a.sessionID = sessionID
	return sessionID, nil
}

func (a *ACPAdapter) IsHealthy(ctx context.Context) bool {
	return a.cmd != nil && a.cmd.Process != nil
}

func mapACPToolKind(k string) string {
	switch k {
	case "read":
		return "Read"
	case "edit":
		return "Edit"
	case "delete":
		return "Delete"
	case "move":
		return "Move"
	case "search":
		return "Search"
	case "execute":
		return "Bash"
	case "fetch":
		return "WebFetch"
	default:
		return k
	}
}
