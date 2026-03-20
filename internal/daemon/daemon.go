package daemon

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/tta-lab/ttal-cli/internal/comment"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/message"

	_ "modernc.org/sqlite"
)

const pidFileName = "daemon.pid"

// Run starts the daemon in the foreground. This is what launchd calls.
// Config-driven: loads all teams from config.toml, no database required.
func Run() error {
	mcfg, err := config.LoadAll()
	if err != nil {
		return err
	}

	if running, pid, _ := IsRunning(); running {
		return fmt.Errorf("daemon already running (pid=%d)", pid)
	}

	pidPath, err := setupDataDir()
	if err != nil {
		return err
	}
	defer os.Remove(pidPath)

	sockPath, err := SocketPath()
	if err != nil {
		return err
	}

	// Open SQLite message database.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	ttalDir := filepath.Join(home, ".ttal")
	if err := os.MkdirAll(ttalDir, 0o755); err != nil {
		return fmt.Errorf("create message db dir: %w", err)
	}
	dbPath := filepath.Join(ttalDir, "messages.db")
	// modernc/sqlite uses _pragma= syntax; foreign_keys and WAL mode required.
	dbDSN := "file:" + dbPath + "?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	drv, err := entsql.Open("sqlite", dbDSN)
	if err != nil {
		return fmt.Errorf("open message db: %w", err)
	}
	entClient := ent.NewClient(ent.Driver(entsql.OpenDB("sqlite3", drv.DB())))
	defer func() { _ = entClient.Close() }()
	if err := entClient.Schema.Create(context.Background()); err != nil {
		return fmt.Errorf("migrate message schema: %w", err)
	}
	msgSvc := message.NewService(entClient)
	commentSvc := comment.NewService(entClient)

	log.Printf("[daemon] starting — http=%s teams=%d",
		sockPath, len(mcfg.Teams))

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := newAdapterRegistry()

	// Run adapter init concurrently with command discovery — they're independent.
	var startupWg sync.WaitGroup
	var discovered []BotCommand

	startupWg.Add(1)
	go func() {
		defer startupWg.Done()
		initAdapters(mcfg)
	}()

	startupWg.Add(1)
	go func() {
		defer startupWg.Done()
		discovered = DiscoverCommands(mcfg.Global.Sync.CommandsPaths)
	}()

	startupWg.Wait()

	// Build frontends per-team and register commands.
	// Frontends capture registry and frontends itself via closure, which is
	// fully populated before Start is called below.
	frontends := buildFrontends(mcfg, registry, msgSvc)
	registerFrontendCommands(frontends, AllCommands(discovered))

	// Start all frontends.
	if err := startFrontends(ctx, done, frontends); err != nil {
		return err
	}

	startUsagePoller(done)
	startHeartbeatScheduler(mcfg, registry, frontends, done)
	startCleanupWatcher(done)
	startPRWatcher(mcfg, frontends, done)
	startReminderPoller(mcfg, frontends, done)
	startWatcher(mcfg, frontends, msgSvc, done)

	shellCfg, err := config.Load()
	if err != nil {
		log.Printf("[daemon] warning: failed to load shell config: %v", err)
		shellCfg = &config.Config{}
	}

	// Pick a default frontend for HTTP handlers that need one.
	defaultFE := frontends[mcfg.DefaultTeamName()]
	if defaultFE == nil {
		close(done)
		return fmt.Errorf("default team %q has no frontend — check config", mcfg.DefaultTeamName())
	}

	askHumanHandler := defaultFE.AskHumanHTTPHandler()

	srv, err := listenHTTP(sockPath, httpHandlers{
		send: func(req SendRequest) error {
			return handleSend(mcfg, registry, frontends, msgSvc, req)
		},
		statusUpdate: handleStatusUpdate,
		taskComplete: func(req TaskCompleteRequest) SendResponse {
			return handleTaskComplete(req, mcfg, registry, frontends)
		},
		breathe: func(req BreatheRequest) SendResponse {
			return handleBreathe(shellCfg, req)
		},
		askHuman: askHumanHandler,
		pipelineAdvance: func(w http.ResponseWriter, r *http.Request) {
			handlePipelineAdvance(w, r, defaultFE, mcfg, string(shellCfg.WorkerRuntime()))
		},
		commentAdd: func(req CommentAddRequest) CommentAddResponse {
			return handleCommentAdd(commentSvc, mcfg.DefaultTeamName(), req)
		},
		commentList: func(req CommentListRequest) CommentListResponse {
			return handleCommentList(commentSvc, mcfg.DefaultTeamName(), req)
		},
		prCreate:              handlePRCreate,
		prModify:              handlePRModify,
		prMerge:               handlePRMerge,
		prCheckMergeable:      handlePRCheckMergeable,
		prCommentCreate:       handlePRCommentCreate,
		prCommentList:         handlePRCommentList,
		prGetPR:               handlePRGetPR,
		prGetCombinedStatus:   handlePRGetCombinedStatus,
		prGetCIFailureDetails: handlePRGetCIFailureDetails,
	})
	if err != nil {
		close(done)
		return err
	}

	log.Printf("[daemon] ready")
	if err := defaultFE.SendNotification(ctx, "✅ Daemon ready"); err != nil {
		log.Printf("[daemon] warning: failed to send ready notification: %v", err)
	}
	awaitShutdown(done, cancel, mcfg, registry, srv)
	return nil
}

// formatUsageString formats UsageData into a human-readable string for the /usage command.
// Returns "" if data is nil (not yet fetched).
func formatUsageString(d *UsageData) string {
	if d == nil {
		return ""
	}
	if d.Error != "" {
		return "Usage fetch error: " + d.Error
	}
	var parts []string
	if d.SessionUsage != nil {
		line := fmt.Sprintf("5-hour:  %.0f%% used", *d.SessionUsage)
		if d.SessionResetAt != "" {
			line += " (resets in " + formatResetAt(d.SessionResetAt) + ")"
		}
		parts = append(parts, line)
	}
	if d.WeeklyUsage != nil {
		line := fmt.Sprintf("Weekly:  %.0f%% used", *d.WeeklyUsage)
		if d.WeeklyResetAt != "" {
			line += " (resets in " + formatResetAt(d.WeeklyResetAt) + ")"
		}
		parts = append(parts, line)
	}
	if len(parts) == 0 {
		return "No usage data available"
	}
	header := "Claude API Usage\n" + strings.Repeat("─", 16)
	body := strings.Join(parts, "\n")
	if d.FetchedAt.IsZero() {
		return header + "\n" + body
	}
	return header + "\n" + body + "\n" + fmt.Sprintf("(as of %s)", d.FetchedAt.Format("15:04"))
}

// formatResetAt formats an RFC3339 reset time as a short duration string.
func formatResetAt(resetAt string) string {
	t, err := time.Parse(time.RFC3339, resetAt)
	if err != nil {
		return resetAt
	}
	remaining := time.Until(t)
	if remaining < 0 {
		return "now"
	}
	return formatDuration(remaining)
}

// formatDuration formats a duration as a compact human-readable string.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h >= 24 {
		days := h / 24
		h = h % 24
		if h == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, h)
	}
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// setupDataDir creates ~/.ttal/ and writes the PID file. Returns the PID file path.
func setupDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dataDir := filepath.Join(home, ".ttal")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", err
	}
	pidPath := filepath.Join(dataDir, pidFileName)
	if err := writePID(pidPath); err != nil {
		return "", fmt.Errorf("failed to write pid file: %w", err)
	}
	return pidPath, nil
}

// awaitShutdown waits for SIGINT/SIGTERM and performs graceful shutdown.
func awaitShutdown(
	done chan struct{}, cancel context.CancelFunc,
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	srv *http.Server,
) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	s := <-sig
	log.Printf("[daemon] received signal %v — shutting down", s)
	close(done)
	cancel()
	shutdownAgents(mcfg, registry)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[daemon] HTTP server shutdown error: %v", err)
	}
}

// IsRunning checks whether the daemon is running by inspecting the pid file.
// Uses fixed path at ~/.ttal/daemon.pid.
func IsRunning() (bool, int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, 0, fmt.Errorf("user home dir: %w", err)
	}
	pidPath := filepath.Join(home, ".ttal", pidFileName)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false, 0, nil
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false, 0, nil
	}

	// Check if process is alive
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0, nil
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid, nil
	}

	return true, pid, nil
}

func writePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// buildFrontends constructs one Frontend per team. The frontends map is passed by
// reference into the OnMessage closure so message routing can use the full map.
func buildFrontends(
	mcfg *config.DaemonConfig, registry *adapterRegistry, msgSvc *message.Service,
) map[string]frontend.Frontend {
	frontends := make(map[string]frontend.Frontend)
	for teamName, team := range mcfg.Teams {
		ft := team.Frontend
		if ft == "" {
			ft = "telegram" // backward compatible default
		}

		// onMsg is shared across all frontend types — extract to avoid duplication.
		onMsg := func(team, agent, text string) {
			if err := deliverToAgent(registry, mcfg, frontends, team, agent, text); err != nil {
				log.Printf("[daemon] deliverToAgent %s/%s failed: %v", team, agent, err)
			}
		}

		switch ft {
		case "telegram":
			fe := frontend.NewTelegram(frontend.TelegramConfig{
				TeamName:   teamName,
				MCfg:       mcfg,
				OnMessage:  onMsg,
				MsgSvc:     msgSvc,
				UserNameFn: func() string { return mcfg.UserNameForTeam(teamName) },
				GetUsageFn: func() string { return formatUsageString(getUsageCache()) },
				RestartFn:  Restart,
			})
			frontends[teamName] = fe
		case "matrix":
			fe, err := frontend.NewMatrix(frontend.MatrixConfig{
				TeamName:   teamName,
				MCfg:       mcfg,
				OnMessage:  onMsg,
				MsgSvc:     msgSvc,
				UserNameFn: func() string { return mcfg.UserNameForTeam(teamName) },
				GetUsageFn: func() string { return formatUsageString(getUsageCache()) },
				RestartFn:  Restart,
			})
			if err != nil {
				log.Printf("[daemon] matrix frontend failed for team %s: %v — skipping", teamName, err)
				continue
			}
			frontends[teamName] = fe
		default:
			log.Printf("[daemon] unknown frontend %q for team %s — skipping", ft, teamName)
		}
	}
	return frontends
}

// registerFrontendCommands converts BotCommand slice to frontend.Command slice
// and registers them with every frontend.
func registerFrontendCommands(frontends map[string]frontend.Frontend, cmds []BotCommand) {
	feCmds := make([]frontend.Command, len(cmds))
	for i, c := range cmds {
		feCmds[i] = frontend.Command{
			Name:         c.Command,
			Description:  c.Description,
			OriginalName: c.OriginalName,
		}
	}
	for teamName, fe := range frontends {
		if err := fe.RegisterCommands(feCmds); err != nil {
			log.Printf("[daemon] RegisterCommands for team %s failed: %v", teamName, err)
		}
	}
}

// startFrontends calls Start for every frontend.
// StartNotificationPoller is called internally by TelegramFrontend.Start.
func startFrontends(ctx context.Context, done chan struct{}, frontends map[string]frontend.Frontend) error {
	for teamName, fe := range frontends {
		if err := fe.Start(ctx); err != nil {
			close(done)
			return fmt.Errorf("start frontend for team %s: %w", teamName, err)
		}
	}
	return nil
}
