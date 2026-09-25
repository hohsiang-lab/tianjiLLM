package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuredOutput_PreservesStandardResponseFormatExactly(t *testing.T) {
	tests := []struct {
		name   string
		format map[string]any
	}{
		{
			name:   "json object",
			format: map[string]any{"type": "json_object"},
		},
		{
			name: "json schema",
			format: map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":        "entity",
					"description": "Extract one entity",
					"schema": map[string]any{
						"type":                 "object",
						"properties":           map[string]any{"name": map[string]any{"type": "string"}},
						"required":             []any{"name"},
						"additionalProperties": false,
					},
					"strict": true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, upstream := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})
			requestBody, err := json.Marshal(map[string]any{
				"model":           contractModel,
				"messages":        []any{map[string]any{"role": "user", "content": "extract"}},
				"response_format": tt.format,
			})
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodPost, "/v1/chat/completions", string(requestBody)))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			requests := upstream.Requests()
			require.Len(t, requests, 1)
			var upstreamBody map[string]any
			require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
			assert.Equal(t, tt.format, upstreamBody["response_format"])
			assert.NotContains(t, upstreamBody, "text")
		})
	}
}
