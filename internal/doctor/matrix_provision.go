package doctor

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // SHA1 required by Synapse shared-secret registration spec
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
)

// checkMatrix verifies Matrix configuration for all teams with frontend=matrix.
// With fix=true it also provisions missing Matrix users, rooms, and config entries.
func checkMatrix(fix bool) Section {
	section := Section{Name: "Matrix"}

	cfg, err := config.Load()
	if err != nil {
		section.add(LevelError, "config", fmt.Sprintf("config load: %v", err))
		return section
	}

	// Find teams with frontend=matrix
	var matrixTeams []string
	for name, team := range cfg.Teams {
		if team.Frontend == "matrix" {
			matrixTeams = append(matrixTeams, name)
		}
	}
	if len(matrixTeams) == 0 {
		section.add(LevelOK, "matrix", "No Matrix teams configured — skipping")
		return section
	}

	// Sort for deterministic output
	for i := 0; i < len(matrixTeams)-1; i++ {
		for j := i + 1; j < len(matrixTeams); j++ {
			if matrixTeams[i] > matrixTeams[j] {
				matrixTeams[i], matrixTeams[j] = matrixTeams[j], matrixTeams[i]
			}
		}
	}

	for _, teamName := range matrixTeams {
		checkMatrixTeam(&section, cfg, teamName, fix)
	}

	return section
}

// checkMatrixTeam runs all Matrix checks for a single team.
func checkMatrixTeam(section *Section, cfg *config.Config, teamName string, fix bool) {
	team := cfg.Teams[teamName]
	if team.Matrix == nil {
		section.add(LevelError, teamName, "frontend=matrix but no [teams."+teamName+".matrix] config")
		return
	}
	matrixCfg := team.Matrix
	if matrixCfg.Homeserver == "" {
		section.add(LevelError, teamName, "matrix.homeserver not set")
		return
	}
	section.add(LevelOK, teamName+".homeserver", matrixCfg.Homeserver)

	if matrixCfg.HumanUserID == "" {
		section.add(LevelWarn, teamName+".human_user_id",
			"human_user_id not set — provisioning cannot invite human to rooms")
	}

	checkMatrixConnectivity(section, teamName, matrixCfg.Homeserver)

	// Discover agents from team_path
	teamPath := team.TeamPath
	if teamPath == "" {
		teamPath = cfg.TeamPath()
	}
	agentNames, _ := agentfs.DiscoverAgents(teamPath)
	// Sort for deterministic output
	for i := 0; i < len(agentNames)-1; i++ {
		for j := i + 1; j < len(agentNames); j++ {
			if agentNames[i] > agentNames[j] {
				agentNames[i], agentNames[j] = agentNames[j], agentNames[i]
			}
		}
	}

	checkMatrixAgents(section, cfg, teamName, matrixCfg, agentNames, fix)
	checkMatrixNotify(section, cfg, teamName, matrixCfg, fix)
}

// checkMatrixAgents verifies or provisions Matrix users for each discovered agent.
func checkMatrixAgents(
	section *Section, cfg *config.Config, teamName string,
	matrixCfg *config.MatrixTeamConfig, agentNames []string, fix bool,
) {
	for _, agentName := range agentNames {
		agentCfg, ok := matrixCfg.Agents[agentName]
		if !ok {
			if fix {
				provisionMatrixAgent(section, cfg, teamName, matrixCfg, agentName)
			} else {
				section.add(LevelError, agentName,
					fmt.Sprintf("Agent %s: no Matrix config (run: ttal doctor --fix)", agentName))
			}
			continue
		}
		token := os.Getenv(agentCfg.AccessTokenEnv)
		if token == "" {
			section.add(LevelError, agentName,
				fmt.Sprintf("Agent %s: %s not set in env", agentName, agentCfg.AccessTokenEnv))
		} else {
			section.add(LevelOK, agentName,
				fmt.Sprintf("Agent %s: token set, room %s", agentName, agentCfg.RoomID))
		}
	}
}

// checkMatrixNotify verifies or provisions the Matrix notification room.
func checkMatrixNotify(
	section *Section, cfg *config.Config, teamName string,
	matrixCfg *config.MatrixTeamConfig, fix bool,
) {
	if matrixCfg.NotifyTokenEnv == "" || matrixCfg.NotifyRoom == "" {
		if fix {
			provisionMatrixNotify(section, cfg, teamName, matrixCfg)
		} else {
			section.add(LevelWarn, teamName+".notify", "Notification room not configured (run: ttal doctor --fix)")
		}
		return
	}
	token := os.Getenv(matrixCfg.NotifyTokenEnv)
	if token == "" {
		section.add(LevelError, teamName+".notify", matrixCfg.NotifyTokenEnv+" not set in env")
	} else {
		section.add(LevelOK, teamName+".notify", "Notification configured")
	}
}

// checkMatrixConnectivity verifies that the homeserver's /_matrix/client/versions endpoint is reachable.
func checkMatrixConnectivity(section *Section, teamName, homeserver string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqURL := homeserver + "/_matrix/client/versions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		section.add(LevelError, teamName+".connectivity", fmt.Sprintf("invalid URL: %v", err))
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		section.add(LevelError, teamName+".connectivity", fmt.Sprintf("cannot reach %s: %v", homeserver, err))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		section.add(LevelError, teamName+".connectivity", fmt.Sprintf("homeserver returned %d", resp.StatusCode))
		return
	}
	section.add(LevelOK, teamName+".connectivity", "Homeserver reachable")
}

// provisionMatrixAgent creates a Matrix user, sets profile, creates a room, and updates config.
// Idempotent: skips steps that are already done. If the user already exists on the homeserver,
// provisioning cannot retrieve the token — logs an actionable message for manual resolution.
func provisionMatrixAgent(
	section *Section, cfg *config.Config, teamName string,
	matrixCfg *config.MatrixTeamConfig, agentName string,
) {
	homeserver := matrixCfg.Homeserver

	// Step 1: Register user via Synapse-compatible admin API
	regSecret := os.Getenv("MATRIX_REGISTRATION_SECRET")
	if regSecret == "" {
		section.add(LevelWarn, agentName,
			fmt.Sprintf("Agent %s: MATRIX_REGISTRATION_SECRET not set — cannot provision", agentName))
		return
	}

	password, err := registerMatrixUser(homeserver, regSecret, agentName)
	if err != nil {
		// User may already exist — we can't retrieve its token programmatically.
		section.add(LevelWarn, agentName,
			fmt.Sprintf("Agent %s: registration failed (user likely already exists). "+
				"To provision manually: get an access token from the homeserver admin, "+
				"add %s_MATRIX_TOKEN=<token> to .env, and add [teams.%s.matrix.agents.%s] "+
				"to config.toml with access_token_env and room_id.",
				agentName, strings.ToUpper(agentName), teamName, agentName))
		return
	}

	// Step 2: Login to get access token
	accessToken, err := loginMatrixUser(homeserver, agentName, password)
	if err != nil {
		section.add(LevelError, agentName,
			fmt.Sprintf("Agent %s: login failed: %v", agentName, err))
		return
	}

	// Step 3: Set display name
	domain := extractDomainFromURL(homeserver)
	userID := id.NewUserID(agentName, domain)
	client, err := mautrix.NewClient(homeserver, userID, accessToken)
	if err != nil {
		section.add(LevelError, agentName, fmt.Sprintf("Agent %s: client init failed: %v", agentName, err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get agent info for display name (emoji + capitalized name)
	teamPath := cfg.Teams[teamName].TeamPath
	if teamPath == "" {
		teamPath = cfg.TeamPath()
	}
	agentInfo, _ := agentfs.Get(teamPath, agentName)
	titleCaser := cases.Title(language.English)
	displayName := titleCaser.String(agentName)
	if agentInfo != nil && agentInfo.Emoji != "" {
		displayName = agentInfo.Emoji + " " + displayName
	}
	if err := client.SetDisplayName(ctx, displayName); err != nil {
		log.Printf("[matrix-provision] display name failed for %s: %v", agentName, err)
	}

	// Step 4: Create room and invite human
	createReq := &mautrix.ReqCreateRoom{
		Name:   fmt.Sprintf("%s — %s", displayName, teamName),
		Preset: "private_chat",
	}
	if matrixCfg.HumanUserID != "" {
		createReq.Invite = []id.UserID{id.UserID(matrixCfg.HumanUserID)}
		createReq.IsDirect = true
	}
	roomResp, err := client.CreateRoom(ctx, createReq)
	if err != nil {
		section.add(LevelError, agentName, fmt.Sprintf("Agent %s: room creation failed: %v", agentName, err))
		return
	}
	roomID := roomResp.RoomID

	// Step 5: Append token to .env
	envKey := strings.ToUpper(agentName) + "_MATRIX_TOKEN"
	if err := appendDotEnv(envKey, accessToken); err != nil {
		section.add(LevelError, agentName, fmt.Sprintf("Agent %s: failed to update .env: %v", agentName, err))
		return
	}

	// Step 6: Update config.toml with agent entry (appends a new TOML sub-table)
	if err := appendMatrixAgentConfig(teamName, agentName, envKey, string(roomID)); err != nil {
		section.add(LevelError, agentName, fmt.Sprintf("Agent %s: failed to update config.toml: %v", agentName, err))
		return
	}

	section.add(LevelOK, agentName,
		fmt.Sprintf("Agent %s: provisioned (user=%s, room=%s)", agentName, userID, roomID))
}

// provisionMatrixNotify creates a Matrix notification user and room, updating config.
func provisionMatrixNotify(section *Section, _ *config.Config, teamName string, matrixCfg *config.MatrixTeamConfig) {
	homeserver := matrixCfg.Homeserver

	regSecret := os.Getenv("MATRIX_REGISTRATION_SECRET")
	if regSecret == "" {
		section.add(LevelWarn, teamName+".notify",
			"MATRIX_REGISTRATION_SECRET not set — cannot provision notification bot")
		return
	}

	notifyUser := "notify-" + teamName
	password, err := registerMatrixUser(homeserver, regSecret, notifyUser)
	if err != nil {
		section.add(LevelWarn, teamName+".notify",
			fmt.Sprintf("Notification user registration failed (user likely already exists). "+
				"Add %s_MATRIX_NOTIFY_TOKEN=<token> to .env and set notification_room/notification_token_env "+
				"in [teams.%s.matrix] manually.", strings.ToUpper(teamName), teamName))
		return
	}

	accessToken, err := loginMatrixUser(homeserver, notifyUser, password)
	if err != nil {
		section.add(LevelError, teamName+".notify",
			fmt.Sprintf("Notification user login failed: %v", err))
		return
	}

	domain := extractDomainFromURL(homeserver)
	userID := id.NewUserID(notifyUser, domain)
	client, err := mautrix.NewClient(homeserver, userID, accessToken)
	if err != nil {
		section.add(LevelError, teamName+".notify", fmt.Sprintf("client init failed: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = client.SetDisplayName(ctx, "🔔 Notifications — "+teamName)

	createReq := &mautrix.ReqCreateRoom{
		Name:   fmt.Sprintf("Notifications — %s", teamName),
		Preset: "private_chat",
	}
	if matrixCfg.HumanUserID != "" {
		createReq.Invite = []id.UserID{id.UserID(matrixCfg.HumanUserID)}
	}
	roomResp, err := client.CreateRoom(ctx, createReq)
	if err != nil {
		section.add(LevelError, teamName+".notify", fmt.Sprintf("room creation failed: %v", err))
		return
	}

	// Append token to .env
	envKey := strings.ToUpper(teamName) + "_MATRIX_NOTIFY_TOKEN"
	if err := appendDotEnv(envKey, accessToken); err != nil {
		section.add(LevelError, teamName+".notify", fmt.Sprintf("failed to update .env: %v", err))
		return
	}

	// Update config.toml — read, parse, modify, write back atomically
	if err := updateMatrixNotifyConfig(teamName, envKey, string(roomResp.RoomID)); err != nil {
		section.add(LevelError, teamName+".notify", fmt.Sprintf("failed to update config.toml: %v", err))
		return
	}

	section.add(LevelOK, teamName+".notify",
		fmt.Sprintf("Notification provisioned (room=%s)", roomResp.RoomID))
}

// registerMatrixUser registers a new user via the Synapse-compatible shared-secret admin API.
// Returns the generated password on success.
func registerMatrixUser(homeserver, sharedSecret, username string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	password := generateRandomPassword(32)

	nonce, err := getRegistrationNonce(ctx, homeserver)
	if err != nil {
		return "", fmt.Errorf("get nonce: %w", err)
	}

	mac := computeRegistrationMAC(nonce, username, password, false, sharedSecret)

	reqBody := map[string]interface{}{
		"nonce":    nonce,
		"username": username,
		"password": password,
		"mac":      mac,
		"admin":    false,
	}
	body, _ := json.Marshal(reqBody)

	regURL := homeserver + "/_synapse/admin/v1/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, regURL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("registration request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("registration failed (%d): %v", resp.StatusCode, errResp)
	}

	return password, nil
}

// getRegistrationNonce fetches a one-time nonce from the homeserver admin registration endpoint.
func getRegistrationNonce(ctx context.Context, homeserver string) (string, error) {
	regURL := homeserver + "/_synapse/admin/v1/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, regURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Nonce string `json:"nonce"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Nonce, nil
}

// computeRegistrationMAC computes the HMAC-SHA1 MAC required by the Synapse shared-secret
// registration endpoint. See https://matrix-org.github.io/synapse/latest/admin_api/register_api.html
func computeRegistrationMAC(nonce, username, password string, admin bool, sharedSecret string) string {
	h := hmac.New(sha1.New, []byte(sharedSecret)) //nolint:gosec // SHA1 required by spec
	h.Write([]byte(nonce))
	h.Write([]byte{0})
	h.Write([]byte(username))
	h.Write([]byte{0})
	h.Write([]byte(password))
	h.Write([]byte{0})
	if admin {
		h.Write([]byte("admin"))
	} else {
		h.Write([]byte("notadmin"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// loginMatrixUser logs in a Matrix user and returns their access token.
func loginMatrixUser(homeserver, username, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	domain := extractDomainFromURL(homeserver)
	userID := id.NewUserID(username, domain)
	client, err := mautrix.NewClient(homeserver, userID, "")
	if err != nil {
		return "", err
	}

	resp, err := client.Login(ctx, &mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: username,
		},
		Password: password,
	})
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	return resp.AccessToken, nil
}

// appendDotEnv appends a key=value entry to the ttal .env file (append-only, never overwrites).
func appendDotEnv(key, value string) error {
	envPath, err := config.DotEnvPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(envPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintf(f, "\n%s=%s\n", key, value)
	return err
}

// appendMatrixAgentConfig appends a new [teams.<team>.matrix.agents.<agent>] TOML sub-table
// to config.toml. Safe to append because sub-tables are new sections.
func appendMatrixAgentConfig(teamName, agentName, envKey, roomID string) error {
	cfgPath, err := config.Path()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	entry := fmt.Sprintf("\n[teams.%s.matrix.agents.%s]\naccess_token_env = %q\nroom_id = %q\n",
		teamName, agentName, envKey, roomID)
	_, err = f.WriteString(entry)
	return err
}

// updateMatrixNotifyConfig reads config.toml, sets notification_token_env and notification_room
// under [teams.<team>.matrix], then atomically writes back.
func updateMatrixNotifyConfig(teamName, envKey, roomID string) error {
	cfgPath, err := config.Path()
	if err != nil {
		return err
	}

	// Read existing config as raw map to preserve unknown fields
	var raw map[string]interface{}
	if _, err := toml.DecodeFile(cfgPath, &raw); err != nil {
		return fmt.Errorf("parse config.toml: %w", err)
	}

	// Navigate to teams.<teamName>.matrix
	teams, _ := raw["teams"].(map[string]interface{})
	if teams == nil {
		return fmt.Errorf("no [teams] in config.toml")
	}
	team, _ := teams[teamName].(map[string]interface{})
	if team == nil {
		return fmt.Errorf("no [teams.%s] in config.toml", teamName)
	}
	matrixMap, _ := team["matrix"].(map[string]interface{})
	if matrixMap == nil {
		return fmt.Errorf("no [teams.%s.matrix] in config.toml", teamName)
	}

	// Set the notification fields
	matrixMap["notification_token_env"] = envKey
	matrixMap["notification_room"] = roomID

	// Atomic write — write to tmp, then rename (prevents corruption on encode failure)
	tmp := cfgPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if encErr := toml.NewEncoder(f).Encode(raw); encErr != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return encErr
	}
	_ = f.Close()
	return os.Rename(tmp, cfgPath)
}

// generateRandomPassword generates a URL-safe base64 string from n random bytes.
func generateRandomPassword(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// extractDomainFromURL extracts the host portion from a homeserver URL.
// e.g. "https://matrix.example.com" → "matrix.example.com"
func extractDomainFromURL(homeserverURL string) string {
	u, err := url.Parse(homeserverURL)
	if err != nil || u.Host == "" {
		return homeserverURL
	}
	return u.Host
}
