package addressee

import (
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
)

// AddresseeKind classifies what kind of entity a name resolves to.
type AddresseeKind int

const (
	KindAgent  AddresseeKind = iota // tmux session delivery
	KindWorker                      // job_id:agent_name → worker tmux or manager window
	KindHuman                       // frontend delivery (Telegram chat_id / Matrix invite)
)

// Addressee is a resolved send target.
type Addressee struct {
	Kind        AddresseeKind
	Name        string            // agent name, human alias, or worker:agent name
	Agent       *config.AgentInfo // populated when Kind == KindAgent
	Human       *humanfs.Human    // populated when Kind == KindHuman
	WorkerJobID string            // populated when Kind == KindWorker
}
