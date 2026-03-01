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

// ---- Audit ----

func TestAuditLogList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listAuditLogsFn = func(_ context.Context, _ db.ListAuditLogsParams) ([]db.AuditLog, error) {
		return []db.AuditLog{{ID: "a1", Action: "create"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/audit?limit=10", nil)
	w := httptest.NewRecorder()
	h.AuditLogList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotNil(t, resp["data"])
}

func TestAuditLogList_NoDB(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	w := httptest.NewRecorder()
	h.AuditLogList(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAuditLogGet_Success(t *testing.T) {
	ms := newMockStore()
	ms.getAuditLogFn = func(_ context.Context, id string) (db.AuditLog, error) {
		return db.AuditLog{ID: id, Action: "update"}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "audit-123")
	req := httptest.NewRequest(http.MethodGet, "/audit/audit-123", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.AuditLogGet(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditLogGet_NoDB(t *testing.T) {
	h := newTestHandlers()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "x")
	req := httptest.NewRequest(http.MethodGet, "/audit/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.AuditLogGet(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ---- Agent ----

func TestAgentList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listAgentsFn = func(_ context.Context, _ db.ListAgentsParams) ([]db.AgentsTable, error) {
		return []db.AgentsTable{{AgentID: "a1", AgentName: "bot"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/agent/list", nil)
	w := httptest.NewRecorder()
	h.AgentList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentGet_Success(t *testing.T) {
	ms := newMockStore()
	ms.getAgentFn = func(_ context.Context, agentID string) (db.AgentsTable, error) {
		return db.AgentsTable{AgentID: agentID, AgentName: "bot"}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("agent_id", "a1")
	req := httptest.NewRequest(http.MethodGet, "/agent/a1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.AgentGet(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteAgentFn = func(_ context.Context, agentID string) error { return nil }
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("agent_id", "a1")
	req := httptest.NewRequest(http.MethodDelete, "/agent/a1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.AgentDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentCreate_Success(t *testing.T) {
	ms := newMockStore()
	ms.createAgentFn = func(_ context.Context, arg db.CreateAgentParams) (db.AgentsTable, error) {
		return db.AgentsTable{AgentID: "a-new", AgentName: arg.AgentName}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"agent_name": "my-bot", "created_by": "alice"})
	req := httptest.NewRequest(http.MethodPost, "/agent/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AgentCreate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentCreate_NoDB(t *testing.T) {
	h := newTestHandlers()
	body, _ := json.Marshal(map[string]string{"agent_name": "bot"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AgentCreate(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAgentUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.getAgentFn = func(_ context.Context, agentID string) (db.AgentsTable, error) {
		return db.AgentsTable{AgentID: agentID, AgentName: "old-name"}, nil
	}
	ms.updateAgentFn = func(_ context.Context, arg db.UpdateAgentParams) (db.AgentsTable, error) {
		return db.AgentsTable{AgentID: arg.AgentID}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("agent_id", "a1")
	body, _ := json.Marshal(map[string]string{"agent_name": "new-name"})
	req := httptest.NewRequest(http.MethodPut, "/agent/a1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.AgentUpdate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
