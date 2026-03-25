package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/internal/config"
)

func TestHandleAsk_InvalidMode(t *testing.T) {
	cfg := &config.Config{}
	handler := handleAsk(cfg)

	body, err := json.Marshal(map[string]string{
		"question": "test",
		"mode":     "invalid",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp SendResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "invalid mode")
}

func TestHandleAsk_InvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	handler := handleAsk(cfg)

	req := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
