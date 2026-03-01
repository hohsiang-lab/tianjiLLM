package passthrough

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test 2: Passthrough handler should NOT overwrite client-provided anthropic-version.
func TestPassthrough_PreservesClientAnthropicVersion(t *testing.T) {
	var receivedVersion string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedVersion = r.Header.Get("anthropic-version")
		w.WriteHeader(http.StatusOK)
		_,_ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	cfg := Config{
		ProviderEndpoints: map[string]string{
			"/anthropic": backend.URL,
		},
		APIKeys: map[string]string{
			"anthropic": "sk-ant-api03-regularkey",
		},
	}

	handler := Handler(cfg)

	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", nil)
	req.Header.Set("anthropic-version", "2024-01-01")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "2024-01-01", receivedVersion,
		"client-provided anthropic-version should be preserved, not overwritten with 2023-06-01")
}
