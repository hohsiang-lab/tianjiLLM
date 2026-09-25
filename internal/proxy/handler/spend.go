package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func tsTZ(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// SpendKeys handles GET /spend/keys
func (h *Handlers) SpendKeys(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	since := h.parseSince(r)

	keys := r.URL.Query()["key"]
	if len(keys) == 0 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "key parameter required", Type: "invalid_request_error"},
		})
		return
	}

	spend, err := h.DB.GetSpendByKey(r.Context(), db.GetSpendByKeyParams{
		Column1:   keys,
		Starttime: tsTZ(since),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, spend)
}

// SpendUsers handles GET /spend/users
func (h *Handlers) SpendUsers(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	since := h.parseSince(r)

	users := r.URL.Query()["user"]
	if len(users) == 0 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user parameter required", Type: "invalid_request_error"},
		})
		return
	}

	spend, err := h.DB.GetSpendByUser(r.Context(), db.GetSpendByUserParams{
		Column1:   users,
		Starttime: tsTZ(since),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, spend)
}

func (h *Handlers) parseEnd(r *http.Request) time.Time {
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, ed); err == nil {
			return t
		}
	}
	return time.Now()
}

func (h *Handlers) parseSince(r *http.Request) time.Time {
	since := r.URL.Query().Get("since")
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			return t
		}
	}
	startDate := r.URL.Query().Get("start_date")
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			return t
		}
	}
	return time.Now().AddDate(0, -1, 0)
}

// SpendByTeams handles GET /spend/teams.
func (h *Handlers) SpendByTeams(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.GetSpendByTeam(r.Context(), tsTZ(h.parseSince(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SpendByTags handles GET /spend/tags.
func (h *Handlers) SpendByTags(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.GetSpendByTag(r.Context(), tsTZ(h.parseSince(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SpendByModels handles GET /spend/models.
func (h *Handlers) SpendByModels(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.GetSpendByModel(r.Context(), tsTZ(h.parseSince(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SpendByEndUsers handles GET /spend/end_users.
func (h *Handlers) SpendByEndUsers(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.GetSpendByEndUser(r.Context(), tsTZ(h.parseSince(r)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
