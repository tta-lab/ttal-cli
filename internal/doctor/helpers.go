package doctor

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"

	"github.com/tta-lab/ttal-cli/ent"
)

func countAgents(dbPath string) (int, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return 0, err
	}
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))
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
