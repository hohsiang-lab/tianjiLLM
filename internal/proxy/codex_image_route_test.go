package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerRoutesImageGenerationThroughCodexBackend(t *testing.T) {
	const masterKey = "test-master-key-32-bytes-long!!!"
	credential := proxyImageRouteCredential(t, masterKey)
	store := proxyImageRouteStore{credential: credential}
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "gpt-image-2",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/gpt-image-2",
				OpenAISubscriptionCredentialIDs: []string{"cred-image"},
				OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
			},
		}},
	}
	cfg.GeneralSettings.MasterKey = masterKey
	cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = "https://chatgpt.com"

	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]any
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: proxyImageRouteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/backend-api/wham/usage" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"email":"operator@example.com","plan_type":"pro","rate_limit":{"primary_window":{"used_percent":0.1,"reset_at":"2026-05-13T07:30:00Z"},"secondary_window":{"used_percent":0.2,"reset_at":"2026-05-18T00:00:00Z"}}}`)),
				Request:    r,
			}, nil
		}
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"cHJveHktcm91dGUtaW1hZ2U="}}`,
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

	handlers := &handler.Handlers{
		Config:            cfg,
		DB:                store,
		Callbacks:         callback.NewRegistry(),
		CodexUsageFetcher: proxyImageRouteUsageFetcher{},
	}
	server := NewServer(ServerConfig{Handlers: handlers, MasterKey: masterKey})
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{
		"model":"gpt-image-2",
		"prompt":"draw a cat",
		"n":1,
		"quality":"low",
		"output_format":"jpeg",
		"background":"opaque",
		"output_compression":60
	}`))
	req.Header.Set("Authorization", "Bearer "+masterKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/backend-api/codex/responses", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "acct_image", capturedHeaders.Get("ChatGPT-Account-Id"))
	assert.Empty(t, capturedHeaders.Get("Accept"))
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
	assert.Equal(t, "low", tool["quality"])
	assert.Equal(t, "jpeg", tool["output_format"])
	assert.Equal(t, "opaque", tool["background"])
	assert.Equal(t, float64(60), tool["output_compression"])

	var out model.ImageGenerationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out.Data, 1)
	assert.Equal(t, "cHJveHktcm91dGUtaW1hZ2U=", out.Data[0].B64JSON)
}

func TestServerRoutesResponsesCompactThroughCodexBackend(t *testing.T) {
	const masterKey = "test-master-key-32-bytes-long!!!"
	credential := proxyImageRouteCredential(t, masterKey)
	store := proxyImageRouteStore{credential: credential}
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "openai/*",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/*",
				OpenAISubscriptionCredentialIDs: []string{"cred-image"},
				OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
			},
		}},
	}
	cfg.GeneralSettings.MasterKey = masterKey
	cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = "https://chatgpt.com"

	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]any
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: proxyImageRouteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/backend-api/wham/usage" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"email":"operator@example.com","plan_type":"pro","rate_limit":{"primary_window":{"used_percent":0.1,"reset_at":"2026-05-13T07:30:00Z"}}}`)),
				Request:    r,
			}, nil
		}
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_compact_route","object":"response.compaction","model":"gpt-5.5","output":[{"type":"message","role":"user","content":[{"type":"input_text","text":"compacted"}]}]}`)),
			Request:    r,
		}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = originalClient })

	handlers := &handler.Handlers{
		Config:            cfg,
		DB:                store,
		Callbacks:         callback.NewRegistry(),
		CodexUsageFetcher: proxyImageRouteUsageFetcher{},
	}
	passthrough := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"unknown pass-through endpoint"}`))
	})
	server := NewServer(ServerConfig{Handlers: handlers, MasterKey: masterKey, PassthroughHandler: passthrough})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"openai/gpt-5.5","input":"say OK","stream":true}`))
	req.Header.Set("Authorization", "Bearer "+masterKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "unknown pass-through endpoint")
	assert.Equal(t, "/backend-api/codex/responses/compact", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "acct_image", capturedHeaders.Get("ChatGPT-Account-Id"))
	assert.Empty(t, capturedHeaders.Get("Accept"))
	assert.Equal(t, "gpt-5.5", capturedBody["model"])
	assert.NotContains(t, capturedBody, "store")
	assert.NotContains(t, capturedBody, "stream")
	assert.Contains(t, w.Body.String(), `"object":"response.compaction"`)
	assert.Contains(t, w.Body.String(), `"output"`)
}

func TestServerRoutesImageEditThroughCodexBackend(t *testing.T) {
	const masterKey = "test-master-key-32-bytes-long!!!"
	credential := proxyImageRouteCredential(t, masterKey)
	store := proxyImageRouteStore{credential: credential}
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "gpt-image-2",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/gpt-image-2",
				OpenAISubscriptionCredentialIDs: []string{"cred-image"},
				OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
			},
		}},
	}
	cfg.GeneralSettings.MasterKey = masterKey
	cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = "https://chatgpt.com"

	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]any
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: proxyImageRouteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/backend-api/wham/usage" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"email":"operator@example.com","plan_type":"pro","rate_limit":{"primary_window":{"used_percent":0.1,"reset_at":"2026-05-13T07:30:00Z"},"secondary_window":{"used_percent":0.2,"reset_at":"2026-05-18T00:00:00Z"}}}`)),
				Request:    r,
			}, nil
		}
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"cHJveHktZWRpdC1pbWFnZQ=="}}`,
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

	handlers := &handler.Handlers{
		Config:            cfg,
		DB:                store,
		Callbacks:         callback.NewRegistry(),
		CodexUsageFetcher: proxyImageRouteUsageFetcher{},
	}
	server := NewServer(ServerConfig{Handlers: handlers, MasterKey: masterKey})
	body, contentType := proxyImageEditMultipartBody(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+masterKey)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	server.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/backend-api/codex/responses", capturedPath)
	assert.Equal(t, "Bearer access-secret", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "acct_image", capturedHeaders.Get("ChatGPT-Account-Id"))

	input, ok := capturedBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 2)
	image, ok := content[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_image", image["type"])

	var out model.ImageGenerationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out.Data, 1)
	assert.Equal(t, "cHJveHktZWRpdC1pbWFnZQ==", out.Data[0].B64JSON)
}

type proxyImageRouteStore struct {
	db.Store
	credential db.CredentialTable
}

func (s proxyImageRouteStore) GetCredential(_ context.Context, credentialID string) (db.CredentialTable, error) {
	if credentialID == s.credential.CredentialID {
		return s.credential, nil
	}
	return db.CredentialTable{}, errors.New("credential not found")
}

func (s proxyImageRouteStore) ListCredentials(_ context.Context) ([]db.CredentialTable, error) {
	return []db.CredentialTable{s.credential}, nil
}

func proxyImageRouteCredential(t *testing.T, masterKey string) db.CredentialTable {
	t.Helper()
	bundle, err := json.Marshal(handler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		ExpiresAt:    time.Now().Add(time.Hour),
		AccountID:    "acct_image",
	})
	require.NoError(t, err)
	encrypted, err := auth.Encrypt(string(bundle), masterKey)
	require.NoError(t, err)
	info, err := json.Marshal(handler.OpenAISubscriptionCredentialInfo{Status: "active"})
	require.NoError(t, err)
	return db.CredentialTable{
		CredentialID:    "cred-image",
		CredentialType:  handler.CredentialTypeOpenAISubscription,
		CredentialValue: encrypted,
		CredentialInfo:  info,
	}
}

type proxyImageRouteRoundTripFunc func(*http.Request) (*http.Response, error)

func (f proxyImageRouteRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type proxyImageRouteUsageFetcher struct{}

func (proxyImageRouteUsageFetcher) Fetch(context.Context, chatgptcodex.UsageRequest) (chatgptcodex.UsageSnapshot, error) {
	used := 0.1
	return chatgptcodex.UsageSnapshot{
		Email:         "operator@example.com",
		PlanType:      "pro",
		PrimaryWindow: chatgptcodex.UsageWindow{UsedPercent: &used, ResetAt: "2026-05-13T07:30:00Z"},
	}, nil
}

func proxyImageEditMultipartBody(t *testing.T) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "make the sky orange"))
	require.NoError(t, writer.WriteField("n", "1"))
	require.NoError(t, writer.WriteField("quality", "low"))
	file, err := writer.CreatePart(proxyImageEditFormFileHeader("image[]", "input.png", "image/png"))
	require.NoError(t, err)
	_, err = file.Write([]byte("image-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body.Bytes(), writer.FormDataContentType()
}

func proxyImageEditFormFileHeader(fieldName, filename, contentType string) textproto.MIMEHeader {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)
	return header
}
