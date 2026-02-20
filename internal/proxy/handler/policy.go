package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/policy"
)

// PolicyCreate handles POST /policy.
func (h *Handlers) PolicyCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req model.CreatePolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request body: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "name is required", Type: "invalid_request_error"},
		})
		return
	}

	conditions, _ := json.Marshal(req.Conditions)
	pipeline, _ := json.Marshal(req.Pipeline)

	result, err := h.DB.CreatePolicy(r.Context(), db.CreatePolicyParams{
		Name:             req.Name,
		ParentID:         req.ParentID,
		Conditions:       conditions,
		GuardrailsAdd:    req.GuardrailsAdd,
		GuardrailsRemove: req.GuardrailsRemove,
		Pipeline:         pipeline,
		Description:      req.Description,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create policy: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// PolicyGet handles GET /policy/{id}.
func (h *Handlers) PolicyGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.DB.GetPolicy(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "policy not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PolicyList handles GET /policy/list.
func (h *Handlers) PolicyList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.ListPolicies(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list policies: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PolicyUpdate handles PUT /policy/{id}.
func (h *Handlers) PolicyUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")

	var req model.UpdatePolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request body: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	conditions, _ := json.Marshal(req.Conditions)
	pipeline, _ := json.Marshal(req.Pipeline)

	name := ""
	if req.Name != nil {
		name = *req.Name
	}

	result, err := h.DB.UpdatePolicy(r.Context(), db.UpdatePolicyParams{
		ID:               id,
		Name:             name,
		ParentID:         req.ParentID,
		Conditions:       conditions,
		GuardrailsAdd:    req.GuardrailsAdd,
		GuardrailsRemove: req.GuardrailsRemove,
		Pipeline:         pipeline,
		Description:      req.Description,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update policy: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PolicyDelete handles DELETE /policy/{id}.
func (h *Handlers) PolicyDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.DB.DeletePolicy(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete policy: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PolicyAttachmentCreate handles POST /policy/attachment.
func (h *Handlers) PolicyAttachmentCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req model.CreatePolicyAttachmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request body: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	if req.PolicyName == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "policy_name is required", Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.CreatePolicyAttachment(r.Context(), db.CreatePolicyAttachmentParams{
		PolicyName: req.PolicyName,
		Scope:      req.Scope,
		Teams:      req.Teams,
		Keys:       req.Keys,
		Models:     req.Models,
		Tags:       req.Tags,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create attachment: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// PolicyAttachmentGet handles GET /policy/attachment/{id}.
func (h *Handlers) PolicyAttachmentGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.DB.GetPolicyAttachment(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "attachment not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PolicyAttachmentList handles GET /policy/attachment/list.
func (h *Handlers) PolicyAttachmentList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	policyName := r.URL.Query().Get("policy_name")
	var result []db.PolicyAttachmentTable
	var err error

	if policyName != "" {
		result, err = h.DB.ListPolicyAttachmentsByPolicy(r.Context(), policyName)
	} else {
		result, err = h.DB.ListPolicyAttachments(r.Context())
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list attachments: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PolicyAttachmentDelete handles DELETE /policy/attachment/{id}.
func (h *Handlers) PolicyAttachmentDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.DB.DeletePolicyAttachment(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete attachment: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PolicyTestPipeline handles POST /policy/test-pipeline.
func (h *Handlers) PolicyTestPipeline(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req model.TestPipelineRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request body: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	p, err := h.DB.GetPolicyByName(r.Context(), req.PolicyName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "policy not found", Type: "not_found"},
		})
		return
	}

	// Use a no-op checker for testing — all guardrails pass by default.
	checker := &testPipelineChecker{}
	result := policy.ExecutePipeline(p.Pipeline, checker, req.Input)

	writeJSON(w, http.StatusOK, model.TestPipelineResponse{
		Result:  result.Action,
		Message: result.Message,
		Steps:   result.Steps,
	})
}

// PolicyResolvedGuardrails handles GET /policy/resolved-guardrails.
func (h *Handlers) PolicyResolvedGuardrails(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil || h.PolicyEng == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "policy engine not configured", Type: "internal_error"},
		})
		return
	}

	req := policy.MatchRequest{
		TeamID: r.URL.Query().Get("team_id"),
		KeyID:  r.URL.Query().Get("key_id"),
		Model:  r.URL.Query().Get("model"),
		Tags:   r.URL.Query()["tag"],
	}

	result, err := h.PolicyEng.Evaluate(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "evaluate policies: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, model.PolicyResolutionResponse{
		Guardrails: result.Guardrails,
		Policies:   result.Policies,
	})
}

// testPipelineChecker is a no-op guardrail checker for testing pipelines.
// All guardrails pass.
type testPipelineChecker struct{}

func (c *testPipelineChecker) Check(guardrailName string, input map[string]any) (bool, error) {
	return true, nil
}
