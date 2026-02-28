# TTAL Tiered Pricing

## Tiers

| Tier | Price | Agents | Teams | Workers |
|------|-------|--------|-------|---------|
| Free | $0 | 2 | 1 | unlimited |
| Pro | $100 lifetime | unlimited | 1 | unlimited |
| Team | $200 lifetime | unlimited | unlimited | unlimited |

## Implementation

### JWT License Validation
- Ed25519-signed JWTs (no external dependencies)
- Public key embedded in binary via `//go:embed`
- License file: `~/.config/ttal/license.jwt`
- No JWT = free tier (default)
- Payload: `{ "sub": "<email>", "tier": "pro"|"team" }` — no expiration (lifetime)

### Enforcement Points
1. **Agent count**: `cmd/agent.go` checks before `agent add`
2. **Team count**: `internal/config/config.go` checks during `Load()` and `LoadAll()`

### CLI Commands
- `ttal license` — show current tier and limits
- `ttal license activate <jwt>` — save license file
- `ttal license deactivate` — remove license, revert to free

### Key Management
- `cmd/ttal-keygen/` — standalone tool for generating keypairs and signing JWTs
- `internal/license/pubkey.pem` — embedded public key (base64-encoded Ed25519)

### Design Decisions
- Workers are ephemeral (spawn-do-PR-done), no limit needed
- The value gate is the orchestration layer (manager agents)
- Local-first: JWT validated with embedded public key, no server ping
- No exp claim: lifetime licenses simplify validation
