package frontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// matrixTestServer starts an httptest server that handles Matrix send-message requests.
// It captures request bodies and returns a success response.
func matrixTestServer(t *testing.T, bodies *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/send/") {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				raw, _ := json.Marshal(body)
				*bodies = append(*bodies, string(raw))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"event_id":"$testEventID"}`))
			return
		}
		// Return empty success for any other endpoints (login, versions, etc.)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// buildTestFrontend creates a MatrixFrontend with a pre-built mautrix.Client pointing at srv.
func buildTestFrontend(t *testing.T, srv *httptest.Server, agentName, roomID string) *MatrixFrontend {
	t.Helper()
	userID := id.NewUserID(agentName, "test")
	client, err := mautrix.NewClient(srv.URL, userID, "test-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return &MatrixFrontend{
		cfg: MatrixConfig{
			TeamName:   "testteam",
			UserNameFn: func() string { return "neil" },
		},
		clients:     map[string]*mautrix.Client{agentName: client},
		roomIDs:     map[string]id.RoomID{agentName: id.RoomID(roomID)},
		lastEventID: make(map[string]id.EventID),
	}
}

// TestNewMatrix_MissingConfig verifies error when team has no [matrix] config block.
func TestNewMatrix_MissingConfig(t *testing.T) {
	mcfg := &config.DaemonConfig{
		Teams: map[string]*config.ResolvedTeam{
			"myteam": {Name: "myteam", Frontend: "matrix", Matrix: nil},
		},
	}
	_, err := NewMatrix(MatrixConfig{
		TeamName: "myteam",
		MCfg:     mcfg,
	})
	if err == nil {
		t.Fatal("expected error for missing [matrix] config, got nil")
	}
}

// TestNewMatrix_SkipsAgentWithoutToken verifies that agents with unset token env vars are skipped.
func TestNewMatrix_SkipsAgentWithoutToken(t *testing.T) {
	mcfg := &config.DaemonConfig{
		Teams: map[string]*config.ResolvedTeam{
			"myteam": {
				Name:     "myteam",
				Frontend: "matrix",
				Matrix: &config.MatrixTeamConfig{
					Homeserver: "https://matrix.example.com",
					Agents: map[string]config.MatrixAgentConfig{
						"yuki": {AccessTokenEnv: "TTAL_TEST_UNSET_TOKEN_12345", RoomID: "!room:example.com"},
					},
				},
			},
		},
	}
	fe, err := NewMatrix(MatrixConfig{
		TeamName:   "myteam",
		MCfg:       mcfg,
		UserNameFn: func() string { return "neil" },
	})
	if err != nil {
		t.Fatalf("NewMatrix should succeed with missing token, got: %v", err)
	}
	if len(fe.clients) != 0 {
		t.Errorf("expected 0 clients (agent skipped), got %d", len(fe.clients))
	}
}

// TestMatrixFrontend_SendText verifies SendText sends the correct request to Matrix.
func TestMatrixFrontend_SendText(t *testing.T) {
	var bodies []string
	srv := matrixTestServer(t, &bodies)
	defer srv.Close()

	fe := buildTestFrontend(t, srv, "yuki", "!testroom:test")

	if err := fe.SendText(context.Background(), "yuki", "hello world"); err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 request body, got %d", len(bodies))
	}
	if !strings.Contains(bodies[0], `"hello world"`) {
		t.Errorf("body %q does not contain message text", bodies[0])
	}
	if !strings.Contains(bodies[0], `"m.text"`) {
		t.Errorf("body %q does not contain msgtype m.text", bodies[0])
	}
}

// TestMatrixFrontend_SendText_UnknownAgent verifies error for unregistered agents.
func TestMatrixFrontend_SendText_UnknownAgent(t *testing.T) {
	fe := &MatrixFrontend{
		clients:     map[string]*mautrix.Client{},
		roomIDs:     map[string]id.RoomID{},
		lastEventID: make(map[string]id.EventID),
	}
	if err := fe.SendText(context.Background(), "unknown", "hi"); err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
}

// TestMatrixFrontend_SendNotification verifies notification messages are sent to the notify room.
func TestMatrixFrontend_SendNotification(t *testing.T) {
	var bodies []string
	srv := matrixTestServer(t, &bodies)
	defer srv.Close()

	userID := id.NewUserID("notify", "test")
	nc, err := mautrix.NewClient(srv.URL, userID, "notify-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	fe := &MatrixFrontend{
		cfg:          MatrixConfig{TeamName: "testteam"},
		clients:      map[string]*mautrix.Client{},
		roomIDs:      map[string]id.RoomID{},
		notifyClient: nc,
		notifyRoom:   id.RoomID("!notifyroom:test"),
		lastEventID:  make(map[string]id.EventID),
	}

	if err := fe.SendNotification(context.Background(), "task done"); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 request body, got %d", len(bodies))
	}
	if !strings.Contains(bodies[0], `"task done"`) {
		t.Errorf("body %q does not contain notification text", bodies[0])
	}
}

// TestMatrixFrontend_SendNotification_NilClient verifies nil notify client returns nil (not error).
func TestMatrixFrontend_SendNotification_NilClient(t *testing.T) {
	fe := &MatrixFrontend{
		cfg:          MatrixConfig{TeamName: "testteam"},
		notifyClient: nil,
		lastEventID:  make(map[string]id.EventID),
	}
	if err := fe.SendNotification(context.Background(), "ping"); err != nil {
		t.Errorf("expected nil when notify client not configured, got: %v", err)
	}
}

// TestMatrixFrontend_StubMethods verifies all stub methods return expected values without blocking.
func TestMatrixFrontend_StubMethods(t *testing.T) {
	fe := &MatrixFrontend{lastEventID: make(map[string]id.EventID)}
	ctx := context.Background()

	if err := fe.SendVoice(ctx, "agent", []byte("data")); err != nil {
		t.Errorf("SendVoice: %v", err)
	}
	if err := fe.SetReaction(ctx, "agent", "👍"); err != nil {
		t.Errorf("SetReaction: %v", err)
	}
	if err := fe.RegisterCommands([]Command{{Name: "help"}}); err != nil {
		t.Errorf("RegisterCommands: %v", err)
	}
	answer, skipped, err := fe.AskHuman(ctx, "agent", "question?", nil)
	if err != nil {
		t.Errorf("AskHuman: %v", err)
	}
	if !skipped {
		t.Errorf("AskHuman should return skipped=true, got false")
	}
	if answer != "" {
		t.Errorf("AskHuman should return empty answer, got %q", answer)
	}
}

// TestSplitMatrixMessage verifies message splitting behavior.
func TestSplitMatrixMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
	}{
		{
			name:    "empty string",
			input:   "",
			wantLen: 1,
		},
		{
			name:    "short string",
			input:   "hello",
			wantLen: 1,
		},
		{
			name:    "exactly at limit",
			input:   strings.Repeat("a", maxMatrixMessageBytes),
			wantLen: 1,
		},
		{
			name:    "over limit splits into two",
			input:   strings.Repeat("a", maxMatrixMessageBytes/2) + "\n\n" + strings.Repeat("b", maxMatrixMessageBytes/2),
			wantLen: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitMatrixMessage(tt.input)
			if len(parts) != tt.wantLen {
				t.Errorf("got %d parts, want %d", len(parts), tt.wantLen)
			}
			for _, p := range parts {
				if len(p) > maxMatrixMessageBytes {
					t.Errorf("part exceeds limit: len=%d", len(p))
				}
			}
		})
	}
}

// TestExtractDomain verifies domain extraction from homeserver URLs.
func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://matrix.example.com", "matrix.example.com"},
		{"https://host:8448", "host:8448"},
		{"not-a-url", "not-a-url"},
		{"http://localhost:8008", "localhost:8008"},
	}
	for _, tt := range tests {
		got := extractDomain(tt.url)
		if got != tt.want {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

// TestMatrixFrontend_ClearTracking verifies ClearTracking removes the last event ID.
func TestMatrixFrontend_ClearTracking(t *testing.T) {
	fe := &MatrixFrontend{
		lastEventID: map[string]id.EventID{
			"yuki": id.EventID("$abc:test"),
		},
	}
	if err := fe.ClearTracking(context.Background(), "yuki"); err != nil {
		t.Fatalf("ClearTracking: %v", err)
	}
	if _, ok := fe.lastEventID["yuki"]; ok {
		t.Error("expected lastEventID for 'yuki' to be cleared")
	}
}
