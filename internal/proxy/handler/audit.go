package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// AuditLogList handles GET /audit — paginated list of audit logs with filters.
func (h *Handlers) AuditLogList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	offset, _ := strconv.Atoi(q.Get("offset"))

	params := db.ListAuditLogsParams{
		Column1: q.Get("changed_by"),
		Column2: q.Get("action"),
		Column3: q.Get("table_name"),
		Column4: q.Get("object_id"),
		Limit:   int32(limit),
		Offset:  int32(offset),
	}

	if start := q.Get("start_date"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			params.Column5 = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if end := q.Get("end_date"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			params.Column6 = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	logs, err := h.DB.ListAuditLogs(r.Context(), params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list audit logs: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": logs,
	})
}

// AuditLogGet handles GET /audit/{id} — single audit log entry.
func (h *Handlers) AuditLogGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "missing id", Type: "invalid_request_error"},
		})
		return
	}

	log, err := h.DB.GetAuditLog(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "audit log not found", Type: "not_found_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, log)
}
