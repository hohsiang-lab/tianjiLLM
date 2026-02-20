package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/policy"
	"github.com/stretchr/testify/assert"
)

func TestPolicyEngine_Evaluate(t *testing.T) {
	eng := policy.NewEngine(nil)

	// Engine with no policies returns empty guardrails
	result, err := eng.Evaluate(context.Background(), policy.MatchRequest{
		Model:  "gpt-4o",
		TeamID: "team-1",
		KeyID:  "key-1",
	})
	assert.NoError(t, err)
	assert.Empty(t, result.Guardrails)
}

func TestPolicyEndpoints_RequireAuth(t *testing.T) {
	srv := newIntegrationServer(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/policy/"},
		{http.MethodGet, "/policy/list"},
		{http.MethodPost, "/policy/attachment"},
		{http.MethodGet, "/policy/attachment/list"},
		{http.MethodPost, "/policy/test-pipeline"},
		{http.MethodGet, "/policy/resolved-guardrails"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.method == http.MethodPost {
				req = httptest.NewRequest(ep.method, ep.path, strings.NewReader("{}"))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestPolicyCRUD_WithAuth_NoDB(t *testing.T) {
	srv := newIntegrationServer(t)

	body := `{"name":"test","conditions":{"model":"gpt-4o"}}`
	req := httptest.NewRequest(http.MethodPost, "/policy/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Without DB, should return 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
