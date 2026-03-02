package daemon

import (
	"context"
	"log"
	"sync"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	cx "github.com/tta-lab/ttal-cli/internal/runtime/codex"
	oclw "github.com/tta-lab/ttal-cli/internal/runtime/openclaw"
	oc "github.com/tta-lab/ttal-cli/internal/runtime/opencode"
)

// adapterRegistry holds adapters for all agents, keyed by "teamName/agentName"
// to avoid collisions when agents in different teams share the same name.
type adapterRegistry struct {
	adapters map[string]runtime.Adapter
	mu       sync.RWMutex
}

// registryKey builds the composite key for an adapter registry entry.
func registryKey(teamName, agentName string) string {
	return teamName + "/" + agentName
}

func newAdapterRegistry() *adapterRegistry {
	return &adapterRegistry{adapters: make(map[string]runtime.Adapter)}
}

func (r *adapterRegistry) get(teamName, agentName string) (runtime.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[registryKey(teamName, agentName)]
	return a, ok
}

func (r *adapterRegistry) set(teamName, agentName string, a runtime.Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[registryKey(teamName, agentName)] = a
}

// stopAll gracefully stops all adapters.
func (r *adapterRegistry) stopAll(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, a := range r.adapters {
		if err := a.Stop(ctx); err != nil {
			log.Printf("[daemon] error stopping adapter for %s: %v", name, err)
		}
	}
}

// createAdapterFromTeam builds the appropriate adapter for an agent's runtime,
// using team-resolved config values for GatewayURL and HooksToken.
func createAdapterFromTeam(
	agentName string, rt runtime.Runtime, agentPath string,
	port int, model string, yolo bool, env []string,
	team *config.ResolvedTeam,
) runtime.Adapter {
	cfg := runtime.AdapterConfig{
		AgentName: agentName,
		WorkDir:   agentPath,
		Port:      port,
		Model:     model,
		Yolo:      yolo,
		Env:       env,
	}
	if team != nil {
		cfg.GatewayURL = team.GatewayURL
		if cfg.GatewayURL == "" {
			cfg.GatewayURL = config.DefaultGatewayURL
		}
		cfg.HooksToken = team.HooksToken
	}

	switch rt {
	case runtime.Codex:
		return cx.New(cfg)
	case runtime.OpenClaw:
		return oclw.New(cfg)
	default:
		return oc.New(cfg)
	}
}
