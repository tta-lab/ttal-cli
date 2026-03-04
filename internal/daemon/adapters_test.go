package daemon

import (
	"context"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// stubAdapter is a minimal Adapter implementation for registry tests.
type stubAdapter struct {
	name string
}

func (s *stubAdapter) Start(context.Context) error                           { return nil }
func (s *stubAdapter) Stop(context.Context) error                            { return nil }
func (s *stubAdapter) SendMessage(context.Context, string) error             { return nil }
func (s *stubAdapter) Events() <-chan runtime.Event                          { return nil }
func (s *stubAdapter) CreateSession(context.Context) (string, error)         { return "", nil }
func (s *stubAdapter) ResumeSession(context.Context, string) (string, error) { return "", nil }
func (s *stubAdapter) IsHealthy(context.Context) bool                        { return true }
func (s *stubAdapter) Runtime() runtime.Runtime                              { return runtime.ClaudeCode }

func TestRegistryKey(t *testing.T) {
	if got := registryKey("team1", "agent"); got != "team1/agent" {
		t.Errorf("registryKey = %q, want %q", got, "team1/agent")
	}
}

func TestAdapterRegistryCrossTeamIsolation(t *testing.T) {
	r := newAdapterRegistry()
	a1 := &stubAdapter{name: "team1/agent"}
	a2 := &stubAdapter{name: "team2/agent"}

	r.set("team1", "agent", a1)
	r.set("team2", "agent", a2)

	got1, ok1 := r.get("team1", "agent")
	if !ok1 {
		t.Fatal("expected team1/agent to exist")
	}
	if got1 != a1 {
		t.Error("team1/agent returned wrong adapter")
	}

	got2, ok2 := r.get("team2", "agent")
	if !ok2 {
		t.Fatal("expected team2/agent to exist")
	}
	if got2 != a2 {
		t.Error("team2/agent returned wrong adapter")
	}

	if got1 == got2 {
		t.Error("adapters for different teams must be distinct")
	}
}

func TestAdapterRegistryGetMiss(t *testing.T) {
	r := newAdapterRegistry()
	_, ok := r.get("noTeam", "noAgent")
	if ok {
		t.Error("expected get on empty registry to return false")
	}
}
