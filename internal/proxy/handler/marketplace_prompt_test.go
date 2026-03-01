package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

// ---- Marketplace / Plugin ----

func TestPluginCreate_Success(t *testing.T) {
	ms := newMockStore()
	ms.createPluginFn = func(_ context.Context, arg db.CreatePluginParams) (db.ClaudeCodePluginTable, error) {
		return db.ClaudeCodePluginTable{Name: arg.Name}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"name": "my-plugin", "version": "1.0"})
	req := httptest.NewRequest(http.MethodPost, "/plugin/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PluginCreate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPluginCreate_NoDB(t *testing.T) {
	h := newTestHandlers()
	body, _ := json.Marshal(map[string]string{"name": "p"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PluginCreate(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPluginGet_Success(t *testing.T) {
	ms := newMockStore()
	ms.getPluginFn = func(_ context.Context, name string) (db.ClaudeCodePluginTable, error) {
		return db.ClaudeCodePluginTable{Name: name}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "my-plugin")
	req := httptest.NewRequest(http.MethodGet, "/plugin/my-plugin", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.PluginGet(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPluginList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listPluginsFn = func(_ context.Context, _ db.ListPluginsParams) ([]db.ClaudeCodePluginTable, error) {
		return []db.ClaudeCodePluginTable{{Name: "p1"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/plugin/list", nil)
	w := httptest.NewRecorder()
	h.PluginList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPluginEnable_Success(t *testing.T) {
	ms := newMockStore()
	ms.enablePluginFn = func(_ context.Context, _ string) error { return nil }
	ms.getPluginFn = func(_ context.Context, name string) (db.ClaudeCodePluginTable, error) {
		return db.ClaudeCodePluginTable{Name: name}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "p1")
	req := httptest.NewRequest(http.MethodPost, "/plugin/p1/enable", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.PluginEnable(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPluginDisable_Success(t *testing.T) {
	ms := newMockStore()
	ms.disablePluginFn = func(_ context.Context, _ string) error { return nil }
	ms.getPluginFn = func(_ context.Context, name string) (db.ClaudeCodePluginTable, error) {
		return db.ClaudeCodePluginTable{Name: name}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "p1")
	req := httptest.NewRequest(http.MethodPost, "/plugin/p1/disable", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.PluginDisable(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPluginDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deletePluginFn2 = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "p1")
	req := httptest.NewRequest(http.MethodDelete, "/plugin/p1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.PluginDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- Prompt Management ----

func TestPromptCreate_Success(t *testing.T) {
	ms := newMockStore()
	ms.createPromptFn = func(_ context.Context, arg db.CreatePromptTemplateParams) (db.PromptTemplateTable, error) {
		return db.PromptTemplateTable{Name: arg.Name}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"name": "my-prompt", "content": "Hello {{name}}"})
	req := httptest.NewRequest(http.MethodPost, "/prompt/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PromptCreate(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPromptCreate_NoDB(t *testing.T) {
	h := newTestHandlers()
	body, _ := json.Marshal(map[string]string{"name": "p"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PromptCreate(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPromptGet_Success(t *testing.T) {
	ms := newMockStore()
	ms.getPromptFn = func(_ context.Context, id string) (db.PromptTemplateTable, error) {
		return db.PromptTemplateTable{ID: id, Name: "test"}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	req := httptest.NewRequest(http.MethodGet, "/prompt/p1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.PromptGet(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPromptList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listPromptsFn = func(_ context.Context) ([]db.PromptTemplateTable, error) {
		return []db.PromptTemplateTable{{Name: "p1"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/prompt/list", nil)
	w := httptest.NewRecorder()
	h.PromptList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPromptDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deletePromptFn2 = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	req := httptest.NewRequest(http.MethodDelete, "/prompt/p1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.PromptDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
