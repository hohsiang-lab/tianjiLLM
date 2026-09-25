package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/require"
)

type requestBodyReadError struct {
	reader *bytes.Reader
}

func (r *requestBodyReadError) Read(p []byte) (int, error) {
	if r.reader.Len() == 0 {
		return 0, errors.New("request body timeout")
	}
	return r.reader.Read(p)
}

func (*requestBodyReadError) Close() error { return nil }

func TestCreateResponse_RequestBodyReadFailureDoesNotFallbackTo501(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()

	for _, path := range []string{"/v1/responses", "/v1/responses?model=gpt-5.6-luna"} {
		t.Run(path, func(t *testing.T) {
			h := &Handlers{Config: &config.ProxyConfig{AssistantSettings: &config.AssistantSettings{APIBase: upstream.URL}}}
			body := `{"model":"gpt-5.6-luna","input":"` + strings.Repeat("x", 128<<10) + `"}`
			req := httptest.NewRequest(http.MethodPost, path, http.NoBody)
			req.Body = &requestBodyReadError{reader: bytes.NewReader([]byte(body))}
			w := httptest.NewRecorder()

			h.CreateResponse(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "read request body: request body timeout")
			require.NotContains(t, w.Body.String(), "OpenAI endpoint not configured")
			require.Zero(t, recorder.count())
		})
	}
}

func TestCreateResponse_OversizedBodyDoesNotFallbackTo501(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()
	h := &Handlers{Config: &config.ProxyConfig{AssistantSettings: &config.AssistantSettings{APIBase: upstream.URL}}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses?model=gpt-5.6-luna", http.NoBody)
	req.Body = &requestBodyReadError{reader: bytes.NewReader(make([]byte, maxOpenAIRequestBodyBytes+1))}
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "request body exceeds 33554432-byte limit")
	require.NotContains(t, w.Body.String(), "OpenAI endpoint not configured")
	require.Zero(t, recorder.count())
}

func TestCreateResponse_UnknownQueryModelReturnsNotFoundWithoutConfig(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses?model=gpt-5.6-luna", strings.NewReader(`{"input":"hi"}`))
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "model_not_found")
}
