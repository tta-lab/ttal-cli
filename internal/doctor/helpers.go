package doctor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tta-lab/ttal-cli/ent"
)

func countAgents(dbPath string) (int, error) {
	dsn := fmt.Sprintf("file:%s?cache=shared&_fk=1&_journal_mode=WAL&_busy_timeout=5000&mode=ro", dbPath)
	client, err := ent.Open("sqlite3", dsn)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	return client.Agent.Query().Count(context.Background())
}

func isVoiceServerRunning() bool {
	client := &http.Client{
		Timeout: 1 * time.Second,
	}
	resp, err := client.Get("http://localhost:8877/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
