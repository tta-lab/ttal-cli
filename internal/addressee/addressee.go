// Package addressee provides the unified Addressee type identifying any send
// target (agent, worker, human) in ttal. Used by both the daemon (routing) and
// frontend (transport) to avoid divergent representations of the same identity.
//
// Plane: shared
package addressee

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
	Name        string // agent name, human alias, or worker:agent name
	Agent       any    // *config.AgentInfo when Kind == KindAgent
	Human       any    // *humanfs.Human when Kind == KindHuman
	WorkerJobID string // populated when Kind == KindWorker
}
