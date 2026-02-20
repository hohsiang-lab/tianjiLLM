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
)

// TianjiError is the unified error type returned by provider calls.
type TianjiError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Type       string `json:"type"`
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
