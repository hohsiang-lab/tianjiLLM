package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestTianjiErrorString(t *testing.T) {
	e := &TianjiError{
		StatusCode: 429,
		Message:    "rate limited",
		Type:       "RateLimitError",
		Provider:   "openai",
		Model:      "gpt-4o",
		Err:        ErrRateLimit,
	}
	s := e.Error()
	if s == "" {
		t.Fatal("empty error string")
	}
}

func TestTianjiErrorUnwrap(t *testing.T) {
	e := &TianjiError{Err: ErrAuthentication}
	if !errors.Is(e, ErrAuthentication) {
		t.Fatal("Unwrap should expose inner error")
	}
}

func TestMapHTTPStatusToError(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{401, ErrAuthentication},
		{403, ErrPermission},
		{404, ErrNotFound},
		{429, ErrRateLimit},
		{400, ErrInvalidRequest},
		{408, ErrTimeout},
		{500, ErrServiceUnavailable},
		{503, ErrServiceUnavailable},
	}
	for _, tt := range tests {
		got := MapHTTPStatusToError(tt.status)
		if !errors.Is(got, tt.want) {
			t.Errorf("status %d: got %v, want %v", tt.status, got, tt.want)
		}
	}

	// Unknown status
	err := MapHTTPStatusToError(418)
	if err == nil {
		t.Fatal("should return error for unknown status")
	}
}

func TestIsStreaming(t *testing.T) {
	r := &ChatCompletionRequest{}
	if r.IsStreaming() {
		t.Fatal("nil stream should not be streaming")
	}

	f := false
	r.Stream = &f
	if r.IsStreaming() {
		t.Fatal("false stream should not be streaming")
	}

	tr := true
	r.Stream = &tr
	if !r.IsStreaming() {
		t.Fatal("true stream should be streaming")
	}
}

func TestUnmarshalJSONExtraParams(t *testing.T) {
	data := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"custom_field":"value"}`
	var r ChatCompletionRequest
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Model != "gpt-4o" {
		t.Fatalf("Model = %q", r.Model)
	}
	if r.ExtraParams == nil {
		t.Fatal("ExtraParams should capture unknown fields")
	}
	if _, ok := r.ExtraParams["custom_field"]; !ok {
		t.Fatal("custom_field not in ExtraParams")
	}
}

func TestUnmarshalJSONBasic(t *testing.T) {
	data := `{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"hello"}]}`
	var r ChatCompletionRequest
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Model != "gpt-3.5-turbo" {
		t.Fatalf("Model = %q", r.Model)
	}
	if len(r.Messages) != 1 {
		t.Fatalf("Messages = %d", len(r.Messages))
	}
}
