package model

import (
	"errors"
	"fmt"
)

// Sentinel errors for LLM provider error classification.
var (
	ErrAuthentication         = errors.New("AuthenticationError")
	ErrRateLimit              = errors.New("RateLimitError")
	ErrBudgetExceeded         = errors.New("BudgetExceededError")
	ErrNotFound               = errors.New("NotFoundError")
	ErrTimeout                = errors.New("Timeout")
	ErrServiceUnavailable     = errors.New("ServiceUnavailableError")
	ErrContextWindowExceeded  = errors.New("ContextWindowExceededError")
	ErrContentPolicyViolation = errors.New("ContentPolicyViolationError")
	ErrInvalidRequest         = errors.New("InvalidRequestError")
	ErrPermission             = errors.New("PermissionDeniedError")

	// ErrClientValidation is returned by provider Transform* methods when the
	// request itself is invalid (e.g. unsupported encoding_format). Handlers
	// should map this to HTTP 400 rather than 500.
	ErrClientValidation = errors.New("ClientValidationError")
)

// TianjiError is the unified error type returned by provider calls.
type TianjiError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Type       string `json:"type"`
	Code       string `json:"code,omitempty"`
	Provider   string `json:"llm_provider"`
	Model      string `json:"model"`
	Err        error  `json:"-"`
}

func (e *TianjiError) Error() string {
	return fmt.Sprintf("[%s] %s: %s (status=%d, model=%s)",
		e.Provider, e.Type, e.Message, e.StatusCode, e.Model)
}

func (e *TianjiError) Unwrap() error {
	return e.Err
}

// ErrorResponse is the JSON error body returned by the proxy.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message  string `json:"message"`
	Type     string `json:"type"`
	Param    string `json:"param,omitempty"`
	Code     string `json:"code,omitempty"`
	Provider string `json:"llm_provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type RequestError struct {
	StatusCode int
	Detail     ErrorDetail
}

func (e *RequestError) Error() string {
	return e.Detail.Message
}

func UnsupportedParameter(param, message string) *RequestError {
	return requestError(400, param, "unsupported_parameter", message)
}

func InvalidValue(param, message string) *RequestError {
	return requestError(400, param, "invalid_value", message)
}

func InvalidRequest(param, message string) *RequestError {
	return requestError(400, param, "invalid_request", message)
}

func ModelNotFound(model string) *RequestError {
	return requestError(404, "", "model_not_found", fmt.Sprintf("model %q not found", model))
}

func requestError(status int, param, code, message string) *RequestError {
	return &RequestError{
		StatusCode: status,
		Detail: ErrorDetail{
			Message: message,
			Type:    "invalid_request_error",
			Param:   param,
			Code:    code,
		},
	}
}

// MapHTTPStatusToError maps an HTTP status code to a sentinel error.
func MapHTTPStatusToError(status int) error {
	switch {
	case status == 401:
		return ErrAuthentication
	case status == 403:
		return ErrPermission
	case status == 404:
		return ErrNotFound
	case status == 429:
		return ErrRateLimit
	case status == 400:
		return ErrInvalidRequest
	case status == 408:
		return ErrTimeout
	case status >= 500:
		return ErrServiceUnavailable
	default:
		return fmt.Errorf("unexpected status code: %d", status)
	}
}
