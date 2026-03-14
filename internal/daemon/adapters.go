package daemon

import (
	"context"
	"log"
	"sync"

	"github.com/tta-lab/ttal-cli/internal/runtime"
	cc "github.com/tta-lab/ttal-cli/internal/runtime/claudecode"
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

// createAdapterFromTeam builds the appropriate adapter for an agent's runtime.
func createAdapterFromTeam(
	agentName string, rt runtime.Runtime, agentPath string,
	model string, env []string,
) runtime.Adapter {
	cfg := runtime.AdapterConfig{
		AgentName: agentName,
		WorkDir:   agentPath,
		Model:     model,
		Env:       env,
	}

	return cc.New(cfg)
}
