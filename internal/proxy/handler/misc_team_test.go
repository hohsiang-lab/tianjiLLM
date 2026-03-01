package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

// ---- Misc (error logs, health check history) ----

func TestErrorLogsList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listErrorLogsFn = func(_ context.Context, _ db.ListErrorLogsParams) ([]db.ErrorLog, error) {
		return []db.ErrorLog{{RequestID: "r1"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/error_logs?limit=10", nil)
	w := httptest.NewRecorder()
	h.ErrorLogsList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestErrorLogsList_NoDB(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/error_logs", nil)
	w := httptest.NewRecorder()
	h.ErrorLogsList(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHealthCheckHistory_Success(t *testing.T) {
	ms := newMockStore()
	ms.listHealthChecksFn = func(_ context.Context, _ db.ListHealthChecksParams) ([]db.HealthCheckTable, error) {
		return []db.HealthCheckTable{}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/health/history?limit=10", nil)
	w := httptest.NewRecorder()
	h.HealthCheckHistory(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthCheckHistory_NoDB(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/health/history", nil)
	w := httptest.NewRecorder()
	h.HealthCheckHistory(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ---- Team Ext ----

func TestTeamListV2_Success(t *testing.T) {
	ms := newMockStore()
	ms.listTeamsFn = func(_ context.Context) ([]db.TeamTable, error) {
		return []db.TeamTable{{TeamID: "t1"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/team/list", nil)
	w := httptest.NewRecorder()
	h.TeamList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResetTeamSpend_Success(t *testing.T) {
	ms := newMockStore()
	ms.resetTeamSpendFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("team_id", "t1")
	req := httptest.NewRequest(http.MethodPost, "/team/t1/reset_spend", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.ResetTeamSpend(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTeamDailyActivity_Success(t *testing.T) {
	ms := newMockStore()
	ms.getTeamDailyActivityFn = func(_ context.Context, _ db.GetTeamDailyActivityParams) ([]db.GetTeamDailyActivityRow, error) {
		return nil, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/team/activity?team_id=t1", nil)
	w := httptest.NewRecorder()
	h.TeamDailyActivity(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserDailyActivity_Success(t *testing.T) {
	ms := newMockStore()
	ms.getUserDailyActivityFn = func(_ context.Context, _ db.GetUserDailyActivityParams) ([]db.GetUserDailyActivityRow, error) {
		return nil, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/user/activity?user_id=u1", nil)
	w := httptest.NewRecorder()
	h.UserDailyActivity(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
