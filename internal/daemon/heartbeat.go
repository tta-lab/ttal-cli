package daemon

import (
	"log"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// startHeartbeatScheduler starts a per-agent ticker for agents with heartbeat_interval configured.
// On each tick, delivers heartbeat_prompt (from roles.toml) to the agent via deliverToAgent.
// Both heartbeat_interval and heartbeat_prompt must be non-empty — skips silently if either is missing.
// Timer resets on daemon restart (no state persistence — acceptable tradeoff per spec).
func startHeartbeatScheduler(mcfg *config.DaemonConfig, registry *adapterRegistry, done <-chan struct{}) {
	started, skipped := 0, 0

	for _, ta := range mcfg.AllAgents() {
		intervalStr := ta.Config.HeartbeatInterval
		if intervalStr == "" {
			continue
		}

		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			log.Printf("[heartbeat] invalid heartbeat_interval %q for %s: %v — skipping", intervalStr, ta.AgentName, err)
			skipped++
			continue
		}

		prompt := mcfg.Global.HeartbeatPrompt(ta.AgentName)
		if prompt == "" {
			log.Printf("[heartbeat] no heartbeat_prompt for %s in roles.toml — skipping", ta.AgentName)
			skipped++
			continue
		}

		log.Printf("[heartbeat] scheduling %s every %s", ta.AgentName, interval)
		started++

		teamName := ta.TeamName
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
					if err := deliverToAgent(registry, mcfg, teamName, agentName, prompt); err != nil {
						log.Printf("[heartbeat] deliver failed for %s/%s: %v", teamName, agentName, err)
					}
				}
			}
		}()
	}

	log.Printf("[heartbeat] started %d scheduler(s), skipped %d due to config errors", started, skipped)
}
