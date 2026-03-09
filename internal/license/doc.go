// Package license implements tiered licensing for TTAL using Ed25519-signed JWTs.
//
// It defines three tiers (free, pro, team) with per-tier limits on agents and teams.
// Licenses are stored as JWT files at ~/.config/ttal/license and validated against
// an embedded Ed25519 public key. The free tier requires no license file.
//
// Plane: shared
package license
