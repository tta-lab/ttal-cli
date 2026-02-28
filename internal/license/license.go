// Package license implements tiered licensing for TTAL using Ed25519-signed JWTs.
//
// Tiers:
//   - Free: 1 team, 2 agents, unlimited workers (no JWT required)
//   - Pro:  1 team, unlimited agents ($100 lifetime)
//   - Team: unlimited teams, unlimited agents ($200 lifetime)
//
// License file: ~/.config/ttal/license.jwt
// JWT payload: { "sub": "<email>", "tier": "pro"|"team" }
// No expiration claim — all licenses are lifetime.
package license

import (
	"crypto/ed25519"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Tier represents a licensing tier.
type Tier string

const (
	Free Tier = "free"
	Pro  Tier = "pro"
	Team Tier = "team"
)

// Limits per tier.
const (
	FreeMaxAgents = 2
	FreeMaxTeams  = 1
	ProMaxTeams   = 1
)

// Claims is the JWT payload.
type Claims struct {
	Sub  string `json:"sub"`  // email or identifier
	Tier Tier   `json:"tier"` // "pro" or "team"
}

// License holds the validated license state.
type License struct {
	Tier   Tier
	Claims *Claims // nil for free tier
}

//go:embed pubkey.b64
var embeddedPubKey []byte

// publicKeyOverride allows tests to inject a different key. Must be nil in production.
var publicKeyOverride ed25519.PublicKey

// publicKey returns the embedded Ed25519 public key.
func publicKey() (ed25519.PublicKey, error) {
	if publicKeyOverride != nil {
		return publicKeyOverride, nil
	}
	// pubkey.b64 contains the raw 32-byte public key, base64-encoded
	trimmed := strings.TrimSpace(string(embeddedPubKey))
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode embedded public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// LicensePath returns the path to the license file.
func LicensePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttal", "license.jwt"), nil
}

// Load reads and validates the license file.
// Returns free tier if no license file exists.
func Load() (*License, error) {
	path, err := LicensePath()
	if err != nil {
		return nil, fmt.Errorf("license path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &License{Tier: Free}, nil
		}
		return nil, fmt.Errorf("read license: %w", err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return &License{Tier: Free}, nil
	}

	claims, err := Validate(token)
	if err != nil {
		return nil, fmt.Errorf("invalid license: %w", err)
	}

	return &License{Tier: claims.Tier, Claims: claims}, nil
}

// Validate parses and verifies a JWT token string.
func Validate(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}

	// Decode payload (part 1)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	// Verify signature (part 2)
	pubKey, err := publicKey()
	if err != nil {
		return nil, err
	}

	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	if !ed25519.Verify(pubKey, []byte(signingInput), sig) {
		return nil, fmt.Errorf("signature verification failed")
	}

	// Parse claims
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	if claims.Tier != Pro && claims.Tier != Team {
		return nil, fmt.Errorf("invalid tier in license: %q", claims.Tier)
	}

	return &claims, nil
}

// Activate writes a JWT token to the license file.
func Activate(token string) error {
	// Validate first
	if _, err := Validate(token); err != nil {
		return err
	}

	path, err := LicensePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

// MaxAgents returns the agent limit for a tier. -1 means unlimited.
func (l *License) MaxAgents() int {
	switch l.Tier {
	case Pro, Team:
		return -1
	default:
		return FreeMaxAgents
	}
}

// MaxTeams returns the team limit for a tier. -1 means unlimited.
func (l *License) MaxTeams() int {
	switch l.Tier {
	case Team:
		return -1
	case Pro:
		return ProMaxTeams
	default:
		return FreeMaxTeams
	}
}

// CheckAgentLimit returns an error if adding an agent would exceed the tier limit.
func (l *License) CheckAgentLimit(currentCount int) error {
	max := l.MaxAgents()
	if max == -1 {
		return nil
	}
	if currentCount >= max {
		return fmt.Errorf(
			"free tier allows %d agents (you have %d) — upgrade to Pro for unlimited agents: https://ttal.guion.io/pricing",
			max, currentCount,
		)
	}
	return nil
}

// CheckTeamLimit returns an error if the number of teams exceeds the tier limit.
func (l *License) CheckTeamLimit(teamCount int) error {
	max := l.MaxTeams()
	if max == -1 {
		return nil
	}
	if teamCount > max {
		return fmt.Errorf(
			"%s tier allows %d team (you have %d) — upgrade to Team for unlimited teams: https://ttal.guion.io/pricing",
			l.Tier, max, teamCount,
		)
	}
	return nil
}
