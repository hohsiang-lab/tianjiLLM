package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	provider.Register("openaicompat", openai.New())
}

type recordedOpenAIRequest struct {
	Method     string
	Path       string
	Auth       string
	OpenAIBeta string
	Body       string
}

type openAIEndpointRecorder struct {
	mu       sync.Mutex
	requests []recordedOpenAIRequest
}

func (r *openAIEndpointRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r.mu.Lock()
		r.requests = append(r.requests, recordedOpenAIRequest{
			Method:     req.Method,
			Path:       req.URL.EscapedPath(),
			Auth:       req.Header.Get("Authorization"),
			OpenAIBeta: req.Header.Get("OpenAI-Beta"),
			Body:       string(body),
		})
		r.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch req.URL.EscapedPath() {
		case "/v1/chat/completions":
			if strings.Contains(string(body), `"stream":true`) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"))
				return
			}
			_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		case "/backend-api/codex/responses":
			if strings.Contains(string(body), `"image_generation"`) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"image_generation_call\",\"result\":\"ZmFrZS1pbWFnZQ==\",\"revised_prompt\":\"cat\"}}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-image\",\"model\":\"dall-e-3\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n"))
				return
			}
			if strings.Contains(string(body), `"stream":true`) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte(strings.Join([]string{
					`data: {"type":"response.output_text.delta","response_id":"resp-test","sequence_number":1,"delta":"hi"}`,
					``,
					`data: {"type":"response.completed","response":{"id":"resp-test","model":"gpt-4o","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
					``,
				}, "\n")))
				return
			}
			_, _ = w.Write([]byte(`{"id":"resp-test","object":"response","model":"gpt-4o","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
		case "/backend-api/codex/responses/compact":
			_, _ = w.Write([]byte(`{"id":"resp-compact","object":"response.compaction","model":"gpt-4o","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}]}`))
		case "/v1/embeddings":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"text-embedding-3-small","usage":{"prompt_tokens":1,"total_tokens":1}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	})
}

func (r *openAIEndpointRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

// responseEndpointCount excludes Codex usage/metadata requests that use the same test server.
func (r *openAIEndpointRecorder) responseEndpointCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, request := range r.requests {
		if strings.HasPrefix(request.Path, "/backend-api/codex/responses") || strings.HasPrefix(request.Path, "/v1/") {
			count++
		}
	}
	return count
}

func (r *openAIEndpointRecorder) last(t *testing.T) recordedOpenAIRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.requests)
	return r.requests[len(r.requests)-1]
}

func (r *openAIEndpointRecorder) all() []recordedOpenAIRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedOpenAIRequest(nil), r.requests...)
}

func withOpenAIHostRewrite(t *testing.T, upstreamURL string) {
	t.Helper()
	parsed, err := url.Parse(upstreamURL)
	require.NoError(t, err)
	originalClient := http.DefaultClient
	originalTransport := http.DefaultTransport
	transport := rewriteOpenAIHostTransport{target: parsed, base: originalTransport}
	http.DefaultClient = &http.Client{Transport: transport}
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultClient = originalClient
		http.DefaultTransport = originalTransport
	})
}

type rewriteOpenAIHostTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t rewriteOpenAIHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "api.openai.com" {
		return t.base.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	copiedURL := *clone.URL
	clone.URL = &copiedURL
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func newOpenAISubscriptionEndpointHandlers(t *testing.T) *Handlers {
	t.Helper()
	now := time.Date(2026, 5, 7, 6, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "subscription-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	apiKey := "sk-fallback-must-not-leak"
	h.Config.ModelList = []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/gpt-4o",
				APIKey:                          &apiKey,
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			},
		},
		{
			ModelName: "gpt-3.5-turbo-instruct",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/gpt-3.5-turbo-instruct",
				APIKey:                          &apiKey,
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			},
		},
		{
			ModelName: "text-embedding-3-small",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/text-embedding-3-small",
				APIKey:                          &apiKey,
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			},
		},
		{
			ModelName: "dall-e-3",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/dall-e-3",
				APIKey:                          &apiKey,
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			},
		},
		{
			ModelName: "whisper-1",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/whisper-1",
				APIKey:                          &apiKey,
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			},
		},
		{
			ModelName: "tts-1",
			TianjiParams: config.TianjiParams{
				Model:                           "openai/tts-1",
				APIKey:                          &apiKey,
				OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
			},
		},
	}
	return h
}

func TestOfficialOpenAISubscriptionUnsupportedDirectHandlersFailClosed(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		handler func(*Handlers, http.ResponseWriter, *http.Request)
	}{
		{
			name: "moderation",
			body: `{"model":"gpt-4o","input":"hello"}`,
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) {
				h.Moderation(w, r)
			},
		},
		{
			name: "rerank",
			body: `{"model":"gpt-4o","query":"hello","documents":["world"]}`,
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) {
				h.Rerank(w, r)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newOpenAISubscriptionEndpointHandlers(t)
			req := httptest.NewRequest(http.MethodPost, "/v1/"+tc.name, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			tc.handler(h, w, req)

			require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "does not support "+tc.name)
		})
	}
}

func TestOfficialOpenAIEndpoints_UseSubscriptionBearer(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		content    string
		makeBody   func(*testing.T) (string, string)
		handler    func(*Handlers, http.ResponseWriter, *http.Request)
		wantPath   string
		wantBeta   string
		wantStatus int
		noUpstream bool
	}{
		{
			name:       "chat completions",
			method:     http.MethodPost,
			target:     "/v1/chat/completions",
			body:       `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			content:    "application/json",
			handler:    func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ChatCompletion(w, r) },
			wantStatus: http.StatusNotImplemented,
			noUpstream: true,
		},
		{
			name:       "chat completions streaming",
			method:     http.MethodPost,
			target:     "/v1/chat/completions",
			body:       `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`,
			content:    "application/json",
			handler:    func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ChatCompletion(w, r) },
			wantStatus: http.StatusNotImplemented,
			noUpstream: true,
		},
		{
			name:       "legacy completions",
			method:     http.MethodPost,
			target:     "/v1/completions",
			body:       `{"model":"gpt-3.5-turbo-instruct","prompt":"hi"}`,
			content:    "application/json",
			handler:    func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.Completion(w, r) },
			wantPath:   "/v1/completions",
			wantStatus: http.StatusNotImplemented,
			noUpstream: true},
		{
			name:     "responses",
			method:   http.MethodPost,
			target:   "/v1/responses",
			body:     `{"model":"gpt-4o","input":"hi"}`,
			content:  "application/json",
			handler:  func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.CreateResponse(w, r) },
			wantPath: "/backend-api/codex/responses",
		},
		{
			name:     "responses compact",
			method:   http.MethodPost,
			target:   "/v1/responses/compact",
			body:     `{"model":"gpt-4o","input":"hi"}`,
			content:  "application/json",
			handler:  func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.CompactResponse(w, r) },
			wantPath: "/backend-api/codex/responses/compact",
		},
		{
			name:       "embeddings",
			method:     http.MethodPost,
			target:     "/v1/embeddings",
			body:       `{"model":"text-embedding-3-small","input":"hello"}`,
			content:    "application/json",
			handler:    func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.Embedding(w, r) },
			wantPath:   "/v1/embeddings",
			wantStatus: http.StatusNotImplemented,
			noUpstream: true},
		{
			name:     "image generation",
			method:   http.MethodPost,
			target:   "/v1/images/generations",
			body:     `{"model":"dall-e-3","prompt":"cat"}`,
			content:  "application/json",
			handler:  func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ImageGeneration(w, r) },
			wantPath: "/backend-api/codex/responses",
		},
		{
			name:     "image edits",
			method:   http.MethodPost,
			target:   "/v1/images/edits",
			makeBody: multipartModelBody("dall-e-3"),
			handler:  func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ImagesEdit(w, r) },
			wantPath: "/backend-api/codex/responses",
		},
		{
			name:       "image variations",
			method:     http.MethodPost,
			target:     "/v1/images/variations",
			makeBody:   multipartModelBody("dall-e-3"),
			handler:    func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ImageVariation(w, r) },
			wantPath:   "/v1/images/variations",
			wantStatus: http.StatusNotImplemented,
			noUpstream: true},
		{
			name:       "audio transcriptions",
			method:     http.MethodPost,
			target:     "/v1/audio/transcriptions",
			body:       "model=whisper-1",
			content:    "application/x-www-form-urlencoded",
			handler:    func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.AudioTranscription(w, r) },
			wantPath:   "/v1/audio/transcriptions",
			wantStatus: http.StatusNotImplemented,
			noUpstream: true},
		{
			name:       "audio speech",
			method:     http.MethodPost,
			target:     "/v1/audio/speech",
			body:       `{"model":"tts-1","input":"hi","voice":"alloy"}`,
			content:    "application/json",
			handler:    func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.AudioSpeech(w, r) },
			wantPath:   "/v1/audio/speech",
			wantStatus: http.StatusNotImplemented,
			noUpstream: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newOpenAISubscriptionEndpointHandlers(t)
			h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
			body := tt.body
			content := tt.content
			if tt.makeBody != nil {
				body, content = tt.makeBody(t)
			}
			before := recorder.responseEndpointCount()
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(body))
			req.Header.Set("Content-Type", content)
			req.Header.Set("Authorization", "Bearer sk-client-virtual-key")
			w := httptest.NewRecorder()

			tt.handler(h, w, req)

			if tt.wantStatus != 0 {
				assert.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.noUpstream {
				assert.Equal(t, before, recorder.responseEndpointCount())
				return
			}
			assert.Less(t, w.Code, http.StatusBadRequest, w.Body.String())
			assert.Equal(t, before+1, recorder.responseEndpointCount())
			got := recorder.last(t)
			assert.Equal(t, tt.wantPath, got.Path)
			assert.Equal(t, "Bearer subscription-access", got.Auth)
			assert.Equal(t, tt.wantBeta, got.OpenAIBeta)
			assert.NotContains(t, got.Auth, "sk-client-virtual-key")
			assert.NotContains(t, got.Auth, "sk-fallback-must-not-leak")
		})
	}
}

func TestAssistantsProxy_FailsClosedWithoutCodexAdapter(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	h := newOpenAISubscriptionEndpointHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/assistants", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.AssistantCreate(w, req)

	require.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Empty(t, recorder.all())
}

func TestAssistantsProxy_OfficialModelDoesNotUseLegacyAssistantSettings(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()

	h := newOpenAISubscriptionEndpointHandlers(t)
	h.Config.AssistantSettings = &config.AssistantSettings{APIBase: upstream.URL, APIKey: "[REDACTED]"}
	req := httptest.NewRequest(http.MethodPost, "/v1/assistants", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.AssistantCreate(w, req)

	require.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Empty(t, recorder.all())
}

func multipartModelBody(modelName string) func(*testing.T) (string, string) {
	return func(t *testing.T) (string, string) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", modelName))
		require.NoError(t, writer.WriteField("prompt", "edit this image"))
		file, err := writer.CreateFormFile("image", "image.png")
		require.NoError(t, err)
		_, err = file.Write([]byte("fake-image"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		return body.String(), writer.FormDataContentType()
	}
}

func TestOfficialOpenAIEndpoints_NoSubscriptionFailsClosed(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	apiKey := "sk-existing"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "gpt-4o",
		TianjiParams: config.TianjiParams{
			Model:  "openai/gpt-4o",
			APIKey: &apiKey,
		},
	}}}}

	tests := []struct {
		name     string
		target   string
		handler  func(*Handlers, http.ResponseWriter, *http.Request)
		wantPath string
	}{
		{
			name:     "responses",
			target:   "/v1/responses",
			handler:  func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.CreateResponse(w, r) },
			wantPath: "/v1/responses",
		},
		{
			name:     "responses compact",
			target:   "/v1/responses/compact",
			handler:  func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.CompactResponse(w, r) },
			wantPath: "/v1/responses/compact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := recorder.responseEndpointCount()
			req := httptest.NewRequest(http.MethodPost, tt.target, strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			tt.handler(h, w, req)

			require.GreaterOrEqual(t, w.Code, http.StatusBadRequest)
			assert.Equal(t, before, recorder.responseEndpointCount())
		})
	}
}

func TestOpenAIEndpointProxy_CustomBaseFailsClosedForOfficialOpenAI(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()

	apiKey := "sk-custom"
	apiBase := upstream.URL + "/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "custom-openai",
		TianjiParams: config.TianjiParams{
			Model:   "openai/gpt-4o",
			APIKey:  &apiKey,
			APIBase: &apiBase,
		},
	}}}}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"custom-openai","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	assert.GreaterOrEqual(t, w.Code, http.StatusBadRequest)
	assert.Empty(t, recorder.all())
}

func TestOpenAIEndpointProxy_NoConfigDoesNotReturnAssistantsError(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "OpenAI endpoint not configured")
	assert.NotContains(t, w.Body.String(), "assistants API not configured")
}

func TestOfficialOpenAIEndpoint_SubscriptionResolutionErrorDoesNotFallbackAPIKey(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(recorder.handler())
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	apiKey := "sk-fallback-must-not-leak"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "gpt-4o",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-4o",
			APIKey:                          &apiKey,
			OpenAISubscriptionCredentialIDs: []string{"cred-missing-db"},
		},
	}}}}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "OpenAI subscription credential resolution failed")
	assert.Zero(t, recorder.responseEndpointCount())
}

func TestResolveProviderBaseURL_CustomBaseUsesAPIKey(t *testing.T) {
	apiKey := "sk-custom"
	apiBase := "https://proxy.example.com/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "custom-openai",
		TianjiParams: config.TianjiParams{
			Model:   "openaicompat/gpt-4o",
			APIKey:  &apiKey,
			APIBase: &apiBase,
		},
	}}}}

	baseURL, gotAPIKey, err := h.resolveProviderBaseURL("custom-openai")

	require.NoError(t, err)
	assert.Equal(t, apiBase, baseURL)
	assert.Equal(t, apiKey, gotAPIKey)
}

func TestOpenAISubscriptionEndpointRequests_DoNotExposeSecretBodyFields(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "refresh-secret")
		r.Body = io.NopCloser(bytes.NewReader(body))
		recorder.handler().ServeHTTP(w, r)
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	h := newOpenAISubscriptionEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-4o","input":"hi"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Less(t, w.Code, http.StatusBadRequest, w.Body.String())
	assert.Equal(t, "Bearer subscription-access", recorder.last(t).Auth)
}

func TestResolveProviderRoute_UsesRequestContextForGlobalSubscription(t *testing.T) {
	h := newOpenAISubscriptionEndpointHandlers(t)
	route, err := h.resolveProviderFromConfigRouteWithContext(context.Background(), "gpt-4o")

	require.NoError(t, err)
	assert.Empty(t, route.APIKey)
	assert.True(t, route.ChatGPTCodexBackend)
	require.NotEmpty(t, route.SubscriptionCandidates)
	assert.Equal(t, "subscription-access", route.SubscriptionCandidates[0].BearerToken)
}

func TestOpenAISubscriptionEndpoints_UpdateRateLimitStateOn200And429(t *testing.T) {
	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	status := http.StatusOK
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-requests", "100")
		w.Header().Set("x-ratelimit-remaining-requests", "0")
		w.Header().Set("x-ratelimit-reset-requests", "60s")
		w.WriteHeader(status)
		if status == http.StatusTooManyRequests {
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	state, ok := h.openAISubscriptionRateLimitState("cred-a")
	require.True(t, ok)
	assert.True(t, state.Gated(h.openAISubscriptionNowUTC()))

	status = http.StatusTooManyRequests
	req = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	h.CreateResponse(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	state, ok = h.openAISubscriptionRateLimitState("cred-b")
	require.True(t, ok)
	assert.True(t, state.Gated(h.openAISubscriptionNowUTC()))
}

func TestOpenAISubscriptionRouting_FailoverAfterRefreshFailure(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()

		if r.Header.Get("Authorization") == "Bearer access-a" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"expired bearer access-a"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.output_text.delta","response_id":"resp-test","delta":"hi"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp-test","model":"gpt-4o","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-cred-a",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "refresh token rejected",
		},
	}}})
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", RefreshToken: "refresh-cred-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", RefreshToken: "refresh-cred-b", AccountID: "acct-b"},
	})
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{TokenURL: tokenServer.TokenURL(), ClientID: "app_test", CodexBackendBaseURL: upstream.URL}
	h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(tokenServer.Host())
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-4o",
		TianjiParams: config.TianjiParams{
			Model: "openai/gpt-4o",
		},
	}}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 2)
	assert.Equal(t, "Bearer access-a", requests[0].Auth)
	assert.Equal(t, "Bearer access-b", requests[1].Auth)
	cred, err := h.DB.GetCredential(context.Background(), "cred-a")
	require.NoError(t, err)
	var info OpenAISubscriptionCredentialInfo
	require.NoError(t, json.Unmarshal(cred.CredentialInfo, &info))
	assert.Equal(t, "refresh_failed", info.Status)
}

func TestOpenAISubscriptionRouting_401RefreshRetrySameCredentialSucceeds(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()

		if r.Header.Get("Authorization") == "Bearer subscription-access" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"expired bearer subscription-access"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.output_text.delta","response_id":"resp-test","delta":"hi"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp-test","model":"gpt-4o","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	now := time.Date(2026, 5, 7, 6, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "subscription-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token": "subscription-access-refreshed",
			"expires_in":   1800,
			"account_id":   "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	apiKey := "sk-fallback-must-not-leak"
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-4o",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-4o",
			APIKey:                          &apiKey,
			OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
		},
	}}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 2)
	assert.Equal(t, "Bearer subscription-access", requests[0].Auth)
	assert.Equal(t, "Bearer subscription-access-refreshed", requests[1].Auth)
	assert.Equal(t, requests[0].Body, requests[1].Body)
	assert.NotContains(t, w.Body.String(), "subscription-access")
}

func TestOpenAISubscriptionRouting_401RefreshFailureFailsOver(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()

		if r.Header.Get("Authorization") == "Bearer access-a" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"expired bearer access-a"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.output_text.delta","response_id":"resp-test","delta":"hi"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp-test","model":"gpt-4o","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-cred-a",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "refresh token rejected",
		},
	}}})
	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{TokenURL: tokenServer.TokenURL(), ClientID: "app_test"}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(tokenServer.Host())
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 2)
	assert.Equal(t, "Bearer access-a", requests[0].Auth)
	assert.Equal(t, "Bearer access-b", requests[1].Auth)
	cred, err := h.DB.GetCredential(context.Background(), "cred-a")
	require.NoError(t, err)
	var info OpenAISubscriptionCredentialInfo
	require.NoError(t, json.Unmarshal(cred.CredentialInfo, &info))
	assert.Equal(t, "refresh_failed", info.Status)
	assert.Equal(t, "refresh_failed", info.DisabledReason)
	assert.NotContains(t, info.LastError, "refresh-cred-a")
}

func TestOpenAISubscriptionProxyTransport_401RefreshRetrySameCredentialSucceeds(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()

		if r.Header.Get("Authorization") == "Bearer subscription-access" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"expired bearer subscription-access"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	now := time.Date(2026, 5, 7, 6, 0, 0, 0, time.UTC)
	store := newOpenAISubscriptionRefreshStore(t, OpenAISubscriptionTokenBundle{
		AccessToken:  "subscription-access",
		RefreshToken: "refresh-secret",
		ExpiresAt:    now.Add(time.Hour),
		AccountID:    "acct_123",
	}, activeOpenAISubscriptionInfo())
	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-secret",
		Response: map[string]any{
			"access_token": "subscription-access-refreshed",
			"expires_in":   1800,
			"account_id":   "acct_123",
		},
	}}})
	h := newOpenAISubscriptionRefreshHarness(t, now, store, tokenServer)
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	apiKey := "sk-fallback-must-not-leak"
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-4o",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-4o",
			APIKey:                          &apiKey,
			OpenAISubscriptionCredentialIDs: []string{"cred-refresh"},
		},
	}}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 2)
	assert.Equal(t, "Bearer subscription-access", requests[0].Auth)
	assert.Equal(t, "Bearer subscription-access-refreshed", requests[1].Auth)
	assert.Equal(t, requests[0].Body, requests[1].Body)
	assert.NotContains(t, w.Body.String(), "subscription-access")
}

func TestOpenAISubscriptionRouting_401RetryStillFailsDisablesAndFailsOver(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()

		switch r.Header.Get("Authorization") {
		case "Bearer access-a", "Bearer access-a-refreshed":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Bearer access-a-refreshed invalid"}}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-cred-a",
		Response: map[string]any{
			"access_token": "access-a-refreshed",
			"expires_in":   1800,
			"account_id":   "acct-a",
		},
	}}})
	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{TokenURL: tokenServer.TokenURL(), ClientID: "app_test"}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(tokenServer.Host())
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 3)
	assert.Equal(t, "Bearer access-a", requests[0].Auth)
	assert.Equal(t, "Bearer access-a-refreshed", requests[1].Auth)
	assert.Equal(t, "Bearer access-b", requests[2].Auth)

	cred, err := h.DB.GetCredential(context.Background(), "cred-a")
	require.NoError(t, err)
	var info OpenAISubscriptionCredentialInfo
	require.NoError(t, json.Unmarshal(cred.CredentialInfo, &info))
	assert.Equal(t, "disabled", info.Status)
	assert.Equal(t, "auth_failed_after_refresh", info.DisabledReason)
	assert.NotContains(t, info.LastError, "access-a")
}

func TestOpenAISubscriptionRouting_AllCredentialsAuthFailedReturnsReauthError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Bearer access-a-refreshed access-b invalid sk-fallback-must-not-leak"}}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-cred-a",
		Response: map[string]any{
			"access_token": "access-a-refreshed",
			"expires_in":   1800,
			"account_id":   "acct-a",
		},
	}, {
		RefreshToken: "refresh-cred-b",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "refresh token rejected",
		},
	}}})
	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{TokenURL: tokenServer.TokenURL(), ClientID: "app_test"}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(tokenServer.Host())
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "openai_subscription_reauthorization_required")
	assert.NotContains(t, w.Body.String(), "access-a")
	assert.NotContains(t, w.Body.String(), "access-b")
	assert.NotContains(t, w.Body.String(), "sk-fallback-must-not-leak")
}

func TestOpenAISubscriptionRouting_DirectHandlerAllCredentialsAuthFailedReturnsReauthError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Bearer access-a-refreshed access-b invalid sk-fallback-must-not-leak"}}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	tokenServer := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		RefreshToken: "refresh-cred-a",
		Response: map[string]any{
			"access_token": "access-a-refreshed",
			"expires_in":   1800,
			"account_id":   "acct-a",
		},
	}, {
		RefreshToken: "refresh-cred-b",
		StatusCode:   http.StatusBadRequest,
		Response: map[string]any{
			"error": "invalid_grant",
		},
	}}})
	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{TokenURL: tokenServer.TokenURL(), ClientID: "app_test"}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	h.OpenAIOAuthHTTPClient = openaitest.NewGuardedClient(tokenServer.Host())
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "openai_subscription_reauthorization_required")
	assert.NotContains(t, w.Body.String(), "access-a")
	assert.NotContains(t, w.Body.String(), "access-b")
	assert.NotContains(t, w.Body.String(), "sk-fallback-must-not-leak")
}

func TestOpenAISubscriptionRouting_401DoesNotRefreshCompatibilityAPIKeyPath(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	h := newOpenAISubscriptionEndpointHandlers(t)
	h.Config.ModelList[0].TianjiParams.Model = "openaicompat/gpt-4o"
	h.Config.ModelList[0].TianjiParams.OpenAISubscriptionCredentialIDs = nil
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 1)
	assert.Equal(t, "Bearer sk-fallback-must-not-leak", requests[0].Auth)
}

func TestOpenAISubscriptionRouting_FailoverOnOpenAI429BeforeReturn(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()

		if r.Header.Get("Authorization") == "Bearer access-a" {
			w.Header().Set("x-ratelimit-limit-requests", "100")
			w.Header().Set("x-ratelimit-remaining-requests", "0")
			w.Header().Set("x-ratelimit-reset-requests", "60s")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 2)
	assert.Equal(t, "Bearer access-a", requests[0].Auth)
	assert.Equal(t, "Bearer access-b", requests[1].Auth)
	state, ok := h.openAISubscriptionRateLimitState("cred-a")
	require.True(t, ok)
	assert.True(t, state.Gated(h.openAISubscriptionNowUTC()))
}

func TestOpenAISubscriptionRouting_ProviderHandlersFailoverOnOpenAI429BeforeReturn(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		target  string
		body    string
		content string
		handler func(*Handlers, http.ResponseWriter, *http.Request)
	}{
		{
			name:    "chat completions",
			method:  http.MethodPost,
			target:  "/v1/chat/completions",
			body:    `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`,
			content: "application/json",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ChatCompletion(w, r) },
		},
		{
			name:    "legacy completions",
			method:  http.MethodPost,
			target:  "/v1/completions",
			body:    `{"model":"gpt-3.5-turbo-instruct","prompt":"hi"}`,
			content: "application/json",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.Completion(w, r) },
		},
		{
			name:    "embeddings",
			method:  http.MethodPost,
			target:  "/v1/embeddings",
			body:    `{"model":"text-embedding-3-small","input":"hello"}`,
			content: "application/json",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.Embedding(w, r) },
		},
		{
			name:    "image generation",
			method:  http.MethodPost,
			target:  "/v1/images/generations",
			body:    `{"model":"dall-e-3","prompt":"cat"}`,
			content: "application/json",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ImageGeneration(w, r) },
		},
		{
			name:    "audio transcriptions",
			method:  http.MethodPost,
			target:  "/v1/audio/transcriptions",
			body:    "model=whisper-1",
			content: "application/x-www-form-urlencoded",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.AudioTranscription(w, r) },
		},
		{
			name:    "audio speech",
			method:  http.MethodPost,
			target:  "/v1/audio/speech",
			body:    `{"model":"tts-1","input":"hi","voice":"alloy"}`,
			content: "application/json",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.AudioSpeech(w, r) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &openAIEndpointRecorder{}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				recorder.mu.Lock()
				recorder.requests = append(recorder.requests, recordedOpenAIRequest{
					Method: r.Method,
					Path:   r.URL.EscapedPath(),
					Auth:   r.Header.Get("Authorization"),
					Body:   string(body),
				})
				recorder.mu.Unlock()

				if r.Header.Get("Authorization") == "Bearer access-a" {
					w.Header().Set("x-ratelimit-limit-requests", "100")
					w.Header().Set("x-ratelimit-remaining-requests", "0")
					w.Header().Set("x-ratelimit-reset-requests", "60s")
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
					return
				}
				if r.URL.Path == "/backend-api/codex/responses" {
					w.Header().Set("Content-Type", "text/event-stream")
					if strings.Contains(string(body), `"image_generation"`) {
						_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"image_generation_call\",\"result\":\"ZmFrZS1pbWFnZQ==\",\"revised_prompt\":\"cat\"}}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-image\",\"model\":\"dall-e-3\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n"))
					} else {
						_, _ = w.Write([]byte(strings.Join([]string{
							`data: {"type":"response.output_text.delta","response_id":"resp-test","delta":"hi"}`,
							``,
							`data: {"type":"response.completed","response":{"id":"resp-test","model":"gpt-4o","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
							``,
							`data: [DONE]`,
							``,
						}, "\n")))
					}
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/v1/embeddings" {
					_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"text-embedding-3-small","usage":{"prompt_tokens":1,"total_tokens":1}}`))
					return
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer upstream.Close()
			withOpenAIHostRewrite(t, upstream.URL)

			h := openAISubscriptionMultiCredentialEndpointHandlers(t)
			h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.content)
			w := httptest.NewRecorder()

			tt.handler(h, w, req)

			if tt.name == "chat completions" || tt.name == "legacy completions" || tt.name == "embeddings" || tt.name == "audio transcriptions" || tt.name == "audio speech" {
				require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
				assert.Empty(t, recorder.all())
				return
			}
			require.Less(t, w.Code, http.StatusBadRequest, w.Body.String())
			requests := recorder.all()
			require.Len(t, requests, 2)
			assert.Equal(t, "Bearer access-a", requests[0].Auth)
			assert.Equal(t, "Bearer access-b", requests[1].Auth)
			assert.Equal(t, requests[0].Body, requests[1].Body)
			state, ok := h.openAISubscriptionRateLimitState("cred-a")
			require.True(t, ok)
			assert.True(t, state.Gated(h.openAISubscriptionNowUTC()))
		})
	}
}

func TestOfficialOpenAIResponsesSiblingEndpointsFailClosed(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		target  string
		handler func(*Handlers, http.ResponseWriter, *http.Request)
	}{
		{
			name:    "get response",
			method:  http.MethodGet,
			target:  "/v1/responses/resp_123",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.GetResponse(w, r) },
		},
		{
			name:    "cancel response",
			method:  http.MethodPost,
			target:  "/v1/responses/resp_123/cancel",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.CancelResponse(w, r) },
		},
		{
			name:    "list input items",
			method:  http.MethodGet,
			target:  "/v1/responses/resp_123/input_items",
			handler: func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ListResponseInputItems(w, r) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &openAIEndpointRecorder{}
			upstream := httptest.NewServer(recorder.handler())
			defer upstream.Close()
			withOpenAIHostRewrite(t, upstream.URL)

			h := openAISubscriptionMultiCredentialEndpointHandlers(t)
			h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
			req := httptest.NewRequest(tt.method, tt.target, nil)
			w := httptest.NewRecorder()

			tt.handler(h, w, req)

			require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
			assert.Empty(t, recorder.all())
		})
	}
}

func TestOpenAISubscriptionRouting_FailoverOnOpenAI5xxBeforeReturn(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()

		if r.Header.Get("Authorization") == "Bearer access-a" {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream overloaded"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	requests := recorder.all()
	require.Len(t, requests, 2)
	assert.Equal(t, "Bearer access-a", requests[0].Auth)
	assert.Equal(t, "Bearer access-b", requests[1].Auth)
}

func TestOpenAISubscriptionRouting_StreamsReturnedWithoutPreemptiveRetry(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"delta\":\"first\"}\n\n"))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, recorder.all(), 1)
	assert.Contains(t, w.Body.String(), "first")
}

func TestOpenAISubscriptionRouting_NoFailoverOnNonRateLimit4xx(t *testing.T) {
	recorder := &openAIEndpointRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedOpenAIRequest{
			Method: r.Method,
			Path:   r.URL.EscapedPath(),
			Auth:   r.Header.Get("Authorization"),
			Body:   string(body),
		})
		recorder.mu.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid request"}}`))
	}))
	defer upstream.Close()
	withOpenAIHostRewrite(t, upstream.URL)

	h := openAISubscriptionMultiCredentialEndpointHandlers(t)
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = upstream.URL
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	requests := recorder.all()
	require.Len(t, requests, 1)
	assert.Equal(t, "Bearer access-a", requests[0].Auth)
}

func openAISubscriptionMultiCredentialEndpointHandlers(t *testing.T) *Handlers {
	t.Helper()
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	apiKey := "sk-fallback-must-not-leak"
	subscriptionParams := func(model string) config.TianjiParams {
		return config.TianjiParams{
			Model:                           model,
			APIKey:                          &apiKey,
			OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
		}
	}
	h.Config.ModelList = []config.ModelConfig{
		{ModelName: "gpt-4o", TianjiParams: subscriptionParams("openai/gpt-4o")},
		{ModelName: "gpt-3.5-turbo-instruct", TianjiParams: subscriptionParams("openai/gpt-3.5-turbo-instruct")},
		{ModelName: "text-embedding-3-small", TianjiParams: subscriptionParams("openai/text-embedding-3-small")},
		{ModelName: "dall-e-3", TianjiParams: subscriptionParams("openai/dall-e-3")},
		{ModelName: "whisper-1", TianjiParams: subscriptionParams("openai/whisper-1")},
		{ModelName: "tts-1", TianjiParams: subscriptionParams("openai/tts-1")},
	}
	return h
}
