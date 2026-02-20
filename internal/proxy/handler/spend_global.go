package handler

import (
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func spendTSTZ(r *http.Request, h *Handlers) (pgtype.Timestamptz, pgtype.Timestamptz) {
	since := h.parseSince(r)
	end := h.parseEnd(r)
	return tsTZ(since), tsTZ(end)
}

// GlobalSpend handles GET /global/spend — aggregate spend for a time range.
func (h *Handlers) GlobalSpend(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetGlobalSpend(r.Context(), db.GetGlobalSpendParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalSpendByKeys handles GET /global/spend/keys — daily spend broken down by key.
func (h *Handlers) GlobalSpendByKeys(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetDailySpendByKey(r.Context(), db.GetDailySpendByKeyParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalSpendByModels handles GET /global/spend/models.
func (h *Handlers) GlobalSpendByModels(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetDailySpendByModel(r.Context(), db.GetDailySpendByModelParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalSpendByTeams handles GET /global/spend/teams.
func (h *Handlers) GlobalSpendByTeams(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetDailySpendByTeam(r.Context(), db.GetDailySpendByTeamParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalSpendByTags handles GET /global/spend/tags.
func (h *Handlers) GlobalSpendByTags(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetDailySpendByTag(r.Context(), db.GetDailySpendByTagParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalSpendByProvider handles GET /global/spend/provider.
func (h *Handlers) GlobalSpendByProvider(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetGlobalSpendByProvider(r.Context(), db.GetGlobalSpendByProviderParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalActivity handles GET /global/activity.
func (h *Handlers) GlobalActivity(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetGlobalActivity(r.Context(), db.GetGlobalActivityParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query activity: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalActivityByModel handles GET /global/activity/model.
func (h *Handlers) GlobalActivityByModel(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetGlobalActivityByModel(r.Context(), db.GetGlobalActivityByModelParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query activity: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GlobalSpendReport handles GET /global/spend/report.
func (h *Handlers) GlobalSpendReport(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	groupBy := r.URL.Query().Get("group_by")

	switch groupBy {
	case "customer":
		result, err := h.DB.GetGlobalSpendReportByCustomer(r.Context(), db.GetGlobalSpendReportByCustomerParams{
			Starttime: from, Starttime_2: to,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "query report: " + err.Error(), Type: "internal_error"},
			})
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "key":
		result, err := h.DB.GetGlobalSpendReportByKey(r.Context(), db.GetGlobalSpendReportByKeyParams{
			Starttime: from, Starttime_2: to,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "query report: " + err.Error(), Type: "internal_error"},
			})
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		result, err := h.DB.GetGlobalSpendReport(r.Context(), db.GetGlobalSpendReportParams{
			Starttime: from, Starttime_2: to,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "query report: " + err.Error(), Type: "internal_error"},
			})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// GlobalSpendReset handles POST /global/spend/reset.
func (h *Handlers) GlobalSpendReset(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	if err := h.DB.ResetAllKeySpend(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "reset key spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	if err := h.DB.ResetAllTeamSpend(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "reset team spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "reset_spend", "global", "all", "", "", nil, map[string]any{"spend": 0})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CacheHitStats handles GET /global/activity/cache_hits.
func (h *Handlers) CacheHitStats(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)
	result, err := h.DB.GetCacheHitStats(r.Context(), db.GetCacheHitStatsParams{
		Starttime:   from,
		Starttime_2: to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query cache stats: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SpendLogs handles GET /spend/logs with optional key, team, model, date filtering.
func (h *Handlers) SpendLogs(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	from, to := spendTSTZ(r, h)

	limit := int32(100)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	offset := int32(0)
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}

	result, err := h.DB.GetSpendLogsByFilter(r.Context(), db.GetSpendLogsByFilterParams{
		Starttime:   from,
		Starttime_2: to,
		Column3:     r.URL.Query().Get("key"),
		Column4:     r.URL.Query().Get("team"),
		Column5:     r.URL.Query().Get("model"),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend logs: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
