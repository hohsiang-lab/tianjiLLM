package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQuickstart_EndpointSmokeTests verifies all key endpoints respond
// with correct status codes and response formats, matching Python LiteLLM behavior.
func TestQuickstart_EndpointSmokeTests(t *testing.T) {
	apiKey := "sk-test"
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{
					ModelName: "gpt-4o",
					TianjiParams: config.TianjiParams{
						Model:  "openai/gpt-4o",
						APIKey: &apiKey,
					},
				},
			},
			RouterSettings: &config.RouterSettings{
				RoutingStrategy: "simple-shuffle",
			},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		checkBody  func(t *testing.T, body []byte)
	}{
		{
			name:       "health check",
			method:     "GET",
			path:       "/health",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]any
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, "healthy", resp["status"])
			},
		},
		{
			name:       "health readiness",
			method:     "GET",
			path:       "/health/readiness",
			wantStatus: http.StatusOK,
		},
		{
			name:       "health liveness",
			method:     "GET",
			path:       "/health/liveness",
			wantStatus: http.StatusOK,
		},
		{
			name:       "list models",
			method:     "GET",
			path:       "/v1/models",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]any
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, "list", resp["object"])
				data, _ := resp["data"].([]any)
				assert.Len(t, data, 1)
				first, _ := data[0].(map[string]any)
				assert.Equal(t, "gpt-4o", first["id"])
			},
		},
		{
			name:       "router settings GET",
			method:     "GET",
			path:       "/router/settings",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]any
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, "simple-shuffle", resp["RoutingStrategy"])
			},
		},
		{
			name:       "key info without DB returns error",
			method:     "GET",
			path:       "/key/info",
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *strings.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			} else {
				body = strings.NewReader("")
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			req.Header.Set("Authorization", "Bearer sk-master")
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "status code mismatch for %s", tt.path)

			if tt.checkBody != nil {
				tt.checkBody(t, rec.Body.Bytes())
			}
		})
	}
}
