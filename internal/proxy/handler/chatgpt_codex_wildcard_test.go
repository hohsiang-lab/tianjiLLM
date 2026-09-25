package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatGPTCodexBackendWildcard_NeverSendsSubscriptionBearerToPlatformChatCompletions(t *testing.T) {
	var backendCalls atomic.Int32
	var platformCalls atomic.Int32
	var leakedBearer atomic.Bool

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCalls.Add(1)
		if r.Header.Get("Authorization") == "Bearer access-secret" {
			leakedBearer.Store(true)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer platform.Close()

	h := newOpenAIWildcardCodexHarness(t, backend.URL, platform.URL+"/v1")
	w := httptest.NewRecorder()
	h.ChatCompletion(w, newWildcardChatRequest(`openai/gpt-5.5`))

	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "not_supported", response.Error.Type)
	assert.Zero(t, backendCalls.Load())
	assert.Zero(t, platformCalls.Load())
	assert.False(t, leakedBearer.Load())
	assert.NotContains(t, w.Body.String(), "access-secret")
}

func TestChatGPTCodexImageGenerationRejectsUnsupportedContentType(t *testing.T) {
	var backendCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer backend.Close()

	h := newImageCodexHarness(t, backend.URL)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, backendCalls.Load())
}

func TestChatGPTCodexImageEditRejectsMultipleContentTypeHeaders(t *testing.T) {
	var backendCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer backend.Close()

	h := newImageCodexHarness(t, backend.URL)
	body, contentType := imageEditMultipartBody(t, map[string]string{
		"model":  "gpt-image-2",
		"prompt": "edit the image",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImagesEdit(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, backendCalls.Load())
}

func TestChatGPTCodexBackendImageGeneration_RoutesToCodexResponsesTool(t *testing.T) {
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]any

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"Y29kZXgtaW1hZ2U=","revised_prompt":"revised image prompt"}}`,
				``,
				`data: {"type":"response.completed","response":{"output":[],"usage":{"input_tokens":1674,"output_tokens":66,"total_tokens":1740}}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))),
			Request: r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newImageCodexHarness(t, "https://chatgpt.com")
	pricing.Default().SetCustomPricing("gpt-image-2", pricing.ModelInfo{
		InputCostPerToken:  0.000005,
		OutputCostPerToken: 0.00003,
	})
	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h.Callbacks = reg
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","n":1,"size":"1024x1024","quality":"low","output_format":"jpeg","background":"opaque","output_compression":60,"user":"end-user-42"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer client-key")
	req.Header.Set("ChatGPT-Account-Id", "client-account")
	req.Header.Set("Accept", "application/vnd.example+json")
	req.Header.Set("OpenAI-Beta", "image-beta")
	req.Header.Set("Version", "image-client-version")
	req.Header.Set("originator", "image-client-originator")
	req.Header.Set("x-codex-turn-state", "image-turn-state")
	req.Header.Set("x-codex-turn-metadata", `{"source":"image-header"}`)
	req.Header.Set("x-codex-routing-hint", "model=image-client-model")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/backend-api/codex/responses", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "acct_image", capturedHeaders.Get("ChatGPT-Account-Id"))
	assert.Equal(t, "application/vnd.example+json", capturedHeaders.Get("Accept"))
	assert.Equal(t, "image-beta", capturedHeaders.Get("OpenAI-Beta"))
	assert.Equal(t, "image-client-version", capturedHeaders.Get("Version"))
	assert.Equal(t, "image-client-originator", capturedHeaders.Get("originator"))
	assert.Equal(t, "image-turn-state", capturedHeaders.Get("x-codex-turn-state"))
	assert.Equal(t, `{"source":"image-header"}`, capturedHeaders.Get("x-codex-turn-metadata"))
	assert.Equal(t, "model=image-client-model", capturedHeaders.Get("x-codex-routing-hint"))
	assert.Equal(t, "application/json; charset=utf-8", capturedHeaders.Get("Content-Type"))
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	assert.Equal(t, false, capturedBody["store"])
	assert.Equal(t, true, capturedBody["stream"])
	input, ok := capturedBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", message["type"])
	tools, ok := capturedBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_generation", tool["type"])
	assert.Equal(t, "gpt-image-2", tool["model"])
	assert.Equal(t, "1024x1024", tool["size"])
	assert.Equal(t, "low", tool["quality"])
	assert.Equal(t, "jpeg", tool["output_format"])
	assert.Equal(t, "opaque", tool["background"])
	assert.Equal(t, float64(60), tool["output_compression"])
	assert.NotContains(t, tool, "moderation")
	assert.NotContains(t, tool, "user")
	assert.Contains(t, w.Body.String(), `"b64_json":"Y29kZXgtaW1hZ2U="`)
	assert.Contains(t, w.Body.String(), `"revised_prompt":"revised image prompt"`)

	logData := cap.wait(t, 2*time.Second)
	assert.Equal(t, "image_generation", logData.CallType)
	assert.Equal(t, "chatgpt_codex_backend", logData.Provider)
	assert.Equal(t, 1674, logData.PromptTokens)
	assert.Equal(t, 66, logData.CompletionTokens)
	assert.Equal(t, 1740, logData.TotalTokens)
	assert.InDelta(t, 0.01035, logData.Cost, 0.000001)

	requestPayload, ok := logData.RequestPayload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_generation", requestPayload["type"])
	assert.Equal(t, "chatgpt_codex_backend", requestPayload["provider"])
	assert.Contains(t, requestPayload, "upstream_payload")

	responsePayload, ok := logData.ResponsePayload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_generation_response", responsePayload["type"])
	assert.Equal(t, 1, responsePayload["data_len"])
	responseData, ok := responsePayload["data"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, responseData, 1)
	assert.Equal(t, len("Y29kZXgtaW1hZ2U="), responseData[0]["b64_json_len"])
	assert.Contains(t, responseData[0], "b64_json_sha256")
	assert.Equal(t, "revised image prompt", responseData[0]["revised_prompt"])
}

func TestChatGPTCodexBackendImageGeneration_UsesCachedUsageBeforeAsyncRefresh(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	var capturedAuth string
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"Y29kZXgtaW1hZ2U="}}`,
				``,
				`data: {"type":"response.completed","response":{"output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))),
			Request: r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = "https://chatgpt.com"
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-a", testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-b", testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available"))
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available")},
		{snapshot: testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available")},
	}}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-image-2",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-image-2",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","n":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Bearer access-a", capturedAuth)
}

func TestChatGPTCodexBackendImageGeneration_RemainsOrgLevelRouting(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"Y29kZXgtaW1hZ2U="}}`,
				``,
				`data: {"type":"response.completed","response":{"output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))),
			Request: r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = "https://chatgpt.com"
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	for _, credentialID := range []string{"cred-a", "cred-b"} {
		seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), credentialID, testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	}
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-image-2",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-image-2",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","n":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	params := h.Config.ModelList[0].TianjiParams
	assert.NotEmpty(t, h.openAISubscriptionSticky[openAISubscriptionRouteKey(req.Context(), params)])
	assert.Empty(t, h.openAISubscriptionSticky[openAISubscriptionCodexSessionRouteKey(req.Context(), params, codexSessionIdentity{SessionID: "unused-session", Source: "direct"})])
}

func TestChatGPTCodexBackendImageEdit_RoutesToCodexResponsesTool(t *testing.T) {
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]any

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"ZWRpdGVkLWltYWdl"}}`,
				``,
				`data: {"type":"response.completed","response":{"output":[],"usage":{"input_tokens":100,"output_tokens":20,"total_tokens":120}}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))),
			Request: r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newImageCodexHarness(t, "https://chatgpt.com")
	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)
	h.Callbacks = reg
	reqBody, contentType := imageEditMultipartBody(t, map[string]string{
		"model":         "gpt-image-2",
		"prompt":        "make the sky orange",
		"n":             "1",
		"size":          "1024x1024",
		"quality":       "low",
		"image_field":   "image[]",
		"output_format": "png",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer client-key")
	req.Header.Set("ChatGPT-Account-Id", "client-account")
	req.Header.Set("Accept", "application/vnd.example+json")
	req.Header.Set("OpenAI-Beta", "image-beta")
	req.Header.Set("Version", "image-client-version")
	req.Header.Set("originator", "image-client-originator")
	req.Header.Set("x-codex-turn-state", "image-turn-state")
	req.Header.Set("x-codex-turn-metadata", `{"source":"image-header"}`)
	req.Header.Set("x-codex-routing-hint", "model=image-client-model")
	w := httptest.NewRecorder()

	h.ImagesEdit(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/backend-api/codex/responses", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "acct_image", capturedHeaders.Get("ChatGPT-Account-Id"))
	assert.Equal(t, "application/vnd.example+json", capturedHeaders.Get("Accept"))
	assert.Equal(t, "image-beta", capturedHeaders.Get("OpenAI-Beta"))
	assert.Equal(t, "image-client-version", capturedHeaders.Get("Version"))
	assert.Equal(t, "image-client-originator", capturedHeaders.Get("originator"))
	assert.Equal(t, "image-turn-state", capturedHeaders.Get("x-codex-turn-state"))
	assert.Equal(t, `{"source":"image-header"}`, capturedHeaders.Get("x-codex-turn-metadata"))
	assert.Equal(t, "model=image-client-model", capturedHeaders.Get("x-codex-routing-hint"))
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))
	assert.Equal(t, "gpt-5.5", capturedBody["model"])

	input, ok := capturedBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 2)
	text, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_text", text["type"])
	assert.Equal(t, "make the sky orange", text["text"])
	image, ok := content[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "data:image/png;base64,aW1hZ2UtYnl0ZXM=", image["image_url"])
	assert.NotContains(t, image, "url")

	tools, ok := capturedBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_generation", tool["type"])
	assert.Equal(t, "gpt-image-2", tool["model"])
	assert.Equal(t, "1024x1024", tool["size"])
	assert.Equal(t, "low", tool["quality"])
	assert.Equal(t, "png", tool["output_format"])
	assert.Contains(t, w.Body.String(), `"b64_json":"ZWRpdGVkLWltYWdl"`)

	logData := cap.wait(t, 2*time.Second)
	assert.Equal(t, "image_edit", logData.CallType)
	requestPayload, ok := logData.RequestPayload.(map[string]any)
	require.True(t, ok)
	payloadBytes, err := json.Marshal(requestPayload)
	require.NoError(t, err)
	assert.NotContains(t, string(payloadBytes), "data:image/png;base64")
	assert.NotContains(t, string(payloadBytes), "aW1hZ2UtYnl0ZXM=")
	assert.Contains(t, string(payloadBytes), `"redacted":true`)
	assert.Contains(t, string(payloadBytes), `"sha256"`)
}

func TestChatGPTCodexBackendImageEdit_UsesCachedUsageBeforeAsyncRefresh(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	var capturedAuth string
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"ZWRpdGVkLWltYWdl"}}`,
				``,
				`data: {"type":"response.completed","response":{"output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))),
			Request: r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = "https://chatgpt.com"
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-a", testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-b", testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available"))
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available")},
		{snapshot: testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available")},
	}}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-image-2",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-image-2",
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	reqBody, contentType := imageEditMultipartBody(t, map[string]string{
		"model":       "gpt-image-2",
		"prompt":      "make the sky orange",
		"n":           "1",
		"image_field": "image[]",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	h.ImagesEdit(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Bearer access-a", capturedAuth)
}

func TestChatGPTCodexBackendImageEdit_RejectsMissingModelBeforeFallbackProxy(t *testing.T) {
	var upstreamCalls atomic.Int32
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls.Add(1)
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"should not be called"}}`)),
			Request:    r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newImageCodexHarness(t, "https://chatgpt.com")
	reqBody, contentType := imageEditMultipartBody(t, map[string]string{
		"prompt": "edit this",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	h.ImagesEdit(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "model is required")
	assert.Equal(t, int32(0), upstreamCalls.Load())
}

func TestChatGPTCodexBackendImageEdit_RejectsInvalidIntegerFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
	}{
		{name: "n", field: "n"},
		{name: "output_compression", field: "output_compression"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newImageCodexHarness(t, "https://chatgpt.com")
			fields := map[string]string{
				"model":  "gpt-image-2",
				"prompt": "edit this",
			}
			fields[tc.field] = "not-an-int"
			reqBody, contentType := imageEditMultipartBody(t, fields)
			req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", contentType)
			w := httptest.NewRecorder()

			h.ImagesEdit(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tc.field+" must be an integer")
		})
	}
}

func TestChatGPTCodexBackendImageEdit_RejectsMaskBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Request:    r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newImageCodexHarness(t, "https://chatgpt.com")
	reqBody, contentType := imageEditMultipartBody(t, map[string]string{
		"model":  "gpt-image-2",
		"prompt": "edit this",
		"mask":   "mask-file",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	h.ImagesEdit(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "mask")
	assert.Equal(t, int32(0), upstreamCalls.Load())
}

func TestChatGPTCodexBackendImageGeneration_HTTPRouteWithAuth(t *testing.T) {
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]any

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"aHR0cC1yb3V0ZS1pbWFnZQ=="}}`,
				``,
				`data: {"type":"response.completed","response":{"output":[]}}`,
				``,
				`data: [DONE]`,
				``,
			}, "\n"))),
			Request: r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newImageCodexHarness(t, "https://chatgpt.com")
	const masterKey = "test-master-key"
	r := chi.NewRouter()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthConfig{MasterKey: masterKey}))
	r.Post("/v1/images/generations", h.ImageGeneration)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","n":1}`))
	req.Header.Set("Authorization", "Bearer "+masterKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/backend-api/codex/responses", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "acct_image", capturedHeaders.Get("ChatGPT-Account-Id"))
	assert.Empty(t, capturedHeaders.Get("Accept"))
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	assert.Equal(t, false, capturedBody["store"])
	assert.Equal(t, true, capturedBody["stream"])
	assert.Equal(t, map[string]any{"type": "image_generation"}, capturedBody["tool_choice"])

	tools, ok := capturedBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_generation", tool["type"])
	assert.Equal(t, "gpt-image-2", tool["model"])

	var out model.ImageGenerationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out.Data, 1)
	assert.Equal(t, "aHR0cC1yb3V0ZS1pbWFnZQ==", out.Data[0].B64JSON)
	assert.Empty(t, out.Data[0].URL)
}

func TestChatGPTCodexBackendImageGeneration_RejectsInvalidCount(t *testing.T) {
	h := newImageCodexHarness(t, "https://chatgpt.com")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","n":2}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"param":"n"`)
	assert.Empty(t, fetcher.calls)
}

func TestChatGPTCodexBackendImageGeneration_RejectsUnsupportedFields(t *testing.T) {
	h := newImageCodexHarness(t, "https://chatgpt.com")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","response_format":"url"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "response_format")
	assert.Empty(t, fetcher.calls)
}

func TestChatGPTCodexBackendImageGeneration_RejectsUnsupportedModeration(t *testing.T) {
	h := newImageCodexHarness(t, "https://chatgpt.com")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","moderation":"low"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "moderation")
	assert.Empty(t, fetcher.calls)
}

func TestChatGPTCodexBackendImageGeneration_RejectsUnknownFields(t *testing.T) {
	h := newImageCodexHarness(t, "https://chatgpt.com")
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat","unexpected_option":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "unknown extra parameters")
	assert.Empty(t, fetcher.calls)
}

func TestChatGPTCodexBackendImageGeneration_RejectsMalformedSSE(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: {not-json}\n\n")),
			Request:    r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	h := newImageCodexHarness(t, "https://chatgpt.com")
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ImageGeneration(w, req)

	require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "parse ChatGPT Codex image generation event")
}

func TestOpenAIAPIKeyWildcard_UsesOpenAICompatibleRouting(t *testing.T) {
	var codexCalls atomic.Int32
	var capturedPath string
	var capturedAuth string
	var capturedBody map[string]any

	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl_api_key",
			"object":  "chat.completion",
			"created": 1770000000,
			"model":   "gpt-5.5",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "api key ok"}, "finish_reason": "stop"}},
		})
	}))
	defer platform.Close()

	codex := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codexCalls.Add(1)
		t.Fatalf("API-key wildcard must not call Codex backend path %s", r.URL.Path)
	}))
	defer codex.Close()

	apiKey := "sk-api-key"
	apiBase := platform.URL + "/v1"
	h := mockHandlers(newMockStore())
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = codex.URL
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "openai/*",
		TianjiParams: config.TianjiParams{
			Model:   "openaicompat/*",
			APIKey:  &apiKey,
			APIBase: &apiBase,
			Overflow: map[string]any{
				"supports_non_stream": true,
			},
		},
	}}
	req := newWildcardChatRequest(`openai/gpt-5.5`)
	w := httptest.NewRecorder()

	h.ChatCompletion(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int32(0), codexCalls.Load(), "API-key wildcard must stay on OpenAI-compatible routing")
	assert.Equal(t, "/v1/chat/completions", capturedPath)
	assert.True(t, capturedAuth == "Bearer "+apiKey, "API-key wildcard must send configured API key as bearer auth")
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	assert.Contains(t, capturedBody, "messages")
}

func newOpenAIWildcardCodexHarness(t *testing.T, backendBaseURL, platformBaseURL string) *Handlers {
	t.Helper()
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-secret", AccountID: "acct_wildcard"},
	})
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backendBaseURL
	params := config.TianjiParams{
		Model:                           "openai/*",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}
	if platformBaseURL != "" {
		params.APIBase = &platformBaseURL
	}
	h.Config.ModelList = []config.ModelConfig{{ModelName: "openai/*", TianjiParams: params}}
	enableTestCodexNonStream(h, "openai/*")
	return h
}

func newImageCodexHarness(t *testing.T, backendBaseURL string) *Handlers {
	t.Helper()
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-secret", AccountID: "acct_image"},
	})
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backendBaseURL
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-image-2",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-image-2",
			OpenAISubscriptionCredentialIDs: []string{"cred-a"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}
	return h
}

func newWildcardChatRequest(model string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"`+model+`",
		"messages":[{"role":"user","content":"hello"}]
	}`))
}

func imageEditMultipartBody(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	imageFieldName := fields["image_field"]
	if imageFieldName == "" {
		imageFieldName = "image"
	}
	for key, value := range fields {
		if key == "image_field" {
			continue
		}
		if key == "mask" {
			part, err := writer.CreatePart(imageEditFormFileHeader("mask", "mask.png", "image/png"))
			require.NoError(t, err)
			_, err = part.Write([]byte(value))
			require.NoError(t, err)
			continue
		}
		require.NoError(t, writer.WriteField(key, value))
	}
	file, err := writer.CreatePart(imageEditFormFileHeader(imageFieldName, "input.png", "image/png"))
	require.NoError(t, err)
	_, err = file.Write([]byte("image-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body.Bytes(), writer.FormDataContentType()
}

func imageEditFormFileHeader(fieldName, filename, contentType string) textproto.MIMEHeader {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)
	return header
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
