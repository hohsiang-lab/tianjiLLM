package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type responsesLogCapture struct {
	successes chan callback.LogData
	failures  chan callback.LogData
}

func newResponsesLogCapture() *responsesLogCapture {
	return &responsesLogCapture{
		successes: make(chan callback.LogData, 4),
		failures:  make(chan callback.LogData, 4),
	}
}

func (c *responsesLogCapture) LogSuccess(data callback.LogData) {
	c.successes <- data
}

func (c *responsesLogCapture) LogFailure(data callback.LogData) {
	c.failures <- data
}

func (c *responsesLogCapture) waitSuccess(t *testing.T) callback.LogData {
	t.Helper()
	select {
	case data := <-c.successes:
		return data
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for responses LogSuccess")
		return callback.LogData{}
	}
}

func (c *responsesLogCapture) waitFailure(t *testing.T) callback.LogData {
	t.Helper()
	select {
	case data := <-c.failures:
		return data
	case <-c.successes:
		t.Fatal("received responses LogSuccess while waiting for LogFailure")
		return callback.LogData{}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for responses LogFailure")
		return callback.LogData{}
	}
}

func (c *responsesLogCapture) assertNoLog(t *testing.T) {
	t.Helper()
	select {
	case data := <-c.successes:
		t.Fatalf("unexpected responses LogSuccess: %+v", data)
	case data := <-c.failures:
		t.Fatalf("unexpected responses LogFailure: %+v", data)
	case <-time.After(100 * time.Millisecond):
	}
}

func (c *responsesLogCapture) assertNoSuccess(t *testing.T) {
	t.Helper()
	select {
	case data := <-c.successes:
		t.Fatalf("unexpected responses LogSuccess: %+v", data)
	default:
	}
}

func newCodexResponsesTestHandler(t *testing.T, backendURL string, accountID string) *Handlers {
	t.Helper()
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-secret", AccountID: accountID},
	})
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backendURL
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/*",
			OpenAISubscriptionCredentialIDs: []string{"cred-a"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	return h
}

func capturedResponseImagePart(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	input, ok := body["input"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, input)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	for _, part := range content {
		item, ok := part.(map[string]any)
		require.True(t, ok)
		if item["type"] == "input_image" {
			return item
		}
	}
	t.Fatalf("input_image content part missing from %#v", content)
	return nil
}

func capturedResponseOutputImageItem(t *testing.T, body map[string]any, inputIndex, outputIndex int) map[string]any {
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

func observedOutputImageURLPayload(imageURL string) string {
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

func TestLogResponsesUpstreamHTTPFailure_RecordsRedactedBody(t *testing.T) {
	inserted := make(chan db.InsertErrorLogParams, 1)
	m := newMockStore()
	m.insertErrorLogFn = func(_ context.Context, arg db.InsertErrorLogParams) error {
		inserted <- arg
		return nil
	}

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := mockHandlers(m)
	h.Callbacks = reg

	body := []byte(`{"error":{"message":"bad Authorization: Bearer access-secret","code":"bad_request"}}`)
	h.logResponsesUpstreamHTTPFailure(context.Background(), "openai/gpt-5.5", "high", time.Now(), http.StatusBadRequest, body)

	data := cap.waitFailure(t)
	require.Error(t, data.Error)
	assert.Contains(t, data.Error.Error(), "upstream error: status 400")
	assert.Contains(t, data.Error.Error(), "body=")
	assert.Contains(t, data.Error.Error(), "[REDACTED]")
	assert.NotContains(t, data.Error.Error(), "access-secret")

	select {
	case log := <-inserted:
		assert.Equal(t, int32(http.StatusInternalServerError), log.StatusCode)
		assert.Equal(t, "provider_error", log.ErrorType)
		assert.Equal(t, "high", log.ReasoningEffort)
		assert.Contains(t, log.ErrorMessage, "upstream error: status 400")
		assert.Contains(t, log.ErrorMessage, "body=")
		assert.Contains(t, log.ErrorMessage, "[REDACTED]")
		assert.NotContains(t, log.ErrorMessage, "access-secret")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for InsertErrorLog")
	}
}

func TestResponsesUpstreamHTTPFailureMessage_TruncatesBody(t *testing.T) {
	body := []byte(strings.Repeat("x", responsesUpstreamErrorBodyLogLimit+1))

	got := responsesUpstreamHTTPFailureMessage(http.StatusBadRequest, body)

	assert.Contains(t, got, "upstream error: status 400 body=")
	assert.Contains(t, got, "...[truncated]")
	assert.LessOrEqual(t, len([]rune(got)), len("upstream error: status 400 body=")+responsesUpstreamErrorBodyLogLimit+len("...[truncated]"))
}

func TestCreateResponse_RoutesSubscriptionWildcardResponsesToCodexBackend(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var capturedAccountID string
	var capturedBody map[string]any
	var capturedClientHeaders http.Header

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedAccountID = r.Header.Get("ChatGPT-Account-Id")
		capturedClientHeaders = make(http.Header)
		for _, name := range []string{"Version", "OpenAI-Beta", "originator", "x-codex-turn-state", "x-codex-turn-metadata", "x-codex-routing-hint", "Accept", "Content-Type"} {
			if values := r.Header.Values(name); len(values) > 0 {
				capturedClientHeaders[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
			}
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","output":[{"content":[{"type":"output_text","text":"OK"}]}]}`))
	}))
	defer backend.Close()

	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("responses wildcard must not call Platform path %s", r.URL.Path)
	}))
	defer platform.Close()

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-secret", AccountID: "acct_responses"},
	})
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	platformBase := platform.URL + "/v1"
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendClientVersion = "0.144.1"
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/*",
			APIBase:                         &platformBase,
			OpenAISubscriptionCredentialIDs: []string{"cred-a"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":true,"client_metadata":{"x-codex-turn-metadata":"{\"source\":\"body\"}"}}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer client-key")
	req.Header.Set("ChatGPT-Account-Id", "client-account")
	req.Header.Set("Version", "0.156.1-client")
	req.Header.Set("OpenAI-Beta", "client-beta")
	req.Header.Set("originator", "hermes-agent")
	req.Header.Set("x-codex-turn-state", "turn-state-client")
	req.Header.Set("x-codex-turn-metadata", `{"source":"client-header"}`)
	req.Header.Set("x-codex-routing-hint", "model=client-model;tier=client-tier")
	req.Header.Set("Accept", "text/event-stream, application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/backend-api/codex/responses", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedAuth)
	assert.Equal(t, "acct_responses", capturedAccountID)
	assert.Equal(t, "0.156.1-client", capturedClientHeaders.Get("Version"))
	assert.Equal(t, "client-beta", capturedClientHeaders.Get("OpenAI-Beta"))
	assert.Equal(t, "hermes-agent", capturedClientHeaders.Get("originator"))
	assert.Equal(t, "turn-state-client", capturedClientHeaders.Get("x-codex-turn-state"))
	assert.Equal(t, `{"source":"client-header"}`, capturedClientHeaders.Get("x-codex-turn-metadata"))
	assert.Equal(t, "model=client-model;tier=client-tier", capturedClientHeaders.Get("x-codex-routing-hint"))
	assert.Equal(t, "text/event-stream, application/json", capturedClientHeaders.Get("Accept"))
	assert.Equal(t, "application/json; charset=utf-8", capturedClientHeaders.Get("Content-Type"))
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	input, ok := capturedBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", message["type"])
	assert.Equal(t, "user", message["role"])
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	part, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_text", part["type"])
	assert.Equal(t, "say OK", part["text"])
	assert.Equal(t, true, capturedBody["stream"])
	assert.Equal(t, false, capturedBody["store"])
	clientMetadata, ok := capturedBody["client_metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, `{"source":"body"}`, clientMetadata["x-codex-turn-metadata"])
	assert.NotContains(t, capturedBody, "instructions")
	assert.NotContains(t, capturedBody, "type")
}

func TestCompactResponse_RoutesSubscriptionWildcardResponsesToCodexBackend(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var capturedAccountID string
	var capturedAccept string
	var capturedOriginator string
	var capturedVersion string
	var capturedBeta string
	var capturedBody map[string]any

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedAccountID = r.Header.Get("ChatGPT-Account-Id")
		capturedAccept = r.Header.Get("Accept")
		capturedVersion = r.Header.Get("Version")
		capturedBeta = r.Header.Get("OpenAI-Beta")
		capturedOriginator = r.Header.Get("originator")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_compact_123","object":"response.compaction","model":"gpt-5.5","output":[{"type":"message","role":"user","content":[{"type":"input_text","text":"compacted"}]}]}`))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendOriginator = "configured-originator"
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendClientVersion = "0.144.1"
	req := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CompactResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/backend-api/codex/responses/compact", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedAuth)
	assert.Equal(t, "acct_responses", capturedAccountID)
	assert.Empty(t, capturedOriginator)
	assert.Empty(t, capturedAccept)
	assert.Empty(t, capturedVersion)
	assert.Empty(t, capturedBeta)
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	assert.NotContains(t, capturedBody, "store")
	assert.NotContains(t, capturedBody, "stream")
	assert.Contains(t, w.Body.String(), `"object":"response.compaction"`)
	assert.Contains(t, w.Body.String(), `"output"`)
	assert.Contains(t, w.Body.String(), "compacted")
}

func TestResponsesHTTPAndCompact_PassSessionIdentityToRouting(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	var capturedAuths []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuths = append(capturedAuths, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/compact") {
			_, _ = w.Write([]byte(`{"id":"resp_compact_123","object":"response.compaction","model":"gpt-5.5","output":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","output":[{"content":[{"type":"output_text","text":"OK"}]}]}`))
	}))
	defer backend.Close()

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	for _, credentialID := range []string{"cred-a", "cred-b"} {
		seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), credentialID, testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	}
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/*",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	sessionID := codexSessionForCredential(t, "cred-b", []string{"cred-a", "cred-b"})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"openai/gpt-5.5",
		"input":"say OK",
		"client_metadata":{"session_id":"`+sessionID+`"}
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateResponse(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	compactReq := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{
		"model":"openai/gpt-5.5",
		"input":"compact",
		"client_metadata":{"x-codex-turn-metadata":"{\"session_id\":\"`+sessionID+`\"}"}
	}`))
	compactReq.Header.Set("Content-Type", "application/json")
	compactW := httptest.NewRecorder()
	h.CompactResponse(compactW, compactReq)
	require.Equal(t, http.StatusOK, compactW.Code, compactW.Body.String())

	require.Equal(t, []string{"Bearer access-b", "Bearer access-b"}, capturedAuths)
}

func TestCreateResponse_NormalizesReferenceImageURLForCodexBackend(t *testing.T) {
	var capturedBody map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","output":[{"content":[{"type":"output_text","text":"OK"}]}]}`))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Callbacks = reg
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"openai/gpt-image-2",
		"input":[{
			"role":"user",
			"content":[
				{"type":"input_text","text":"Keep the subject, change the background"},
				{"type":"input_image","image_url":"data:image/png;base64,cmVmZXJlbmNl","detail":"auto"}
			]
		}],
		"tools":[{"type":"image_generation","model":"gpt-image-2"}],
		"tool_choice":{"type":"image_generation"},
		"stream":true,
		"store":false
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	image := capturedResponseImagePart(t, capturedBody)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "data:image/png;base64,cmVmZXJlbmNl", image["image_url"])
	assert.Equal(t, "auto", image["detail"])
	body, err := json.Marshal(capturedBody)
	require.NoError(t, err)
	assert.NotContains(t, string(body), `"image_url":{"url":`)

	logData := cap.waitSuccess(t)
	requestPayload, ok := logData.RequestPayload.(map[string]any)
	require.True(t, ok)
	payloadBytes, err := json.Marshal(requestPayload)
	require.NoError(t, err)
	assert.NotContains(t, string(payloadBytes), "data:image/png;base64")
	assert.NotContains(t, string(payloadBytes), "cmVmZXJlbmNl")
	assert.Contains(t, string(payloadBytes), `"redacted":true`)
	assert.Contains(t, string(payloadBytes), `"sha256"`)
}

func TestCreateResponse_NormalizesLegacyInputImageURLObjectForCodexBackend(t *testing.T) {
	var capturedBody map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","output":[{"content":[{"type":"output_text","text":"OK"}]}]}`))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"openai/gpt-image-2",
		"input":[{
			"role":"user",
			"content":[
				{"type":"input_text","text":"Keep the subject, change the background"},
				{"type":"input_image","image_url":{"url":"https://example.test/reference.png","detail":"high"}}
			]
		}],
		"tools":[{"type":"image_generation","model":"gpt-image-2"}],
		"tool_choice":{"type":"image_generation"},
		"stream":true,
		"store":false
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	image := capturedResponseImagePart(t, capturedBody)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "https://example.test/reference.png", image["image_url"])
	assert.Equal(t, "high", image["detail"])
	body, err := json.Marshal(capturedBody)
	require.NoError(t, err)
	assert.NotContains(t, string(body), `"image_url":{"url":`)
}

func TestCreateResponse_RejectsMalformedReferenceImageBeforeCodexBackend(t *testing.T) {
	calls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"openai/gpt-image-2",
		"input":[{
			"role":"user",
			"content":[{"type":"input_image","image_url":{"detail":"auto"}}]
		}],
		"stream":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "image content part url")
	assert.NotContains(t, w.Body.String(), "auto")
	assert.Zero(t, calls)
}

func TestCreateResponse_RejectsMalformedOutputImageURLBeforeCodexBackend(t *testing.T) {
	calls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(observedOutputImageURLPayload("data:image/png,not-base64")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "input[63].output[1].image_url")
	assert.Contains(t, w.Body.String(), ";base64")
	assert.NotContains(t, w.Body.String(), "not-base64")
	assert.Zero(t, calls)
}

func TestCreateResponse_ForwardsValidOutputImageDataURLToCodexBackend(t *testing.T) {
	var capturedBody map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"id\":\"resp_123\",\"object\":\"response\",\"model\":\"gpt-5.5\",\"output\":[{\"content\":[{\"type\":\"output_text\",\"text\":\"OK\"}]}]}"))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Callbacks = reg
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(observedOutputImageURLPayload("data:image/png;base64,aW1hZ2U=")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	image := capturedResponseOutputImageItem(t, capturedBody, 63, 1)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "data:image/png;base64,aW1hZ2U=", image["image_url"])

	logData := cap.waitSuccess(t)
	requestPayload, ok := logData.RequestPayload.(map[string]any)
	require.True(t, ok)
	payloadBytes, err := json.Marshal(requestPayload)
	require.NoError(t, err)
	assert.NotContains(t, string(payloadBytes), "data:image/png;base64")
	assert.NotContains(t, string(payloadBytes), "aW1hZ2U=")
	assert.Contains(t, string(payloadBytes), `"redacted":true`)
	assert.Contains(t, string(payloadBytes), `"sha256"`)
}

func TestCreateResponse_CodexStreamModePassThrough(t *testing.T) {
	for _, modelName := range []string{"openai/gpt-5.4-mini", "openai/gpt-5.6-luna", "openai/gpt-5.5"} {
		for _, mode := range []struct {
			name    string
			field   string
			value   any
			present bool
		}{
			{name: "true", field: `,"stream":true`, value: true, present: true},
			{name: "false", field: `,"stream":false`, value: false, present: true},
			{name: "null", field: `,"stream":null`, present: true},
			{name: "omitted"},
		} {
			for _, upstream := range []struct {
				name        string
				contentType string
				body        string
			}{
				{name: "JSON", contentType: "application/json; charset=utf-8", body: "{ \"id\": \"resp_native\", \"status\": \"completed\", \"output\": [], \"future_option\": null }\n"},
				{name: "SSE", contentType: "text/event-stream", body: "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_native\",\"delta\":\"OK\"}\n\ndata:{\"type\":\"response.completed\",\"response\":{\"id\":\"resp_native\",\"status\":\"completed\",\"output\":[]}}\n\ndata: [DONE]\n\n"},
			} {
				t.Run(modelName+"/"+mode.name+"/"+upstream.name, func(t *testing.T) {
					captured := make(chan map[string]any, 1)
					backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var payload map[string]any
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
							t.Errorf("decode upstream request: %v", err)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						captured <- payload
						w.Header().Set("Content-Type", upstream.contentType)
						w.Header().Add("X-Codex-Native", "first")
						w.Header().Add("X-Codex-Native", "second")
						_, _ = io.WriteString(w, upstream.body)
					}))
					defer backend.Close()

					h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
					req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"`+modelName+`","input":"say OK"`+mode.field+`}`))
					req.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()
					h.CreateResponse(w, req)

					require.Equal(t, http.StatusOK, w.Code, w.Body.String())
					select {
					case payload := <-captured:
						value, present := payload["stream"]
						assert.Equal(t, mode.present, present, "stream presence")
						assert.Equal(t, mode.value, value, "stream value")
					default:
						t.Fatal("no upstream request captured")
					}
					assert.Equal(t, upstream.contentType, w.Header().Get("Content-Type"))
					assert.Equal(t, []string{"first", "second"}, w.Header().Values("X-Codex-Native"))
					assert.Equal(t, upstream.body, w.Body.String())
				})
			}
		}
	}
}

func TestCreateResponse_CodexBadRequestPassThroughWithoutRetry(t *testing.T) {
	const body = "{ \"error\": {\"message\":\"stream must be true\",\"type\":\"invalid_request_error\",\"param\":\"stream\"} }\n"
	var calls atomic.Int32
	var capturedModel string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/backend-api/codex/responses", r.URL.Path)
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		capturedModel, _ = payload["model"].(string)
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Codex-Error", "native-400")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, body)
	}))
	defer backend.Close()

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "alias/*",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/*",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	cap := newResponsesLogCapture()
	h.Callbacks = callback.NewRegistry()
	h.Callbacks.Register(cap)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"alias/gpt-5.4-mini","input":"say OK","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateResponse(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "gpt-5.4-mini", capturedModel)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
	assert.Equal(t, "native-400", w.Header().Get("X-Codex-Error"))
	assert.Equal(t, body, w.Body.String())
	assert.Equal(t, int32(1), calls.Load())
	require.Error(t, cap.waitFailure(t).Error)
	cap.assertNoSuccess(t)
}

func TestCreateResponse_ForwardsNonStreamingCodexTerminals(t *testing.T) {
	for _, upstream := range []struct {
		name        string
		contentType string
		body        string
		wantFailure string
		wantSuccess bool
	}{
		{
			name: "failed JSON response", contentType: "application/json",
			body:        `{"id":"resp_failed","status":"failed"}`,
			wantFailure: "status=failed",
		},
		{
			name: "incomplete JSON response", contentType: "application/json",
			body:        `{"id":"resp_incomplete","status":"incomplete"}`,
			wantFailure: "status=incomplete",
		},
		{
			name: "missing terminal", contentType: "text/event-stream",
			body: "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_incomplete\"}}\n\ndata: [DONE]\n\n",
		},
		{
			name: "null completed response", contentType: "text/event-stream",
			body: "data: {\"type\":\"response.completed\",\"response\":null}\n\ndata: [DONE]\n\n",
		},
		{
			name: "empty completed response", contentType: "text/event-stream",
			body: "data: {\"type\":\"response.completed\",\"response\":{}}\n\ndata: [DONE]\n\n",
		},
		{
			name: "multiple completed responses", contentType: "text/event-stream",
			body: strings.Join([]string{
				`data: {"type":"response.completed","response":{"id":"resp_one","status":"completed"}}`,
				`data: {"type":"response.completed","response":{"id":"resp_two","status":"completed"}}`,
				`data: [DONE]`,
			}, "\n\n"),
		},
		{
			name: "error event before completed response", contentType: "text/event-stream",
			wantFailure: "event=error",
			body: strings.Join([]string{
				`data: {"type":"error","error":{"message":"backend failed"}}`,
				`data: {"type":"response.completed","response":{"id":"resp_after_error","status":"completed"}}`,
				`data: [DONE]`,
			}, "\n\n"),
		},
		{
			name: "incomplete event before completed response", contentType: "text/event-stream",
			wantFailure: "status=incomplete",
			body: strings.Join([]string{
				`data: {"type":"response.incomplete","response":{"id":"resp_incomplete","status":"incomplete"}}`,
				`data: {"type":"response.completed","response":{"id":"resp_after_incomplete","status":"completed"}}`,
				`data: [DONE]`,
			}, "\n\n"),
		},
		{
			name: "incomplete event without response", contentType: "text/event-stream",
			body:        "data: {\"type\":\"response.incomplete\"}\n\ndata: [DONE]\n\n",
			wantFailure: "event=response.incomplete",
		},
		{
			name: "nested incomplete status", contentType: "text/event-stream",
			body:        "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_incomplete\",\"status\":\"incomplete\"}}\n\ndata: [DONE]\n\n",
			wantFailure: "status=incomplete",
		},
		{
			name: "empty terminal output after text and tool items", contentType: "text/event-stream",
			wantSuccess: true,
			body: strings.Join([]string{
				`data: {"type":"response.output_text.delta","response_id":"resp_native","item_id":"msg_native","output_index":0,"delta":"OK"}`,
				`data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","call_id":"call_lookup","name":"lookup","arguments":"{}"}}`,
				`data: {"type":"response.completed","response":{"id":"resp_native","status":"completed","output":[]}}`,
				`data: [DONE]`,
			}, "\n\n"),
		},
	} {
		t.Run(upstream.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", upstream.contentType)
				_, _ = io.WriteString(w, upstream.body)
			}))
			defer backend.Close()

			h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
			cap := newResponsesLogCapture()
			h.Callbacks = callback.NewRegistry()
			h.Callbacks.Register(cap)
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.4-mini","input":"say OK"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CreateResponse(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, upstream.contentType, w.Header().Get("Content-Type"))
			assert.Equal(t, upstream.body, w.Body.String())
			if upstream.wantFailure != "" {
				data := cap.waitFailure(t)
				require.ErrorContains(t, data.Error, upstream.wantFailure)
				cap.assertNoSuccess(t)
			} else if upstream.wantSuccess {
				require.NoError(t, cap.waitSuccess(t).Error)
			}
		})
	}
}

func TestCreateResponse_CodexBodyReadFailureIsNotSuccess(t *testing.T) {
	const body = `{"id":"resp_partial","output":[`
	var calls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)+1))
		w.Header().Set("X-Codex-Response", "partial")
		_, _ = io.WriteString(w, body)
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	cap := newResponsesLogCapture()
	h.Callbacks = callback.NewRegistry()
	h.Callbacks.Register(cap)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.4-mini","input":"say OK","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateResponse(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, strconv.Itoa(len(body)+1), w.Header().Get("Content-Length"))
	assert.Equal(t, "partial", w.Header().Get("X-Codex-Response"))
	assert.Equal(t, body, w.Body.String())
	assert.Equal(t, int32(1), calls.Load())
	require.ErrorIs(t, cap.waitFailure(t).Error, io.ErrUnexpectedEOF)
	cap.assertNoSuccess(t)
}

func TestCreateResponse_LogsSuccessfulCodexResponsesRequest(t *testing.T) {
	var capturedBody map[string]any
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","output":[{"content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":11,"output_tokens":3,"total_tokens":14}}`))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Callbacks = reg

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, false, capturedBody["stream"])
	data := cap.waitSuccess(t)
	assert.Equal(t, "openai/gpt-5.5", data.Model)
	assert.Equal(t, "responses", data.CallType)
	assert.Equal(t, 11, data.PromptTokens)
	assert.Equal(t, 3, data.CompletionTokens)
	assert.Equal(t, 14, data.TotalTokens)
	assert.Equal(t, "cred-a", data.UpstreamTokenKey)
	require.NotNil(t, data.OpenAISubscription)
	assert.Equal(t, "cred-a", data.OpenAISubscription.CredentialID)
	assert.Equal(t, "success", data.OpenAISubscription.Status)
}

func TestCreateResponse_AttachesJSONResponseAuditPayload(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","status":"completed","output":[{"content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":11,"output_tokens":3,"total_tokens":14}}`))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Callbacks = reg

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := cap.waitSuccess(t)
	responsePayload, ok := data.ResponsePayload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "responses_response", responsePayload["type"])
	assert.Equal(t, "json", responsePayload["format"])
	response, ok := responsePayload["response"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "resp_123", response["id"])
	assert.Equal(t, "gpt-5.5", response["model"])
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, 1, response["output_len"])
	output, ok := response["output"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, output, 1)
	content, ok := output[0]["content"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	assert.Equal(t, "output_text", content[0]["type"])
	assert.Equal(t, "OK", content[0]["text"])
	usage, ok := response["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(11), usage["input_tokens"])
	assert.Equal(t, float64(3), usage["output_tokens"])
	assert.Equal(t, float64(14), usage["total_tokens"])
}

func TestCreateResponse_AttachesResponsesAuditPayloads(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`event: response.created`,
			`data: {"type":"response.created","response":{"id":"resp_123","model":"gpt-5.5","instructions":"caller instructions","usage":null}}`,
			``,
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"OK","sequence_number":1}`,
			``,
			`event: response.completed`,
			`data: {"type":"response.completed","response":{"id":"resp_123","model":"gpt-5.5","status":"completed","output":[],"usage":{"input_tokens":11,"output_tokens":3,"total_tokens":14}}}`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Callbacks = reg

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","instructions":"caller instructions","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}],"stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := cap.waitSuccess(t)
	requestPayload, ok := data.RequestPayload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "responses_request", requestPayload["type"])
	receivedBody, ok := requestPayload["received_payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5.5", receivedBody["model"])
	assert.Equal(t, "caller instructions", receivedBody["instructions"])
	upstreamBody, ok := requestPayload["upstream_payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gpt-5.5", upstreamBody["model"])
	assert.Equal(t, "caller instructions", upstreamBody["instructions"])
	assert.Equal(t, false, upstreamBody["store"])
	assert.Equal(t, true, upstreamBody["stream"])

	responsePayload, ok := data.ResponsePayload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "responses_response", responsePayload["type"])
	assert.Equal(t, "sse", responsePayload["format"])
	assert.Equal(t, 3, responsePayload["events_len"])
	events, ok := responsePayload["events"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, events, 3)
	completedResponse, ok := events[2]["response"].(map[string]any)
	require.True(t, ok)
	usage, ok := completedResponse["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(11), usage["input_tokens"])
	assert.Equal(t, float64(3), usage["output_tokens"])
	assert.Equal(t, float64(14), usage["total_tokens"])
}

func TestExtractResponsesUsage_ReadsCompletedResponseUsageFields(t *testing.T) {
	body := []byte(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_123","usage":null}}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_123","usage":{"input_tokens":37,"input_tokens_details":{"cached_tokens":5},"output_tokens":15,"output_tokens_details":{"reasoning_tokens":8},"total_tokens":52}}}`,
		``,
	}, "\n"))

	usage := extractResponsesUsage(body)

	assert.Equal(t, 37, usage.PromptTokens)
	assert.Equal(t, 15, usage.CompletionTokens)
	assert.Equal(t, 52, usage.TotalTokens)
	assert.Equal(t, 5, usage.CacheReadInputTokens)
	assert.Equal(t, 0, usage.CacheCreationInputTokens)
}

func TestLogResponsesSuccess_CachedInputTokensUseCacheRate(t *testing.T) {
	pricing.Default().SetCustomPricing("gpt-5.5-cache-test", pricing.ModelInfo{
		InputCostPerToken:         5e-06,
		OutputCostPerToken:        3e-05,
		CacheReadCostPerToken:     5e-07,
		CacheCreationCostPerToken: 0,
	})

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := &Handlers{Callbacks: reg}

	start := time.Date(2026, 5, 19, 18, 52, 15, 0, time.UTC)
	h.logResponsesSuccess(
		context.Background(),
		"gpt-5.5-cache-test",
		"low",
		model.Usage{
			PromptTokens:         211931,
			CompletionTokens:     31,
			TotalTokens:          211962,
			CacheReadInputTokens: 210944,
		},
		start,
		start.Add(4*time.Second),
		4*time.Second,
		nil,
		nil,
		nil,
	)

	data := cap.waitSuccess(t)
	assert.Equal(t, 211931, data.PromptTokens)
	assert.Equal(t, 210944, data.CacheReadInputTokens)
	assert.True(t, data.CacheHit)
	assert.Equal(t, "low", data.ReasoningEffort)

	// OpenAI Responses input_tokens includes cached_tokens. Only uncached
	// input should use the full input rate; cached input uses cache_read rate.
	const want = (211931-210944)*5e-06 + 210944*5e-07 + 31*3e-05
	assert.InDelta(t, want, data.Cost, 1e-9)
}

func TestExtractResponsesUsage_UsesLastTokenBearingSSEEvent(t *testing.T) {
	body := []byte(strings.Join([]string{
		`data: {"type":"response.created","response":{"usage":null}}`,
		`data: {"type":"response.in_progress","response":{"usage":null}}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13}}}`,
		``,
	}, "\n"))

	usage := extractResponsesUsage(body)

	assert.Equal(t, 9, usage.PromptTokens)
	assert.Equal(t, 4, usage.CompletionTokens)
	assert.Equal(t, 13, usage.TotalTokens)
}

func TestExtractResponsesUsage_ReadsLargeCompletedSSEEvent(t *testing.T) {
	largeMetadata := strings.Repeat("x", 70<<10)
	body := []byte(strings.Join([]string{
		`data: {"type":"response.created","response":{"usage":null}}`,
		`data: {"type":"response.completed","response":{"metadata":{"large":"` + largeMetadata + `"},"usage":{"input_tokens":21,"output_tokens":8,"total_tokens":29}}}`,
		``,
	}, "\n"))

	usage := extractResponsesUsage(body)

	assert.Equal(t, 21, usage.PromptTokens)
	assert.Equal(t, 8, usage.CompletionTokens)
	assert.Equal(t, 29, usage.TotalTokens)
}

func TestCreateResponse_LogsFailedCodexResponsesRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream unavailable"}}`))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Callbacks = reg

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
	data := cap.waitFailure(t)
	assert.Equal(t, "openai/gpt-5.5", data.Model)
	assert.Equal(t, "responses", data.CallType)
	require.Error(t, data.Error)
	assert.Contains(t, data.Error.Error(), "status 502")
}

func TestCreateResponse_LogsCodexSSEFailedWithoutSyntheticBackoff(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_capacity","model":"gpt-5.5","status":"in_progress","usage":null}}`,
			``,
			`data: {"type":"response.failed","response":{"id":"resp_capacity","model":"gpt-5.5","status":"failed","error":{"message":"Selected model is at capacity. Please try a different model.","type":"server_error","code":"model_at_capacity"},"usage":null}}`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	h.Callbacks = reg

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "response.failed")
	data := cap.waitFailure(t)
	require.Error(t, data.Error)
	assert.Contains(t, data.Error.Error(), "model_at_capacity")
	assert.Equal(t, "responses", data.CallType)
	assert.Equal(t, "cred-a", data.OpenAISubscription.CredentialID)
	cap.assertNoSuccess(t)

	_, ok := h.openAISubscriptionRateLimitState("cred-a")
	assert.False(t, ok)
}

func TestCreateResponse_FetchesCodexUsageOnRoute(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","output":[{"content":[{"type":"output_text","text":"OK"}]}]}`))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_responses")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	calls := requireCodexUsageCalls(t, fetcher, 1)
	assert.Equal(t, "access-secret", calls[0].AccessToken)
	assert.Equal(t, "acct_responses", calls[0].AccountID)
}

func TestCreateResponse_UsesCachedUsageBeforeAsyncRefresh(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	var capturedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","model":"gpt-5.5","output":[{"content":[{"type":"output_text","text":"OK"}]}]}`))
	}))
	defer backend.Close()

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-a", testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-b", testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available"))
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available")},
		{snapshot: testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available")},
	}}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/*",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Bearer access-a", capturedAuth)
}

func TestCreateResponseWebSocket_RoutesResponseCreateFramesToCodexBackendAndForwardsEvents(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var capturedAccountID string
	var capturedBodies []map[string]any
	var capturedHeaders []http.Header

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedAccountID = r.Header.Get("ChatGPT-Account-Id")
		capturedHeaders = append(capturedHeaders, r.Header.Clone())
		var capturedBody map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		if len(capturedBodies) > 0 {
			if _, ok := capturedBody["previous_response_id"]; ok {
				http.Error(w, `{"detail":"previous_response_id must be expanded by Tianji"}`, http.StatusBadRequest)
				return
			}
			input, _ := capturedBody["input"].([]any)
			if len(input) != 2 {
				http.Error(w, `{"detail":"follow-up input must include cached prior turn"}`, http.StatusBadRequest)
				return
			}
		}
		capturedBodies = append(capturedBodies, capturedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_ws","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/responses"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test-master-key")
	headers.Set("OpenAI-Beta", "responses_websockets=2026-02-06")
	headers.Set("x-client-request-id", "req_ws")
	headers.Set("session_id", "session_ws")
	headers.Set("thread_id", "thread_ws")
	headers.Set("Accept", "application/vnd.example+json")
	headers.Set("originator", "ws-client-originator")
	headers.Set("Version", "ws-client-version")
	headers.Set("x-codex-turn-state", "ws-turn-state")
	headers.Set("x-codex-turn-metadata", `{"source":"ws-header"}`)
	headers.Set("x-codex-routing-hint", "model=ws-model")

	conn, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: headers})
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	err = conn.Write(context.Background(), websocket.MessageText, []byte(`{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}]
	}`))
	require.NoError(t, err)

	var messages []string
	for len(messages) < 3 {
		_, data, readErr := conn.Read(context.Background())
		require.NoError(t, readErr)
		messages = append(messages, string(data))
	}

	err = conn.Write(context.Background(), websocket.MessageText, []byte(`{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":"resp_ws",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say again"}]}]
	}`))
	require.NoError(t, err)

	for len(messages) < 6 {
		_, data, readErr := conn.Read(context.Background())
		require.NoError(t, readErr)
		messages = append(messages, string(data))
	}

	assert.Equal(t, "/backend-api/codex/responses", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedAuth)
	assert.Equal(t, "acct_ws", capturedAccountID)
	require.Len(t, capturedBodies, 2)
	require.Len(t, capturedHeaders, 2)
	for _, captured := range capturedHeaders {
		assert.Equal(t, "application/vnd.example+json", captured.Get("Accept"))
		assert.Equal(t, "responses_websockets=2026-02-06", captured.Get("OpenAI-Beta"))
		assert.Equal(t, "ws-client-originator", captured.Get("originator"))
		assert.Equal(t, "ws-client-version", captured.Get("Version"))
		assert.Equal(t, "ws-turn-state", captured.Get("x-codex-turn-state"))
		assert.Equal(t, `{"source":"ws-header"}`, captured.Get("x-codex-turn-metadata"))
		assert.Equal(t, "model=ws-model", captured.Get("x-codex-routing-hint"))
	}
	assert.Equal(t, "gpt-5.5", capturedBodies[0]["model"])
	assert.Equal(t, true, capturedBodies[0]["stream"])
	assert.Equal(t, false, capturedBodies[0]["store"])
	assert.Equal(t, "gpt-5.5", capturedBodies[1]["model"])
	assert.Equal(t, true, capturedBodies[1]["stream"])
	assert.Equal(t, false, capturedBodies[1]["store"])
	input, ok := capturedBodies[0]["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	secondInput, ok := capturedBodies[1]["input"].([]any)
	require.True(t, ok)
	require.Len(t, secondInput, 2)
	assert.NotContains(t, capturedBodies[0], "type")
	assert.NotContains(t, capturedBodies[1], "type")
	assert.NotContains(t, capturedBodies[1], "previous_response_id")
	assert.Contains(t, messages[0], `"type":"response.created"`)
	assert.Contains(t, messages[1], `"type":"response.output_text.delta"`)
	assert.Contains(t, messages[2], `"type":"response.completed"`)
	assert.Contains(t, messages[3], `"type":"response.created"`)
	assert.Contains(t, messages[4], `"type":"response.output_text.delta"`)
	assert.Contains(t, messages[5], `"type":"response.completed"`)
}

func TestResponsesWebSocket_UnknownModelReturnsModelNotFound(t *testing.T) {
	apiBase := "http://compat.example/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "configured-model",
		TianjiParams: config.TianjiParams{
			Model:   "openaicompat/upstream-model",
			APIBase: &apiBase,
		},
	}}}}
	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/responses"
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, []byte(`{"type":"response.create","model":"unknown-model"}`)))

	_, data, err := conn.Read(context.Background())
	require.NoError(t, err)
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(data, &response))
	assert.Equal(t, "model_not_found", response.Error.Code)
	assert.Equal(t, `model "unknown-model" not found`, response.Error.Message)
}

func TestResponsesWebSocket_PassesCreateFrameSessionIdentityToRouting(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	var capturedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_ws","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	for _, credentialID := range []string{"cred-a", "cred-b"} {
		seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), credentialID, testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	}
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/*",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	sessionID := codexSessionForCredential(t, "cred-b", []string{"cred-a", "cred-b"})

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()
	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"input":"say OK",
		"client_metadata":{"session_id":"`+sessionID+`"}
	}`)
	for i := 0; i < 3; i++ {
		_, _, err := conn.Read(context.Background())
		require.NoError(t, err)
	}

	assert.Equal(t, "Bearer access-b", capturedAuth)
}

func TestCreateResponseWebSocket_FetchesCodexUsageOnRoute(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_ws","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher
	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"stream":true,
		"store":false,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}]
	}`)

	readWebSocketJSON(t, conn)
	readWebSocketJSON(t, conn)
	readWebSocketJSON(t, conn)

	calls := requireCodexUsageCalls(t, fetcher, 1)
	assert.Equal(t, "access-secret", calls[0].AccessToken)
	assert.Equal(t, "acct_ws", calls[0].AccountID)
}

func TestCreateResponseWebSocket_UsesCachedUsageBeforeAsyncRefresh(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	var capturedAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_ws","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-a", testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-b", testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available"))
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available")},
		{snapshot: testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available")},
	}}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/*",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()
	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"stream":true,
		"store":false,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}]
	}`)

	readWebSocketJSON(t, conn)
	readWebSocketJSON(t, conn)
	readWebSocketJSON(t, conn)
	assert.Equal(t, "Bearer access-a", capturedAuth)
}

func TestCreateResponseWebSocket_LogsSuccessfulGenerationButNotPrewarm(t *testing.T) {
	var backendCalls int

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_ws","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_ws","model":"gpt-5.5","usage":{"input_tokens":7,"output_tokens":2,"total_tokens":9}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	h.Callbacks = reg

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"generate":false,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}]
	}`)
	warmupID := responseIDFromCreatedEvent(t, readWebSocketJSON(t, conn))
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, 0, backendCalls)
	cap.assertNoLog(t)

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":`+jsonQuote(warmupID)+`,
		"input":[]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.output_text.delta", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])

	data := cap.waitSuccess(t)
	assert.Equal(t, "openai/gpt-5.5", data.Model)
	assert.Equal(t, "responses", data.CallType)
	assert.Equal(t, 7, data.PromptTokens)
	assert.Equal(t, 2, data.CompletionTokens)
	assert.Equal(t, 9, data.TotalTokens)
	assert.Equal(t, "cred-a", data.UpstreamTokenKey)
	require.NotNil(t, data.OpenAISubscription)
	assert.Equal(t, "cred-a", data.OpenAISubscription.CredentialID)
	assert.Equal(t, "success", data.OpenAISubscription.Status)
	requestPayload, ok := data.RequestPayload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "responses_request", requestPayload["type"])
	receivedPayload, ok := requestPayload["received_payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "openai/gpt-5.5", receivedPayload["model"])
	assert.Equal(t, warmupID, receivedPayload["previous_response_id"])
	upstreamPayload, ok := requestPayload["upstream_payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gpt-5.5", upstreamPayload["model"])
	assert.NotContains(t, upstreamPayload, "previous_response_id")
	upstreamInput, ok := upstreamPayload["input"].([]any)
	require.True(t, ok)
	assert.Len(t, upstreamInput, 1)
	responsePayload, ok := data.ResponsePayload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "responses_response", responsePayload["type"])
	assert.Equal(t, "websocket_sse", responsePayload["format"])
	assert.Equal(t, 3, responsePayload["events_len"])
	assert.Equal(t, 1, backendCalls)
}

func TestCreateResponseWebSocket_LogsFailureWhenUpstreamEndsBeforeCompleted(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_ws","delta":"partial"}`,
			``,
		}, "\n")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	defer backend.Close()

	cap := newResponsesLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	h.Callbacks = reg

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.output_text.delta", readWebSocketJSON(t, conn)["type"])

	data := cap.waitFailure(t)
	assert.Equal(t, "openai/gpt-5.5", data.Model)
	assert.Equal(t, "responses", data.CallType)
	require.Error(t, data.Error)
	assert.Contains(t, data.Error.Error(), "responses websocket stream failed")
	cap.assertNoSuccess(t)
}

func TestCreateResponseWebSocket_CodexPrewarmConsumesGenerateFalseAndExpandsFollowup(t *testing.T) {
	var capturedBodies []map[string]any

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var capturedBody map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		capturedBodies = append(capturedBodies, capturedBody)
		if _, ok := capturedBody["generate"]; ok {
			http.Error(w, `{"detail":"Unsupported parameter: generate"}`, http.StatusBadRequest)
			return
		}
		input, _ := capturedBody["input"].([]any)
		if len(input) == 0 {
			http.Error(w, `{"detail":"input is required"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_generated","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_text.delta","response_id":"resp_generated","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_generated","model":"gpt-5.5"}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"generate":false,
		"tools":[],
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}]
	}`)

	warmupCreated := readWebSocketJSON(t, conn)
	assert.Equal(t, "response.created", warmupCreated["type"])
	warmupID := responseIDFromCreatedEvent(t, warmupCreated)
	warmupCompleted := readWebSocketJSON(t, conn)
	assert.Equal(t, "response.completed", warmupCompleted["type"])

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":`+jsonQuote(warmupID)+`,
		"input":[]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.output_text.delta", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])

	require.Len(t, capturedBodies, 1)
	assert.NotContains(t, capturedBodies[0], "generate")
	assert.NotContains(t, capturedBodies[0], "previous_response_id")
	input, ok := capturedBodies[0]["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	assert.Equal(t, true, capturedBodies[0]["stream"])
	assert.Equal(t, false, capturedBodies[0]["store"])
}

func TestCreateResponseWebSocket_CodexPrewarmMergesIncrementalFollowupInput(t *testing.T) {
	var capturedBody map[string]any

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_generated","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_generated","model":"gpt-5.5"}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"generate":false,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"initial"}]}]
	}`)
	warmupID := responseIDFromCreatedEvent(t, readWebSocketJSON(t, conn))
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":`+jsonQuote(warmupID)+`,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"incremental"}]}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])

	input, ok := capturedBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)
	assert.NotContains(t, capturedBody, "previous_response_id")
}

func TestCreateResponseWebSocket_PreviousResponseIncludesCompletedOutputBeforeToolOutput(t *testing.T) {
	var capturedBodies []map[string]any

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var capturedBody map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		capturedBodies = append(capturedBodies, capturedBody)
		if len(capturedBodies) == 2 {
			input, ok := capturedBody["input"].([]any)
			require.True(t, ok)
			require.Len(t, input, 3)
			toolCall, ok := input[1].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "function_call", toolCall["type"])
			assert.Equal(t, "call_test", toolCall["call_id"])
			toolOutput, ok := input[2].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "function_call_output", toolOutput["type"])
			assert.Equal(t, "call_test", toolOutput["call_id"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_tool","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_tool","model":"gpt-5.5","output":[{"type":"function_call","name":"shell","call_id":"call_test","arguments":"{}"}]}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"use a tool"}]}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	firstCompleted := readWebSocketJSON(t, conn)
	assert.Equal(t, "response.completed", firstCompleted["type"])
	responseID := responseIDFromCreatedEvent(t, firstCompleted)

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":`+jsonQuote(responseID)+`,
		"input":[{"type":"function_call_output","call_id":"call_test","output":"done"}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])
	require.Len(t, capturedBodies, 2)
	assert.NotContains(t, capturedBodies[1], "previous_response_id")
}

func TestCreateResponseWebSocket_FullReplayStripsPreviousResponseItemIDs(t *testing.T) {
	var capturedBodies []map[string]any

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var capturedBody map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		capturedBodies = append(capturedBodies, capturedBody)

		responseID := fmt.Sprintf("resp_reasoning_%d", len(capturedBodies))
		completed := fmt.Sprintf(
			`data: {"type":"response.completed","response":{"id":%s,"model":"gpt-5.6-terra","output":[]}}`,
			jsonQuote(responseID),
		)
		if len(capturedBodies) == 1 {
			completed = fmt.Sprintf(
				`data: {"type":"response.completed","response":{"id":%s,"model":"gpt-5.6-terra","output":[{"type":"reasoning","id":"rs_test","summary":[],"encrypted_content":"encrypted-reasoning"}]}}`,
				jsonQuote(responseID),
			)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			fmt.Sprintf(
				`data: {"type":"response.created","response":{"id":%s,"model":"gpt-5.6-terra"}}`,
				jsonQuote(responseID),
			),
			``,
			completed,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.6-terra",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"first"}]}]
	}`)

	responseID := responseIDFromCreatedEvent(t, readWebSocketJSON(t, conn))
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.6-terra",
		"previous_response_id":`+jsonQuote(responseID)+`,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"second"}]}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])

	require.Len(t, capturedBodies, 2)
	assert.Equal(t, false, capturedBodies[1]["store"])
	assert.NotContains(t, capturedBodies[1], "previous_response_id")
	secondInput, ok := capturedBodies[1]["input"].([]any)
	require.True(t, ok)
	require.Len(t, secondInput, 3)
	reasoning, ok := secondInput[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "reasoning", reasoning["type"])
	assert.NotContains(t, reasoning, "id")
	assert.Equal(t, "encrypted-reasoning", reasoning["encrypted_content"])
	assert.Contains(t, reasoning, "summary")
}

func TestCreateResponseWebSocket_RejectsMalformedPreviousOutputImageURLBeforeCodexBackend(t *testing.T) {
	backendCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalls++
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

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"make image"}]}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	firstCompleted := readWebSocketJSON(t, conn)
	assert.Equal(t, "response.completed", firstCompleted["type"])
	responseID := responseIDFromCreatedEvent(t, firstCompleted)

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":`+jsonQuote(responseID)+`,
		"input":[]
	}`)

	event := readWebSocketJSON(t, conn)
	errorBody, ok := event["error"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, errorBody["message"], "input[2].image_url")
	assert.Contains(t, errorBody["message"], ";base64")
	assert.NotContains(t, errorBody["message"], "not-base64")
	assert.Equal(t, 1, backendCalls)
}

func TestCreateResponseWebSocket_PreviousResponseUsesOutputItemDoneWhenCompletedOutputEmpty(t *testing.T) {
	var capturedBodies []map[string]any

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var capturedBody map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		capturedBodies = append(capturedBodies, capturedBody)
		if len(capturedBodies) == 2 {
			input, ok := capturedBody["input"].([]any)
			require.True(t, ok)
			require.Len(t, input, 3)
			toolCall, ok := input[1].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "custom_tool_call", toolCall["type"])
			assert.Equal(t, "call_custom", toolCall["call_id"])
			toolOutput, ok := input[2].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "custom_tool_call_output", toolOutput["type"])
			assert.Equal(t, "call_custom", toolOutput["call_id"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_custom_tool","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.output_item.done","item":{"type":"custom_tool_call","name":"shell","call_id":"call_custom","input":"pwd"}}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_custom_tool","model":"gpt-5.5","output":[]}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"use a custom tool"}]}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.output_item.done", readWebSocketJSON(t, conn)["type"])
	firstCompleted := readWebSocketJSON(t, conn)
	assert.Equal(t, "response.completed", firstCompleted["type"])
	responseID := responseIDFromCreatedEvent(t, firstCompleted)

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":`+jsonQuote(responseID)+`,
		"input":[{"type":"custom_tool_call_output","call_id":"call_custom","output":"done"}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.output_item.done", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])
	require.Len(t, capturedBodies, 2)
	assert.NotContains(t, capturedBodies[1], "previous_response_id")
}

func TestCreateResponseWebSocket_AcceptsLargeCodexFrameOverDefaultReadLimit(t *testing.T) {
	var capturedBody map[string]any

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_large","model":"gpt-5.5"}}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_large","model":"gpt-5.5"}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	largeInstructions := strings.Repeat("Use the Codex tool contract exactly. ", 1200)
	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"instructions":`+jsonQuote(largeInstructions)+`,
		"tools":[],
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"say OK"}]}]
	}`)

	assert.Equal(t, "response.created", readWebSocketJSON(t, conn)["type"])
	assert.Equal(t, "response.completed", readWebSocketJSON(t, conn)["type"])
	require.NotNil(t, capturedBody)
	assert.Equal(t, largeInstructions, capturedBody["instructions"])
}

func TestCreateResponseWebSocket_RejectsFramesAboveBoundedReadLimit(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("frame above bounded read limit must not call backend")
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	tooLargeInstructions := strings.Repeat("x", (4<<20)+1)
	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"instructions":`+jsonQuote(tooLargeInstructions)+`,
		"input":[]
	}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read limited")
	assert.NotContains(t, err.Error(), tooLargeInstructions[:128])
}

func TestCreateResponseWebSocket_MissingCachedPreviousResponseIDReturnsRedactedError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("missing cached previous_response_id must not call backend")
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":"resp_unknown",
		"input":[]
	}`)

	event := readWebSocketJSON(t, conn)
	errorBody, ok := event["error"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, errorBody["message"], "previous response state not found")
	assert.Equal(t, "previous_response_not_found", errorBody["code"])
	assert.Equal(t, "previous_response_id", errorBody["param"])
	assert.NotContains(t, errorBody["message"], "access-secret")
	assert.NotContains(t, errorBody["message"], "Bearer")
	assert.Empty(t, fetcher.calls)
}

func TestCreateResponseWebSocket_MissingCachedPreviousResponseIDWithInputReturnsRedactedError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("uncached previous_response_id with input must not call backend")
	}))
	defer backend.Close()

	h := newCodexResponsesTestHandler(t, backend.URL, "acct_ws")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher

	server := httptest.NewServer(http.HandlerFunc(h.CreateResponse))
	defer server.Close()

	conn := dialResponsesWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	writeWebSocketJSON(t, conn, `{
		"type":"response.create",
		"model":"openai/gpt-5.5",
		"previous_response_id":"resp_unknown",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"still unknown"}]}]
	}`)

	event := readWebSocketJSON(t, conn)
	errorBody, ok := event["error"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, errorBody["message"], "previous response state not found")
	assert.Equal(t, "previous_response_not_found", errorBody["code"])
	assert.Equal(t, "previous_response_id", errorBody["param"])
	assert.Empty(t, fetcher.calls)
}

func TestCreateResponseWebSocket_RejectsPlainGET(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body, err := io.ReadAll(w.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "WebSocket upgrade request")
}

func dialResponsesWebSocket(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/v1/responses"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test-master-key")
	headers.Set("OpenAI-Beta", "responses_websockets=2026-02-06")
	headers.Set("x-client-request-id", "req_ws")
	headers.Set("session_id", "session_ws")
	headers.Set("thread_id", "thread_ws")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: headers})
	require.NoError(t, err)
	return conn
}

func writeWebSocketJSON(t *testing.T, conn *websocket.Conn, payload string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte(payload)))
}

func readWebSocketJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	require.NoError(t, err)
	var event map[string]any
	require.NoError(t, json.Unmarshal(data, &event), string(data))
	return event
}

func responseIDFromCreatedEvent(t *testing.T, event map[string]any) string {
	t.Helper()
	response, ok := event["response"].(map[string]any)
	require.True(t, ok)
	id, ok := response["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, id)
	return id
}

func jsonQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func TestDecodeResponsesRequestBodyRejectsUnsupportedContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hello"}`))
	req.Header.Set("Content-Type", "text/plain")

	_, _, err := decodeResponsesRequestBody(req)
	require.ErrorContains(t, err, "unsupported Content-Type")
}

func TestDecodeResponsesRequestBodyRejectsMissingContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hello"}`))

	_, _, err := decodeResponsesRequestBody(req)
	require.ErrorContains(t, err, "Content-Type is required")
}

func TestDecodeResponsesRequestBodyRejectsMultipleContentTypeHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hello"}`))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Content-Type", "text/plain")

	_, _, err := decodeResponsesRequestBody(req)
	require.ErrorContains(t, err, "multiple Content-Type values")
}
