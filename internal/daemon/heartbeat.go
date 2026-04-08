package daemon

import (
	"log"
	"path/filepath"
	"time"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
)

// startHeartbeatScheduler starts a per-agent ticker for agents with heartbeat_interval configured.
// Interval is read from roles.toml via the agent's role field in AGENTS.md frontmatter.
// On each tick, delivers heartbeat_prompt (from roles.toml) to the agent via deliverToAgent.
// Both heartbeat_interval and heartbeat_prompt must be non-empty — skips silently if either is missing.
// Timer resets on daemon restart (no state persistence — acceptable tradeoff per spec).
func startHeartbeatScheduler(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend, done <-chan struct{},
) {
	started, skipped := 0, 0

	roles := mcfg.Global.Roles()
	for _, ta := range mcfg.AllAgents() {
		if ta.TeamPath == "" {
			continue
		}
		info, err := agentfs.GetFromPath(filepath.Join(ta.TeamPath, ta.AgentName))
		if err != nil || info.Role == "" {
			continue
		}

		intervalStr := roles.HeartbeatIntervalForRole(info.Role)
		if intervalStr == "" {
			continue
		}

		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			log.Printf("[heartbeat] invalid heartbeat_interval %q for role %s (%s): %v — skipping",
				intervalStr, info.Role, ta.AgentName, err)
			skipped++
			continue
		}

		prompt := mcfg.Global.HeartbeatPrompt(ta.AgentName)
		if prompt == "" {
			log.Printf("[heartbeat] no heartbeat_prompt for %s in roles.toml — skipping", ta.AgentName)
			skipped++
			continue
		}

		log.Printf("[heartbeat] scheduling %s (role: %s) every %s", ta.AgentName, info.Role, interval)
		started++

		teamName := config.DefaultTeamName
		agentName := ta.AgentName
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					log.Printf("[heartbeat] firing for %s", agentName)
					if err := deliverToAgent(registry, mcfg, frontends, agentName, prompt); err != nil {
						log.Printf("[heartbeat] deliver failed for %s/%s: %v", teamName, agentName, err)
					}
				}
			}
		}()
	}

	log.Printf("[heartbeat] started %d scheduler(s), skipped %d due to config errors", started, skipped)
}
