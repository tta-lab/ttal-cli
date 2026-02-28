package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// testKeypair generates a fresh Ed25519 keypair for testing.
func testKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	// Use the embedded public key's corresponding private key from generation.
	// For tests, we create a new keypair and test the validation logic directly.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func signJWT(t *testing.T, priv ed25519.PrivateKey, claims Claims) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	sig := ed25519.Sign(priv, []byte(signingInput))
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestValidate_WithEmbeddedKey(t *testing.T) {
	// Test that the embedded public key can be loaded.
	pub, err := publicKey()
	if err != nil {
		t.Fatalf("publicKey() error: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("unexpected key size: %d", len(pub))
	}
}

func TestValidate_MalformedJWT(t *testing.T) {
	_, err := Validate("not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for malformed JWT")
	}
}

func TestValidate_BadSignature(t *testing.T) {
	// Create a JWT signed with a different key.
	_, priv := testKeypair(t)
	token := signJWT(t, priv, Claims{Sub: "test@example.com", Tier: Pro})

	// This should fail because the embedded key is different.
	_, err := Validate(token)
	if err == nil {
		t.Fatal("expected error for wrong signing key")
	}
	if !strings.Contains(err.Error(), "signature verification failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLicense_Limits(t *testing.T) {
	tests := []struct {
		tier      Tier
		maxAgents int
		maxTeams  int
	}{
		{Free, FreeMaxAgents, FreeMaxTeams},
		{Pro, -1, ProMaxTeams},
		{Team, -1, -1},
	}

	for _, tt := range tests {
		lic := &License{Tier: tt.tier}
		if got := lic.MaxAgents(); got != tt.maxAgents {
			t.Errorf("tier %s: MaxAgents() = %d, want %d", tt.tier, got, tt.maxAgents)
		}
		if got := lic.MaxTeams(); got != tt.maxTeams {
			t.Errorf("tier %s: MaxTeams() = %d, want %d", tt.tier, got, tt.maxTeams)
		}
	}
}

func TestCheckAgentLimit(t *testing.T) {
	free := &License{Tier: Free}

	// Under limit — should pass.
	if err := free.CheckAgentLimit(0); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
	if err := free.CheckAgentLimit(1); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}

	// At limit — should fail.
	if err := free.CheckAgentLimit(2); err == nil {
		t.Error("expected error at limit")
	}

	// Pro has no limit.
	pro := &License{Tier: Pro}
	if err := pro.CheckAgentLimit(100); err != nil {
		t.Errorf("pro should have no limit, got: %v", err)
	}
}

func TestCheckTeamLimit(t *testing.T) {
	free := &License{Tier: Free}

	if err := free.CheckTeamLimit(1); err != nil {
		t.Errorf("expected nil for 1 team on free: %v", err)
	}
	if err := free.CheckTeamLimit(2); err == nil {
		t.Error("expected error for 2 teams on free")
	}

	team := &License{Tier: Team}
	if err := team.CheckTeamLimit(50); err != nil {
		t.Errorf("team tier should have no limit, got: %v", err)
	}
}

// withTestKey sets up a test keypair and returns the private key for signing.
// It overrides publicKeyOverride and restores it on test cleanup.
func withTestKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	pub, priv := testKeypair(t)
	publicKeyOverride = pub
	t.Cleanup(func() { publicKeyOverride = nil })
	return priv
}

func TestValidate_ValidToken(t *testing.T) {
	priv := withTestKey(t)
	token := signJWT(t, priv, Claims{Sub: "user@example.com", Tier: Pro})

	claims, err := Validate(token)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if claims.Sub != "user@example.com" {
		t.Errorf("sub = %q, want %q", claims.Sub, "user@example.com")
	}
	if claims.Tier != Pro {
		t.Errorf("tier = %q, want %q", claims.Tier, Pro)
	}
}

func TestValidate_InvalidTier(t *testing.T) {
	priv := withTestKey(t)
	token := signJWT(t, priv, Claims{Sub: "user@example.com", Tier: "enterprise"})

	_, err := Validate(token)
	if err == nil {
		t.Fatal("expected error for invalid tier")
	}
	if !strings.Contains(err.Error(), "invalid tier") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActivate_RoundTrip(t *testing.T) {
	priv := withTestKey(t)
	token := signJWT(t, priv, Claims{Sub: "buyer@example.com", Tier: Team})

	// Use a temp HOME so Activate writes to a temp directory.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	if err := Activate(token); err != nil {
		t.Fatalf("Activate() error: %v", err)
	}

	// Load should read back the license.
	lic, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if lic.Tier != Team {
		t.Errorf("tier = %q, want %q", lic.Tier, Team)
	}
	if lic.Claims == nil {
		t.Fatal("expected non-nil claims")
	}
	if lic.Claims.Sub != "buyer@example.com" {
		t.Errorf("sub = %q, want %q", lic.Claims.Sub, "buyer@example.com")
	}
}

func TestLoad_NoFile(t *testing.T) {
	// Override home to a temp dir so no license file exists.
	t.Setenv("HOME", t.TempDir())

	lic, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if lic.Tier != Free {
		t.Errorf("expected free tier, got: %s", lic.Tier)
	}
	if lic.Claims != nil {
		t.Error("expected nil claims for free tier")
	}
}
