package handler

import (
	"errors"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func writeRequestError(w http.ResponseWriter, err *model.RequestError) {
	writeJSON(w, err.StatusCode, model.ErrorResponse{Error: err.Detail})
}

func writeOpenAIEndpointRouteError(w http.ResponseWriter, err error) {
	var requestErr *model.RequestError
	if errors.As(err, &requestErr) {
		writeRequestError(w, requestErr)
		return
	}
	writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
		Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
	})
}

func writeEmbeddingError(w http.ResponseWriter, err error, modelName string) {
	var requestErr *model.RequestError
	if errors.As(err, &requestErr) {
		writeRequestError(w, requestErr)
		return
	}
	if errors.Is(err, model.ErrClientValidation) {
		writeRequestError(w, model.InvalidRequest("", "Invalid embedding request"))
		return
	}

	var upstreamErr *model.TianjiError
	if errors.As(err, &upstreamErr) && upstreamErr.StatusCode == http.StatusBadRequest {
		writeRequestError(w, model.InvalidRequest("", "Upstream rejected the embedding request"))
		return
	}
	writeUpstreamRequestFailure(w, err, modelName)
}
