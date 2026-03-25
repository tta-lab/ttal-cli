package ask

import (
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
)

// BuildProvider creates a fantasy.Provider and resolved model ID from a model string.
// Model format: "provider/model-id" or bare model ID (defaults to anthropic).
// Currently supports: "minimax/" prefix (→ anthropic-compat via MINIMAX_API_URL/MINIMAX_API_KEY)
// and bare model IDs (→ anthropic via ANTHROPIC_API_KEY).
func BuildProvider(model string) (fantasy.Provider, string, error) {
	switch {
	case strings.HasPrefix(model, "minimax/"):
		baseURL := os.Getenv("MINIMAX_API_URL")
		apiKey := os.Getenv("MINIMAX_API_KEY")
		if baseURL == "" || apiKey == "" {
			return nil, "", fmt.Errorf("minimax/ model requires MINIMAX_API_URL and MINIMAX_API_KEY env vars")
		}
		modelID := strings.TrimPrefix(model, "minimax/")
		p, err := anthropic.New(anthropic.WithBaseURL(baseURL), anthropic.WithAPIKey(apiKey))
		if err != nil {
			return nil, "", fmt.Errorf("minimax provider (%s): %w", modelID, err)
		}
		return p, modelID, nil
	default:
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, "", fmt.Errorf("ANTHROPIC_API_KEY is not set")
		}
		p, err := anthropic.New(anthropic.WithAPIKey(apiKey))
		if err != nil {
			return nil, "", fmt.Errorf("anthropic provider (%s): %w", model, err)
		}
		return p, model, nil
	}
}
