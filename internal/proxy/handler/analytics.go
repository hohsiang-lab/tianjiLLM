package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/spend"
)

// SpendAnalytics handles GET /spend/analytics?group_by=team&start_date=...&end_date=...
func (h *Handlers) SpendAnalytics(w http.ResponseWriter, r *http.Request) {
	pool := h.getPool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	q := h.parseAnalyticsQuery(r)

	result, err := spend.QueryByGroup(r.Context(), pool, q)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SpendTopN handles GET /spend/top?group_by=model&n=10
func (h *Handlers) SpendTopN(w http.ResponseWriter, r *http.Request) {
	pool := h.getPool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	q := h.parseAnalyticsQuery(r)
	if n := r.URL.Query().Get("n"); n != "" {
		if v, err := strconv.Atoi(n); err == nil {
			q.TopN = v
		}
	}

	result, err := spend.QueryTopN(r.Context(), pool, q)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SpendTrend handles GET /spend/trend?start_date=...&end_date=...
func (h *Handlers) SpendTrend(w http.ResponseWriter, r *http.Request) {
	pool := h.getPool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	q := h.parseAnalyticsQuery(r)

	result, err := spend.QueryTrend(r.Context(), pool, q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) parseAnalyticsQuery(r *http.Request) spend.AnalyticsQuery {
	q := spend.AnalyticsQuery{
		GroupBy:   r.URL.Query().Get("group_by"),
		StartDate: time.Now().AddDate(0, -1, 0),
		EndDate:   time.Now(),
	}

	if sd := r.URL.Query().Get("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			q.StartDate = t
		}
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			q.EndDate = t
		}
	}

	return q
}

// getPool extracts the pgxpool from DB if available.
func (h *Handlers) getPool() *pgxpool.Pool {
	if h.DB == nil {
		return nil
	}
	return h.DB.Pool()
}
