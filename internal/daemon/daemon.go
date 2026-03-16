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
	"sync"
	"syscall"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/notify"

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

	log.Printf("[daemon] starting — http=%s teams=%d",
		sockPath, len(mcfg.Teams))

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx // reserved for future use

	registry := newAdapterRegistry()
	ahs := newAskHumanStore()
	mt := newMessageTracker()

	// Run adapter init and bot command registration concurrently — they're independent.
	var startupWg sync.WaitGroup
	var allCommands []BotCommand

	startupWg.Add(1)
	go func() {
		defer startupWg.Done()
		initAdapters(mcfg)
	}()

	startupWg.Add(1)
	go func() {
		defer startupWg.Done()
		allCommands = discoverAndRegisterCommands(mcfg)
	}()

	startupWg.Wait()
	startTelegramPollers(mcfg, registry, done, ahs, allCommands, mt, msgSvc)
	startNotificationPollers(mcfg, done)
	startUsagePoller(done)
	startHeartbeatScheduler(mcfg, registry, done)
	startCleanupWatcher(done)
	startPRWatcher(mcfg, done)
	startReminderPoller(mcfg, done)
	startWatcher(mcfg, mt, msgSvc, done)

	srv, err := listenHTTP(sockPath, httpHandlers{
		send: func(req SendRequest) error {
			return handleSend(mcfg, registry, msgSvc, req)
		},
		statusUpdate: handleStatusUpdate,
		taskComplete: func(req TaskCompleteRequest) SendResponse {
			return handleTaskComplete(req, mcfg, registry)
		},
		askHuman: handleHTTPAskHuman(ahs, mcfg),
	})
	if err != nil {
		close(done)
		return err
	}

	log.Printf("[daemon] ready")
	notifyDaemonReady(mcfg)
	awaitShutdown(done, cancel, mcfg, registry, srv)
	return nil
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

// notifyDaemonReady sends a startup notification to the default team via its notification bot token.
func notifyDaemonReady(mcfg *config.DaemonConfig) {
	defaultTeam := mcfg.DefaultTeamName()
	team, ok := mcfg.Teams[defaultTeam]
	if !ok {
		log.Printf("[daemon] warning: default team %q not found in config", defaultTeam)
		return
	}
	if err := notify.SendWithConfig(team.NotificationToken, team.ChatID, "✅ Daemon ready"); err != nil {
		log.Printf("[daemon] warning: failed to send ready notification: %v", err)
	}
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
