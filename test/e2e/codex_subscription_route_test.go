//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/spend"
)

func TestCodexSubscriptionRouteE2E_DBManagedWildcardStreamsFromCredential(t *testing.T) {
	f := setup(t)
	var captureMu sync.Mutex
	var capturedAuth string
	var capturedAccountID string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-api/wham/usage" {
			writeCodexUsageSnapshot(t, w, "stream@example.com")
			return
		}
		require.Equal(t, "/backend-api/codex/responses", r.URL.Path)
		captureMu.Lock()
		capturedAuth = r.Header.Get("Authorization")
		capturedAccountID = r.Header.Get("ChatGPT-Account-Id")
		captureMu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.output_text.delta","response_id":"resp_e2e","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_e2e","model":"gpt-5.5","usage":{"input_tokens":7,"output_tokens":1,"total_tokens":8}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	platformCalls := configureCodexResponsesE2ERoute(t, f, codexResponsesE2EConfig{
		CredentialID: "cred-codex-stream",
		Email:        "stream@example.com",
		AccessToken:  "access-stream",
		AccountID:    "acct_stream",
		BackendURL:   backend.URL,
	})

	resp := postProxyResponses(t, `{
		"model":"openai/gpt-5.5",
		"stream":true,
		"instructions":"You are concise.",
		"input":"say OK"
	}`)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	assert.Equal(t, int32(0), platformCalls.Load())
	captureMu.Lock()
	assert.Equal(t, "Bearer access-stream", capturedAuth)
	assert.Equal(t, "acct_stream", capturedAccountID)
	captureMu.Unlock()
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	assert.Contains(t, string(body), `"type":"response.output_text.delta"`)
	assert.Contains(t, string(body), `"delta":"OK"`)
	assert.Contains(t, string(body), `data: [DONE]`)
}

func TestCodexResponsesRouteE2E_DBManagedWildcardUsesCredential(t *testing.T) {
	f := setup(t)
	credID := f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   "cred-codex-responses",
		Name: "Codex responses seat",
		Info: map[string]any{"email": "responses@example.com", "status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-responses",
		RefreshToken: "refresh-responses",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct_responses",
	})

	var platformCalls atomic.Int32
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCalls.Add(1)
		t.Fatalf("responses route must not call Platform path %s", r.URL.Path)
	}))
	defer platform.Close()

	var captureMu sync.Mutex
	var capturedPaths []string
	var capturedAuth string
	var capturedAccountID string
	var capturedBody map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captureMu.Lock()
		capturedPaths = append(capturedPaths, r.URL.Path)
		captureMu.Unlock()
		if r.URL.Path == "/backend-api/wham/usage" {
			writeCodexUsageSnapshot(t, w, "responses@example.com")
			return
		}
		require.Equal(t, "/backend-api/codex/responses", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		captureMu.Lock()
		capturedAuth = r.Header.Get("Authorization")
		capturedAccountID = r.Header.Get("ChatGPT-Account-Id")
		capturedBody = body
		captureMu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_http","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_http","model":"gpt-5.5","usage":{"input_tokens":7,"output_tokens":1,"total_tokens":8}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	oldBaseURL := cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL
	cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	t.Cleanup(func() {
		cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = oldBaseURL
	})

	f.SeedModel(SeedModelOpts{
		ModelName: "openai/*",
		Model:     "openai/*",
		APIBase:   platform.URL + "/v1",
		Extra: map[string]any{
			"openai_subscription_credential_ids": []string{credID},
			"openai_subscription_transport":      "chatgpt_codex_backend",
		},
	})
	require.NoError(t, proxyHandler.RefreshRuntimeModels(context.Background()))

	resp := postProxyResponses(t, `{
		"model":"openai/gpt-5.5",
		"input":"say OK",
		"stream":true
	}`)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	assert.Equal(t, int32(0), platformCalls.Load())
	captureMu.Lock()
	assert.Contains(t, capturedPaths, "/backend-api/codex/responses")
	assert.Equal(t, "Bearer access-responses", capturedAuth)
	assert.Equal(t, "acct_responses", capturedAccountID)
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	assert.Equal(t, false, capturedBody["store"])
	captureMu.Unlock()
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	assert.Contains(t, string(body), `"type":"response.output_text.delta"`)
	assert.Contains(t, string(body), `data: [DONE]`)
}

func TestCodexResponsesRouteE2E_RejectsMalformedOutputImageURLBeforeCodexBackend(t *testing.T) {
	f := setup(t)
	var backendCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	platformCalls := configureCodexResponsesE2ERoute(t, f, codexResponsesE2EConfig{
		CredentialID: "cred-codex-output-invalid",
		Email:        "output-invalid@example.com",
		AccessToken:  "access-output-invalid",
		AccountID:    "acct_output_invalid",
		BackendURL:   backend.URL,
	})

	resp := postProxyResponses(t, e2eObservedOutputImageURLPayload("data:image/png,not-base64"))
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode, string(body))
	assert.Contains(t, string(body), "input[63].output[1].image_url")
	assert.Contains(t, string(body), ";base64")
	assert.NotContains(t, string(body), "not-base64")
	assert.Equal(t, int32(0), backendCalls.Load())
	assert.Equal(t, int32(0), platformCalls.Load())
}

func TestCodexResponsesRouteE2E_ForwardsValidOutputImageDataURLToCodexBackend(t *testing.T) {
	f := setup(t)
	var capturedBody map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-api/wham/usage" {
			writeCodexUsageSnapshot(t, w, "output-valid@example.com")
			return
		}
		require.Equal(t, "/backend-api/codex/responses", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_output_valid","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_output_valid","model":"gpt-5.5","usage":{"input_tokens":7,"output_tokens":1,"total_tokens":8}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	platformCalls := configureCodexResponsesE2ERoute(t, f, codexResponsesE2EConfig{
		CredentialID: "cred-codex-output-valid",
		Email:        "output-valid@example.com",
		AccessToken:  "access-output-valid",
		AccountID:    "acct_output_valid",
		BackendURL:   backend.URL,
	})

	resp := postProxyResponses(t, e2eObservedOutputImageURLPayload("data:image/png;base64,aW1hZ2U="))
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	image := e2eCapturedResponseOutputImageItem(t, capturedBody, 63, 1)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "data:image/png;base64,aW1hZ2U=", image["image_url"])
	assert.Equal(t, int32(0), platformCalls.Load())
}

func TestCodexResponsesWebSocketE2E_RejectsMalformedPreviousOutputImageURLBeforeCodexBackend(t *testing.T) {
	f := setup(t)
	var codexCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-api/wham/usage" {
			writeCodexUsageSnapshot(t, w, "output-ws@example.com")
			return
		}
		require.Equal(t, "/backend-api/codex/responses", r.URL.Path)
		call := codexCalls.Add(1)
		require.Equal(t, int32(1), call, "malformed expanded payload must not make a second backend request")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_image_output","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_image_output","model":"gpt-5.5","output":[{"type":"output_text","text":"created"},{"type":"input_image","image_url":"data:image/png,not-base64"}]}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	configureCodexResponsesE2ERoute(t, f, codexResponsesE2EConfig{
		CredentialID: "cred-codex-output-ws",
		Email:        "output-ws@example.com",
		AccessToken:  "access-output-ws",
		AccountID:    "acct_output_ws",
		BackendURL:   backend.URL,
	})

	conn := dialProxyResponsesWebSocket(t)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()
	writeProxyResponsesWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"make image"}]}]
	}`)
	assert.Equal(t, "response.created", readProxyResponsesWebSocketJSON(t, conn)["type"])
	firstCompleted := readProxyResponsesWebSocketJSON(t, conn)
	assert.Equal(t, "response.completed", firstCompleted["type"])
	responseID := e2eResponseIDFromEvent(t, firstCompleted)

	writeProxyResponsesWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":`+jsonQuote(responseID)+`,
		"input":[]
	}`)
	event := readProxyResponsesWebSocketJSON(t, conn)
	errorBody, ok := event["error"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, errorBody["message"], "input[2].image_url")
	assert.Contains(t, errorBody["message"], ";base64")
	assert.NotContains(t, errorBody["message"], "not-base64")
	assert.Equal(t, int32(1), codexCalls.Load())
}

func TestCodexResponsesRouteE2E_RequestLogsShowsUpstreamToken(t *testing.T) {
	f := setup(t)
	credID := f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   "cred-codex-logs",
		Name: "Codex logs seat",
		Info: map[string]any{"email": "logs@example.com", "status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-logs",
		RefreshToken: "refresh-logs",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    "acct_logs",
	})

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_logs","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_logs","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_logs","model":"gpt-5.5","usage":{"input_tokens":13,"output_tokens":2,"total_tokens":15}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	oldBaseURL := cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL
	cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	t.Cleanup(func() {
		cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = oldBaseURL
	})

	oldCallbacks := proxyHandler.Callbacks
	reg := callback.NewRegistry()
	reg.Register(spend.NewTracker(testDB, nil, false))
	proxyHandler.Callbacks = reg
	t.Cleanup(func() {
		proxyHandler.Callbacks = oldCallbacks
	})

	f.SeedModel(SeedModelOpts{
		ModelName: "openai/*",
		Model:     "openai/*",
		Extra: map[string]any{
			"openai_subscription_credential_ids": []string{credID},
			"openai_subscription_transport":      "chatgpt_codex_backend",
		},
	})
	require.NoError(t, proxyHandler.RefreshRuntimeModels(context.Background()))

	resp := postProxyResponses(t, `{
		"model":"openai/gpt-5.5",
		"input":"say OK",
		"stream":true
	}`)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	require.Eventually(t, func() bool {
		var got string
		err := testPool.QueryRow(context.Background(), `
			SELECT upstream_token_key
			FROM "SpendLogs"
			WHERE model = 'openai/gpt-5.5'
			ORDER BY starttime DESC
			LIMIT 1
		`).Scan(&got)
		return err == nil && got == credID
	}, 2*time.Second, 50*time.Millisecond)

	f.NavigateToLogs()
	logsText := f.Text("#logs-table")
	assert.Contains(t, logsText, "openai/gpt-5.5")
	assert.Contains(t, logsText, credID)
}

func TestCodexSubscriptionRoute_NonStreamingContractIsDeterministic(t *testing.T) {
	f := setup(t)
	const upstreamErrorBody = `{"error":{"message":"Codex non-stream request rejected","type":"api_error"}}`

	var codexCalls atomic.Int32
	var capturedAuth string
	var capturedAccountID string
	var capturedBody map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-api/wham/usage" {
			writeCodexUsageSnapshot(t, w, "nonstream@example.com")
			return
		}
		require.Equal(t, "/backend-api/codex/responses", r.URL.Path)
		require.Equal(t, int32(1), codexCalls.Add(1), "native non-stream request must make one Codex backend call")
		capturedAuth = r.Header.Get("Authorization")
		capturedAccountID = r.Header.Get("ChatGPT-Account-Id")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(upstreamErrorBody))
		require.NoError(t, err)
	}))
	defer backend.Close()

	platformCalls := configureCodexResponsesE2ERoute(t, f, codexResponsesE2EConfig{
		CredentialID: "cred-codex-nonstream",
		Email:        "nonstream@example.com",
		AccessToken:  "access-nonstream",
		AccountID:    "acct_nonstream",
		BackendURL:   backend.URL,
	})

	resp := postProxyResponses(t, `{
		"model":"openai/gpt-5.5",
		"instructions":"You are concise.",
		"input":"say OK",
		"temperature":0.2,
		"top_p":0.8,
		"max_tokens":77,
		"max_output_tokens":123,
		"stream":false
	}`)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode, string(body))
	require.Equal(t, upstreamErrorBody, string(body))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, int32(1), codexCalls.Load())
	assert.Equal(t, int32(0), platformCalls.Load())
	assert.Equal(t, "Bearer access-nonstream", capturedAuth)
	assert.Equal(t, "acct_nonstream", capturedAccountID)
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	assert.Equal(t, false, capturedBody["stream"])
	assert.Equal(t, "You are concise.", capturedBody["instructions"])
	assert.Equal(t, 0.2, capturedBody["temperature"])
	assert.Equal(t, 0.8, capturedBody["top_p"])
	assert.Equal(t, float64(77), capturedBody["max_tokens"])
	assert.Equal(t, float64(123), capturedBody["max_output_tokens"])
	input, ok := capturedBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	part, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "say OK", part["text"])
	assert.NotContains(t, string(body), "access-nonstream")
	assert.NotContains(t, string(body), "refresh-cred-codex-nonstream")
}

func writeCodexUsageSnapshot(t *testing.T, w http.ResponseWriter, email string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"email":     email,
		"plan_type": "pro",
		"rate_limit": map[string]any{
			"primary_window": map[string]any{
				"used_percent": 0.1,
				"reset_at":     time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
			},
			"secondary_window": map[string]any{
				"used_percent": 0.2,
				"reset_at":     time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
			},
		},
	}))
}

func postProxyChatCompletion(t *testing.T, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/v1/chat/completions", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+masterKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func postProxyResponses(t *testing.T, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/v1/responses", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+masterKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

type codexResponsesE2EConfig struct {
	CredentialID string
	Email        string
	AccessToken  string
	AccountID    string
	BackendURL   string
}

func configureCodexResponsesE2ERoute(t *testing.T, f *Fixture, opts codexResponsesE2EConfig) *atomic.Int32 {
	t.Helper()
	credID := f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   opts.CredentialID,
		Name: opts.CredentialID,
		Info: map[string]any{"email": opts.Email, "status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  opts.AccessToken,
		RefreshToken: "refresh-" + opts.CredentialID,
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    opts.AccountID,
	})

	platformCalls := &atomic.Int32{}
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCalls.Add(1)
		http.Error(w, "responses route must not call Platform path "+r.URL.Path, http.StatusInternalServerError)
	}))
	t.Cleanup(platform.Close)

	oldBaseURL := cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL
	cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = opts.BackendURL
	t.Cleanup(func() {
		cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = oldBaseURL
	})
	f.SeedModel(SeedModelOpts{
		ModelName: "openai/*",
		Model:     "openai/*",
		APIBase:   platform.URL + "/v1",
		Extra: map[string]any{
			"openai_subscription_credential_ids": []string{credID},
			"openai_subscription_transport":      "chatgpt_codex_backend",
		},
	})
	require.NoError(t, proxyHandler.RefreshRuntimeModels(context.Background()))
	return platformCalls
}

func e2eObservedOutputImageURLPayload(imageURL string) string {
	input := make([]any, 64)
	for i := range input {
		input[i] = map[string]any{
			"type":    "message",
			"role":    "user",
			"content": []any{map[string]any{"type": "input_text", "text": "context"}},
		}
	}
	input[63] = map[string]any{
		"type": "message",
		"role": "assistant",
		"output": []any{
			map[string]any{"type": "output_text", "text": "created"},
			map[string]any{"type": "input_image", "image_url": imageURL},
		},
	}
	payload := map[string]any{
		"model":  "openai/gpt-5.5",
		"input":  input,
		"stream": true,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func e2eCapturedResponseOutputImageItem(t *testing.T, body map[string]any, inputIndex, outputIndex int) map[string]any {
	t.Helper()
	input, ok := body["input"].([]any)
	require.True(t, ok)
	require.Greater(t, len(input), inputIndex)
	message, ok := input[inputIndex].(map[string]any)
	require.True(t, ok)
	output, ok := message["output"].([]any)
	require.True(t, ok)
	require.Greater(t, len(output), outputIndex)
	image, ok := output[outputIndex].(map[string]any)
	require.True(t, ok)
	return image
}

func dialProxyResponsesWebSocket(t *testing.T) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/v1/responses"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization":       []string{"Bearer " + masterKey},
			"OpenAI-Beta":         []string{"responses_websockets=2026-02-06"},
			"x-client-request-id": []string{"req_ws_e2e"},
			"session_id":          []string{"session_ws_e2e"},
			"thread_id":           []string{"thread_ws_e2e"},
		},
	})
	require.NoError(t, err)
	return conn
}

func writeProxyResponsesWebSocketJSON(t *testing.T, conn *websocket.Conn, body string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte(body)))
}

func readProxyResponsesWebSocketJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	messageType, data, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, websocket.MessageText, messageType)
	var event map[string]any
	require.NoError(t, json.Unmarshal(data, &event))
	return event
}

func jsonQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func e2eResponseIDFromEvent(t *testing.T, event map[string]any) string {
	t.Helper()
	response, ok := event["response"].(map[string]any)
	require.True(t, ok)
	responseID, ok := response["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, responseID)
	return responseID
}
