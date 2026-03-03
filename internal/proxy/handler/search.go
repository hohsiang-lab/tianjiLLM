package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/search"
)

// SearchHandler handles POST /v1/search/{search_tool_name}.
func (h *Handlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "search_tool_name")

	// Find config for this search tool
	var providerName, apiKey, apiBase string
	found := false
	if h.Config != nil {
		for _, st := range h.Config.SearchTools {
			if st.SearchToolName == toolName {
				providerName = st.TianjiParams.SearchProvider
				apiKey = st.TianjiParams.APIKey
				apiBase = st.TianjiParams.APIBase
				found = true
				break
			}
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "search tool not found: " + toolName, Type: "not_found"},
		})
		return
	}

	provider, err := search.Get(providerName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	var params search.SearchParams
	if err = decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	if params.Query == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "query is required", Type: "invalid_request_error"},
		})
		return
	}

	headers, err := provider.ValidateEnvironment(apiKey, apiBase)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	upstreamURL := provider.GetCompleteURL(apiBase, params)

	_srchProvider, _srchURL, _srchHeaders, _srchParams := provider, upstreamURL, headers, params
	resp, err := doUpstreamWithRetry(r.Context(), http.DefaultClient, func() (*http.Request, error) {
		var req2 *http.Request
		if _srchProvider.HTTPMethod() == http.MethodPost {
			reqBody := _srchProvider.TransformRequest(_srchParams)
			bodyBytes, _ := json.Marshal(reqBody)
			req2, _ = http.NewRequestWithContext(r.Context(), http.MethodPost, _srchURL, bytes.NewReader(bodyBytes))
		} else {
			req2, _ = http.NewRequestWithContext(r.Context(), http.MethodGet, _srchURL, nil)
		}
		for k, vs := range _srchHeaders {
			for _, v := range vs {
				req2.Header.Set(k, v)
			}
		}
		return req2, nil
	}, h.MaxUpstreamRetries)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "upstream request failed: " + err.Error(), Type: "upstream_error"},
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "read upstream response: " + err.Error(), Type: "upstream_error"},
		})
		return
	}

	if resp.StatusCode >= 400 {
		writeJSON(w, resp.StatusCode, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "upstream error: " + string(body), Type: "upstream_error"},
		})
		return
	}

	searchResp, err := provider.TransformResponse(body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, searchResp)
}
