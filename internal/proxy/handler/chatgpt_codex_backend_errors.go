package handler

import (
	"errors"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func writeChatGPTCodexBackendPreparationError(w http.ResponseWriter, err error) {
	if clientErr, ok := chatGPTCodexBackendClientValidationError(err); ok {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: clientErr.Message,
				Type:    clientErr.Type,
			},
		})
		return
	}
	writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
		Error: model.ErrorDetail{
			Message: "upstream request failed: " + err.Error(),
			Type:    "internal_error",
		},
	})
}

func chatGPTCodexBackendClientValidationError(err error) (*model.TianjiError, bool) {
	var clientErr *model.TianjiError
	if errors.As(err, &clientErr) && errors.Is(clientErr.Err, model.ErrClientValidation) {
		return clientErr, true
	}
	return nil, false
}
