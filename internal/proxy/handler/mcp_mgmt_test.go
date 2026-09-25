package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestMCPServerCreate_Success(t *testing.T) {
	m := newMockStore()
	m.createMCPServerFn = func(_ context.Context, arg db.CreateMCPServerParams) (db.MCPServerTable, error) {
		return db.MCPServerTable{ServerID: arg.ServerID}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/mcp_server", strings.NewReader(`{"server_id":"s1","transport":"sse","url":"http://localhost"}`))
	r.Header.Set("Content-Type", "application/json")
	h.MCPServerCreate(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestMCPServerGet_Success(t *testing.T) {
	m := newMockStore()
	m.getMCPServerFn = func(_ context.Context, id string) (db.MCPServerTable, error) {
		return db.MCPServerTable{ServerID: id}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/mcp_server/s1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.MCPServerGet(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMCPServerList_Success(t *testing.T) {
	m := newMockStore()
	m.listMCPServersFn = func(_ context.Context) ([]db.MCPServerTable, error) {
		return []db.MCPServerTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/mcp_server/list", nil)
	h.MCPServerList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMCPServerUpdate_Success(t *testing.T) {
	m := newMockStore()
	m.updateMCPServerFn = func(_ context.Context, arg db.UpdateMCPServerParams) (db.MCPServerTable, error) {
		return db.MCPServerTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/mcp_server/s1", strings.NewReader(`{"transport":"sse","url":"http://updated"}`))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.MCPServerUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMCPServerDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteMCPServerFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/mcp_server/s1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.MCPServerDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
