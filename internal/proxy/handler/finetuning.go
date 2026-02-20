package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// FineTuningCreate handles POST /v1/fine_tuning/jobs — create a fine-tuning job.
func (h *Handlers) FineTuningCreate(w http.ResponseWriter, r *http.Request) {
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/fine_tuning/jobs", apiKey, "application/json")
}

// FineTuningGet handles GET /v1/fine_tuning/jobs/{job_id} — get job status.
func (h *Handlers) FineTuningGet(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "fine_tuning_job_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/fine_tuning/jobs/"+jobID, apiKey, "")
}

// FineTuningCancel handles POST /v1/fine_tuning/jobs/{job_id}/cancel — cancel a job.
func (h *Handlers) FineTuningCancel(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "fine_tuning_job_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/fine_tuning/jobs/"+jobID+"/cancel", apiKey, "application/json")
}

// FineTuningListEvents handles GET /v1/fine_tuning/jobs/{job_id}/events — list events.
func (h *Handlers) FineTuningListEvents(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "fine_tuning_job_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	url := baseURL + "/fine_tuning/jobs/" + jobID + "/events"
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	h.forwardToProvider(w, r, url, apiKey, "")
}

// FineTuningListCheckpoints handles GET /v1/fine_tuning/jobs/{job_id}/checkpoints — list checkpoints.
func (h *Handlers) FineTuningListCheckpoints(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "fine_tuning_job_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	url := baseURL + "/fine_tuning/jobs/" + jobID + "/checkpoints"
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	h.forwardToProvider(w, r, url, apiKey, "")
}
