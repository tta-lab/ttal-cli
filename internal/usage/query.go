package usage

import (
	"context"
	"time"

	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/ent/toolusage"
)

// Count represents a grouped usage count for a subcommand+target pair.
type Count struct {
	Subcommand string `json:"subcommand"`
	Target     string `json:"target"`
	Count      int    `json:"count"`
}

// Summary returns usage counts for an agent since a given time, grouped by subcommand and target.
func Summary(agent string, since time.Time) ([]Count, error) {
	client, err := open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var results []Count
	err = client.ToolUsage.Query().
		Where(
			toolusage.AgentEQ(agent),
			toolusage.CreatedAtGTE(since),
		).
		GroupBy(toolusage.FieldSubcommand, toolusage.FieldTarget).
		Aggregate(ent.Count()).
		Scan(ctx, &results)
	return results, err
}
