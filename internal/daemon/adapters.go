package daemon

import (
	"context"
	"log"
	"sync"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/runtime/claudecode"
	cx "codeberg.org/clawteam/ttal-cli/internal/runtime/codex"
	oc "codeberg.org/clawteam/ttal-cli/internal/runtime/opencode"
	"codeberg.org/clawteam/ttal-cli/internal/telegram"
)

// adapterRegistry holds adapters for all agents, keyed by agent name.
type adapterRegistry struct {
	adapters map[string]runtime.Adapter
	mu       sync.RWMutex
}

func newAdapterRegistry() *adapterRegistry {
	return &adapterRegistry{adapters: make(map[string]runtime.Adapter)}
}

func (r *adapterRegistry) get(agentName string) (runtime.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[agentName]
	return a, ok
}

func (r *adapterRegistry) set(agentName string, a runtime.Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[agentName] = a
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

// createAdapter builds the appropriate adapter for an agent's runtime.
func createAdapter(
	agentName string, rt runtime.Runtime, agentPath string,
	port int, model string, yolo bool, env []string,
) runtime.Adapter {
	cfg := runtime.AdapterConfig{
		AgentName: agentName,
		WorkDir:   agentPath,
		Port:      port,
		Model:     model,
		Yolo:      yolo,
		Env:       env,
	}

	switch rt {
	case runtime.OpenCode:
		return oc.New(cfg)
	case runtime.Codex:
		return cx.New(cfg)
	default:
		return claudecode.New(cfg)
	}
}

// bridgeEvents reads events from an adapter and routes them to Telegram.
func bridgeEvents(agentName string, adapter runtime.Adapter, cfg *config.Config) {
	agentCfg, ok := cfg.Agents[agentName]
	if !ok || agentCfg.BotToken == "" {
		return
	}
	chatID := cfg.AgentChatID(agentName)

	go func() {
		for event := range adapter.Events() {
			switch event.Type {
			case runtime.EventText:
				if err := telegram.SendMessage(agentCfg.BotToken, chatID, event.Text); err != nil {
					log.Printf("[daemon] telegram send error for %s: %v", agentName, err)
				}
			case runtime.EventError:
				log.Printf("[daemon] runtime error for %s: %s", agentName, event.Text)
			}
		}
	}()
}
