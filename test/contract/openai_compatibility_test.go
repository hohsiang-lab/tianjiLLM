package contract

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardClientRoutes_DoNotDependOnCallerIdentity(t *testing.T) {
	srv, upstream := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})
	clients := []string{"Graphiti", "Cognee", "OpenAI-SDK", "LiteLLM", ""}

	for _, client := range clients {
		name := client
		if name == "" {
			name = "anonymous"
		}
		t.Run(name, func(t *testing.T) {
			tests := []struct {
				method string
				path   string
				body   string
			}{
				{http.MethodGet, "/v1/models/" + contractModel, ""},
				{http.MethodPost, "/v1/chat/completions", `{"model":"contract-model","messages":[{"role":"user","content":"hi"}]}`},
				{http.MethodPost, "/v1/embeddings", `{"model":"contract-model","input":"hello"}`},
			}

			for _, tt := range tests {
				req := newAuthenticatedContractRequest(tt.method, tt.path, tt.body)
				req.Header.Set("User-Agent", client)
				req.Header.Set("X-Client-Name", client)
				recorder := httptest.NewRecorder()
				srv.ServeHTTP(recorder, req)
				require.Equal(t, http.StatusOK, recorder.Code, "%s %s: %s", tt.method, tt.path, recorder.Body.String())
			}
		})
	}

	requests := upstream.Requests()
	require.Len(t, requests, len(clients)*2)
	for i, request := range requests {
		wantPath := "/v1/chat/completions"
		if i%2 == 1 {
			wantPath = "/v1/embeddings"
		}
		assert.Equal(t, wantPath, request.Path)
	}
}
