package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	usagePollInterval = 30 * time.Minute
	usageAPIURL       = "https://api.anthropic.com/api/oauth/usage"
	usageAPITimeout   = 5 * time.Second
)

// UsageData holds the parsed Claude.ai usage response.
type UsageData struct {
	SessionUsage   *float64  `json:"sessionUsage,omitempty"`
	SessionResetAt string    `json:"sessionResetAt,omitempty"`
	WeeklyUsage    *float64  `json:"weeklyUsage,omitempty"`
	WeeklyResetAt  string    `json:"weeklyResetAt,omitempty"`
	FetchedAt      time.Time `json:"fetchedAt"`
	Error          string    `json:"error,omitempty"`
}

var (
	usageMu    sync.RWMutex
	usageCache *UsageData
)

// getUsageCache returns the current in-memory usage data (nil if not yet fetched).
func getUsageCache() *UsageData {
	usageMu.RLock()
	defer usageMu.RUnlock()
	return usageCache
}

func setUsageCache(d *UsageData) {
	usageMu.Lock()
	usageCache = d
	usageMu.Unlock()
}

// startUsagePoller fetches usage immediately then polls every 30 minutes.
func startUsagePoller(done <-chan struct{}) {
	// Try to warm cache from disk on startup
	if d, err := readUsageDiskCache(); err == nil {
		setUsageCache(d)
	} else if !os.IsNotExist(err) {
		log.Printf("[usage] ignoring bad disk cache: %v", err)
	}

	go func() {
		// Fetch immediately on start
		fetchAndCacheUsage()

		ticker := time.NewTicker(usagePollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fetchAndCacheUsage()
			}
		}
	}()
}

func fetchAndCacheUsage() {
	token, err := getOAuthToken()
	if err != nil {
		log.Printf("[usage] no OAuth token: %v", err)
		return
	}

	data, err := fetchUsageFromAPI(token)
	if err != nil {
		log.Printf("[usage] fetch error: %v", err)
		return
	}

	data.FetchedAt = time.Now()
	setUsageCache(data)
	if err := writeUsageDiskCache(data); err != nil {
		log.Printf("[usage] disk write error: %v", err)
	}
	log.Printf("[usage] fetched — 5hr: %.0f%%, weekly: %.0f%%",
		pctOrZero(data.SessionUsage), pctOrZero(data.WeeklyUsage))
}

func getOAuthToken() (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return "", fmt.Errorf("parse keychain json: %w", err)
	}
	if creds.ClaudeAiOauth.AccessToken == "" {
		return "", fmt.Errorf("accessToken empty in keychain")
	}
	return creds.ClaudeAiOauth.AccessToken, nil
}

func fetchUsageFromAPI(token string) (*UsageData, error) {
	req, err := http.NewRequest("GET", usageAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{Timeout: usageAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited (429)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api status %d", resp.StatusCode)
	}

	var raw struct {
		FiveHour struct {
			Utilization *float64 `json:"utilization"`
			ResetsAt    string   `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay struct {
			Utilization *float64 `json:"utilization"`
			ResetsAt    string   `json:"resets_at"`
		} `json:"seven_day"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return &UsageData{
		SessionUsage:   raw.FiveHour.Utilization,
		SessionResetAt: raw.FiveHour.ResetsAt,
		WeeklyUsage:    raw.SevenDay.Utilization,
		WeeklyResetAt:  raw.SevenDay.ResetsAt,
	}, nil
}

func usageDiskCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttal", "usage.json"), nil
}

func readUsageDiskCache() (*UsageData, error) {
	path, err := usageDiskCachePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d UsageData
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func writeUsageDiskCache(d *UsageData) error {
	path, err := usageDiskCachePath()
	if err != nil {
		return err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func pctOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
