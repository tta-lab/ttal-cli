package frontend

import (
	"html"

	"github.com/tta-lab/ttal-cli/internal/config"
	ttalruntime "github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/sendfmt"
)

func formatInboundForAgent(cfg *config.Config, channel, agentName, senderName, text string) string {
	return sendfmt.Format(sendfmt.Envelope{
		Channel:      channel,
		SenderName:   html.EscapeString(senderName),
		Body:         text,
		ReplyAlias:   replyHumanAlias(cfg),
		ReplyRuntime: replyRuntimeForAgent(cfg, agentName),
	})
}

func replyRuntimeForAgent(cfg *config.Config, agentName string) ttalruntime.Runtime {
	if cfg == nil || agentName == "" {
		return ""
	}
	return cfg.RuntimeForAgent(agentName)
}
