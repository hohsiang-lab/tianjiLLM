package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestGlobalSpend_Success(t *testing.T) {
	m := newMockStore()
	m.getGlobalSpendFn = func(_ context.Context, _ db.GetGlobalSpendParams) (db.GetGlobalSpendRow, error) {
		return db.GetGlobalSpendRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend?since=2024-01-01T00:00:00Z", nil)
	h.GlobalSpend(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendByKeys_Success(t *testing.T) {
	m := newMockStore()
	m.getDailySpendByKeyFn = func(_ context.Context, _ db.GetDailySpendByKeyParams) ([]db.GetDailySpendByKeyRow, error) {
		return []db.GetDailySpendByKeyRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/keys?since=2024-01-01T00:00:00Z", nil)
	h.GlobalSpendByKeys(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendByModels_Success(t *testing.T) {
	m := newMockStore()
	m.getDailySpendByModelFn = func(_ context.Context, _ db.GetDailySpendByModelParams) ([]db.GetDailySpendByModelRow, error) {
		return []db.GetDailySpendByModelRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/models?since=2024-01-01T00:00:00Z", nil)
	h.GlobalSpendByModels(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendByTeams_Success(t *testing.T) {
	m := newMockStore()
	m.getDailySpendByTeamFn = func(_ context.Context, _ db.GetDailySpendByTeamParams) ([]db.GetDailySpendByTeamRow, error) {
		return []db.GetDailySpendByTeamRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/teams?since=2024-01-01T00:00:00Z", nil)
	h.GlobalSpendByTeams(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendByTags_Success(t *testing.T) {
	m := newMockStore()
	m.getDailySpendByTagFn = func(_ context.Context, _ db.GetDailySpendByTagParams) ([]db.GetDailySpendByTagRow, error) {
		return []db.GetDailySpendByTagRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/tags?since=2024-01-01T00:00:00Z", nil)
	h.GlobalSpendByTags(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendByProvider_Success(t *testing.T) {
	m := newMockStore()
	m.getGlobalSpendByProviderFn = func(_ context.Context, _ db.GetGlobalSpendByProviderParams) ([]db.GetGlobalSpendByProviderRow, error) {
		return []db.GetGlobalSpendByProviderRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/provider?since=2024-01-01T00:00:00Z", nil)
	h.GlobalSpendByProvider(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalActivity_Success(t *testing.T) {
	m := newMockStore()
	m.getGlobalActivityFn = func(_ context.Context, _ db.GetGlobalActivityParams) ([]db.GetGlobalActivityRow, error) {
		return []db.GetGlobalActivityRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/activity?since=2024-01-01T00:00:00Z", nil)
	h.GlobalActivity(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalActivityByModel_Success(t *testing.T) {
	m := newMockStore()
	m.getGlobalActivityByModelFn = func(_ context.Context, _ db.GetGlobalActivityByModelParams) ([]db.GetGlobalActivityByModelRow, error) {
		return []db.GetGlobalActivityByModelRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/activity/model?since=2024-01-01T00:00:00Z", nil)
	h.GlobalActivityByModel(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendReport_Default(t *testing.T) {
	m := newMockStore()
	m.getGlobalSpendReportFn = func(_ context.Context, _ db.GetGlobalSpendReportParams) ([]db.GetGlobalSpendReportRow, error) {
		return []db.GetGlobalSpendReportRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/report?since=2024-01-01T00:00:00Z", nil)
	h.GlobalSpendReport(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendReport_ByCustomer(t *testing.T) {
	m := newMockStore()
	m.getGlobalSpendReportByCustomerFn = func(_ context.Context, _ db.GetGlobalSpendReportByCustomerParams) ([]db.GetGlobalSpendReportByCustomerRow, error) {
		return []db.GetGlobalSpendReportByCustomerRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/report?since=2024-01-01T00:00:00Z&group_by=customer", nil)
	h.GlobalSpendReport(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendReport_ByKey(t *testing.T) {
	m := newMockStore()
	m.getGlobalSpendReportByKeyFn = func(_ context.Context, _ db.GetGlobalSpendReportByKeyParams) ([]db.GetGlobalSpendReportByKeyRow, error) {
		return []db.GetGlobalSpendReportByKeyRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/spend/report?since=2024-01-01T00:00:00Z&group_by=key", nil)
	h.GlobalSpendReport(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGlobalSpendReset_Success(t *testing.T) {
	m := newMockStore()
	m.resetAllKeySpendFn = func(_ context.Context) error { return nil }
	m.resetAllTeamSpendFn = func(_ context.Context) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/global/spend/reset", nil)
	h.GlobalSpendReset(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheHitStats_Success(t *testing.T) {
	m := newMockStore()
	m.getCacheHitStatsFn = func(_ context.Context, _ db.GetCacheHitStatsParams) ([]db.GetCacheHitStatsRow, error) {
		return []db.GetCacheHitStatsRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/global/activity/cache_hits?since=2024-01-01T00:00:00Z", nil)
	h.CacheHitStats(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSpendLogs_Success(t *testing.T) {
	m := newMockStore()
	m.getSpendLogsByFilterFn = func(_ context.Context, _ db.GetSpendLogsByFilterParams) ([]db.GetSpendLogsByFilterRow, error) {
		return []db.GetSpendLogsByFilterRow{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/spend/logs?since=2024-01-01T00:00:00Z&limit=10", nil)
	h.SpendLogs(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
