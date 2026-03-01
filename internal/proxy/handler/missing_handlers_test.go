package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestA2AHandlers_NotConfigured tests A2A handlers when AgentRegistry is nil.
func TestA2AHandlers_NotConfigured(t *testing.T) {
	h := newTestHandlers()

	fns := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"A2AAgentCard", h.A2AAgentCard},
		{"A2AMessage", h.A2AMessage},
	}
	for _, f := range fns {
		t.Run(f.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			f.fn(w, r)
			if w.Code == http.StatusOK {
				t.Fatalf("expected non-200, got 200")
			}
		})
	}
}

// TestBatchesHandlers_NoProvider tests Batches handlers with no provider configured.
func TestBatchesHandlers_NoProvider(t *testing.T) {
	h := newTestHandlers()

	fns := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"BatchesCreate", h.BatchesCreate},
		{"BatchesGet", h.BatchesGet},
		{"BatchesCancel", h.BatchesCancel},
		{"BatchesList", h.BatchesList},
	}
	for _, f := range fns {
		t.Run(f.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			f.fn(w, r)
			if w.Code == http.StatusOK {
				t.Fatalf("%s: expected non-200 with no provider, got 200", f.name)
			}
		})
	}
}

// TestFineTuningHandlers_NoProvider tests FineTuning handlers with no provider configured.
func TestFineTuningHandlers_NoProvider(t *testing.T) {
	h := newTestHandlers()

	fns := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"FineTuningCreate", h.FineTuningCreate},
		{"FineTuningGet", h.FineTuningGet},
		{"FineTuningCancel", h.FineTuningCancel},
		{"FineTuningListEvents", h.FineTuningListEvents},
		{"FineTuningListCheckpoints", h.FineTuningListCheckpoints},
	}
	for _, f := range fns {
		t.Run(f.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			f.fn(w, r)
			if w.Code == http.StatusOK {
				t.Fatalf("%s: expected non-200, got 200", f.name)
			}
		})
	}
}
