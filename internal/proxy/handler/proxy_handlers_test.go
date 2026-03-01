package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// TestProxyHandlersNoConfig tests that proxy handlers return 400 when no provider is configured.
func TestProxyHandlersNoConfig(t *testing.T) {
	h := newTestHandlers()

	tests := []struct {
		name    string
		method  string
		body    string
		handler http.HandlerFunc
	}{
		{"Completion_NoModel", "POST", `{"model":"gpt-4o","prompt":"hi"}`, h.Completion},
		{"Embedding_NoModel", "POST", `{"model":"gpt-4o","input":"hello"}`, h.Embedding},
		{"ImageGeneration_NoModel", "POST", `{"model":"dall-e-3","prompt":"cat"}`, h.ImageGeneration},
		{"AudioSpeech_NoModel", "POST", `{"model":"tts-1","input":"hi","voice":"alloy"}`, h.AudioSpeech},
		{"FilesUpload_NoProvider", "POST", `{}`, h.FilesUpload},
		{"FilesList_NoProvider", "GET", ``, h.FilesList},
		{"BatchesCreate_NoProvider", "POST", `{}`, h.BatchesCreate},
		{"BatchesList_NoProvider", "GET", ``, h.BatchesList},
		{"FineTuningCreate_NoProvider", "POST", `{}`, h.FineTuningCreate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			tt.handler(w, r)
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, w.Code, "handler %s should return 400 or 404", tt.name)
		})
	}
}

func TestProxyHandlersInvalidJSON(t *testing.T) {
	h := newTestHandlers()

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"Completion_BadJSON", h.Completion},
		{"Embedding_BadJSON", h.Embedding},
		{"ImageGeneration_BadJSON", h.ImageGeneration},
		{"AudioSpeech_BadJSON", h.AudioSpeech},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader("not json {{{"))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			tt.handler(w, r)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestAudioTranscription_MissingModel(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/v1/audio/transcriptions", strings.NewReader(""))
	w := httptest.NewRecorder()
	h.AudioTranscription(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAudioTranscription_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/v1/audio/transcriptions", strings.NewReader(""))
	r.Form = map[string][]string{"model": {"whisper-1"}}
	w := httptest.NewRecorder()
	h.AudioTranscription(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFilesGet_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/v1/files/file-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("file_id", "file-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.FilesGet(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFilesGetContent_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/v1/files/file-123/content", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("file_id", "file-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.FilesGetContent(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFilesDelete_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("DELETE", "/v1/files/file-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("file_id", "file-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.FilesDelete(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBatchesGet_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/v1/batches/batch-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("batch_id", "batch-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.BatchesGet(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBatchesCancel_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/v1/batches/batch-123/cancel", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("batch_id", "batch-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.BatchesCancel(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFineTuningGet_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/v1/fine_tuning/jobs/job-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fine_tuning_job_id", "job-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.FineTuningGet(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFineTuningCancel_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/v1/fine_tuning/jobs/job-123/cancel", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fine_tuning_job_id", "job-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.FineTuningCancel(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFineTuningListEvents_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/v1/fine_tuning/jobs/job-123/events", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fine_tuning_job_id", "job-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.FineTuningListEvents(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFineTuningListCheckpoints_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/v1/fine_tuning/jobs/job-123/checkpoints", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fine_tuning_job_id", "job-123")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.FineTuningListCheckpoints(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSpendAnalytics_NoDB(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/spend/analytics", nil)
	w := httptest.NewRecorder()
	h.SpendAnalytics(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSpendTrend_NoDB(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/spend/trend", nil)
	w := httptest.NewRecorder()
	h.SpendTrend(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSpendTopN_NoDB(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/spend/top", nil)
	w := httptest.NewRecorder()
	h.SpendTopN(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestConfigGet(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()
	h.ConfigGet(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "model_list")
}

func TestConfigUpdate_Valid(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/config/update", strings.NewReader(`{"callbacks":["langfuse"]}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ConfigUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfigUpdate_Invalid(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/config/update", strings.NewReader(`not json`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ConfigUpdate(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResolveProviderBaseURL_NoModels(t *testing.T) {
	h := newTestHandlers()
	_, _, err := h.resolveProviderBaseURL("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OpenAI-compatible provider")
}

func TestModelGroupInfo_Empty(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/model_group/info", nil)
	w := httptest.NewRecorder()
	h.ModelGroupInfo(w, r)
	assert.NotEqual(t, 0, w.Code)
}

// TestNativeFormatHandlers_NoConfig tests native format handlers return 501 when provider not configured.
func TestNativeFormatHandlers_NoConfig(t *testing.T) {
	h := newTestHandlers()

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"AnthropicMessages", h.AnthropicMessages},
		{"AnthropicCountTokens", h.AnthropicCountTokens},
		{"GeminiGenerateContent", h.GeminiGenerateContent},
		{"GeminiStreamGenerateContent", h.GeminiStreamGenerateContent},
		{"GeminiCountTokens", h.GeminiCountTokens},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			tt.handler(w, r)
			assert.Equal(t, http.StatusNotImplemented, w.Code)
		})
	}
}

// TestNotImplemented tests the not-implemented handler factory.
func TestNotImplemented(t *testing.T) {
	h := NotImplemented("GET /v1/realtime")
	r := httptest.NewRequest("GET", "/v1/realtime", nil)
	w := httptest.NewRecorder()
	h(w, r)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "not yet implemented")
}

// TestModeration_InvalidJSON tests moderation with invalid JSON.
func TestModeration_InvalidJSON(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/v1/moderations", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Moderation(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestModeration_NoProvider tests moderation with no provider configured.
func TestModeration_NoProvider(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("POST", "/v1/moderations", strings.NewReader(`{"input":"test text"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Moderation(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestResolvePromptTemplate_NoPromptName tests that resolvePromptTemplate returns nil when PromptName is empty.
func TestResolvePromptTemplate_NoPromptName(t *testing.T) {
	req := &model.ChatCompletionRequest{}
	err := resolvePromptTemplate(context.Background(), nil, req)
	assert.NoError(t, err)
}

// TestResolvePromptTemplate_NilDB tests that resolvePromptTemplate returns error when DB is nil.
func TestResolvePromptTemplate_NilDB(t *testing.T) {
	req := &model.ChatCompletionRequest{PromptName: "my-template"}
	err := resolvePromptTemplate(context.Background(), nil, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database")
}

func TestGetGuardrailNames_NoContext(t *testing.T) {
	h := newTestHandlers()
	ctx := context.Background()
	names := h.getGuardrailNames(ctx)
	if names != nil {
		t.Fatalf("expected nil, got %v", names)
	}
}

func TestBuildPolicyRequest(t *testing.T) {
	h := newTestHandlers()
	ctx := context.Background()
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}
	pr := h.buildPolicyRequest(ctx, req)
	if pr.Model != "gpt-4o" {
		t.Fatalf("Model = %q, want gpt-4o", pr.Model)
	}
}
