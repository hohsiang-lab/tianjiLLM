package openaitest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpstreamMockServer_ModelsEndpoint(t *testing.T) {
	server := NewUpstreamServer(t, UpstreamServerOptions{
		Models: []ModelFixture{{ID: "gpt-4o", OwnedBy: "openai"}},
	})

	resp, err := server.Client().Get(server.BaseURL() + "/models")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "list", body["object"])
	data, ok := body["data"].([]any)
	require.True(t, ok)
	firstModel, ok := data[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gpt-4o", firstModel["id"])

	requests := server.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "/v1/models", requests[0].Path)
}

func TestUpstreamMockServer_ChatCompletionsEndpoint(t *testing.T) {
	server := NewUpstreamServer(t, UpstreamServerOptions{})
	payload := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	req, err := http.NewRequest(http.MethodPost, server.BaseURL()+"/chat/completions", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "chat.completion", body["object"])

	requests := server.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "/v1/chat/completions", requests[0].Path)
	assert.Equal(t, "Bearer sk-test", requests[0].Header.Get("Authorization"))
	assert.JSONEq(t, string(payload), string(requests[0].Body))
}

func TestUpstreamMockServer_EmbeddingsEndpoint(t *testing.T) {
	server := NewUpstreamServer(t, UpstreamServerOptions{})
	payload := []byte(`{"model":"text-embedding-3-small","input":"hello"}`)
	req, err := http.NewRequest(http.MethodPost, server.BaseURL()+"/embeddings", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer sk-test")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "list", body["object"])

	requests := server.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "/v1/embeddings", requests[0].Path)
	assert.JSONEq(t, string(payload), string(requests[0].Body))
}

func TestUpstreamMockServer_ResponseHeadersFixture(t *testing.T) {
	server := NewUpstreamServer(t, UpstreamServerOptions{
		ResponseHeaders: http.Header{
			"x-openai-mock-quota-status":      []string{"allowed"},
			"x-openai-mock-quota-utilization": []string{"0.25"},
		},
	})

	resp, err := server.Client().Get(server.BaseURL() + "/models")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "allowed", resp.Header.Get("x-openai-mock-quota-status"))
	assert.Equal(t, "0.25", resp.Header.Get("x-openai-mock-quota-utilization"))
}

func TestUpstreamMockServer_ChatCompletionsSSE(t *testing.T) {
	server := NewUpstreamServer(t, UpstreamServerOptions{
		ChatCompletionSSE: []string{
			`{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
			`[DONE]`,
		},
	})

	resp, err := server.Client().Post(server.BaseURL()+"/chat/completions", "application/json", bytes.NewBufferString(`{"stream":true}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Contains(t, string(body), `"content":"ok"`)
	assert.Contains(t, string(body), "data: [DONE]")
}
