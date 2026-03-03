package doctor

import (
	"net/http"
	"time"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
)

func countAgents(teamPath string) (int, error) {
	if teamPath == "" {
		return 0, nil
	}
	return agentfs.Count(teamPath)
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
